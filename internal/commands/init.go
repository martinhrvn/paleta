package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/scan"
	"github.com/martinhrvn/paleta/internal/ui"
)

// InitOutcome describes the result of running the interactive init wizard.
type InitOutcome int

const (
	InitWritten         InitOutcome = iota // a new .pltrc was written
	InitNoProjects                         // nothing detected in the tree
	InitCanceled                           // user aborted the wizard
	InitNothingSelected                    // wizard confirmed with no locations
)

// runWizard is the interactive step of RunInitWizard: it takes over the terminal
// and returns what the user confirmed. It is a variable so the decision logic
// around it (see bootstrap.go) can be exercised without a tty.
var runWizard = func(items []ui.WizardItem) ([]config.Location, bool, error) {
	wizard := ui.NewWizardModel(items)
	return wizard.Run()
}

// RunInitWizard scans root, lets the user pick which projects to include, and
// writes root's .pltrc, preserving any existing config there as the starting
// state. Scanning and writing share one root on purpose: the paths in the file
// are relative to it, so a wizard run from somewhere else would write locations
// that point at the wrong folders. It performs no stdout output, so it is safe to
// call from within `plt select` (whose stdout carries the selection JSON).
// Callers decide how to report the returned outcome.
//
// An existing config that can't be parsed stops the run: the wizard's job is to
// preserve what is already configured, and it can't do that from a file it can't
// read. force ignores it and starts from the scan instead, replacing the file.
func RunInitWizard(root string, force bool) (InitOutcome, error) {
	if root == "" {
		root = "."
	}
	configPath := filepath.Join(root, config.ConfigFileName)

	cands, err := scan.Scan(root)
	if err != nil {
		return InitNoProjects, fmt.Errorf("scanning for projects: %w", err)
	}

	// The existing file is the starting state, and it is edited in place so its
	// comments survive; --force ignores it and starts from the scan.
	var file *config.File
	var authored *config.Config
	if !force {
		file, err = config.OpenFile(configPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return InitNoProjects, err
		}
		if file != nil {
			if authored, err = file.Authored(); err != nil {
				return InitNoProjects, err
			}
		}
	}
	if file == nil {
		file = config.NewFile(configPath)
	}

	items := BuildWizardItems(cands, authored)
	if len(items) == 0 {
		return InitNoProjects, nil
	}

	locations, confirmed, err := runWizard(items)
	if err != nil {
		return InitCanceled, fmt.Errorf("running wizard: %w", err)
	}
	if !confirmed {
		return InitCanceled, nil
	}
	if len(locations) == 0 {
		return InitNothingSelected, nil
	}

	if err := file.SetLocations(locations); err != nil {
		return InitWritten, fmt.Errorf("writing config: %w", err)
	}
	if err := file.Save(); err != nil {
		return InitWritten, fmt.Errorf("writing config: %w", err)
	}
	return InitWritten, nil
}

// BuildWizardItems unions scan candidates with the locations from an existing
// config (authored may be nil). Authored locations come first and are preserved
// verbatim; a candidate matching an authored location by path marks it Detected,
// while remaining candidates are appended as newly detected items.
func BuildWizardItems(cands []scan.Candidate, authored *config.Config) []ui.WizardItem {
	var items []ui.WizardItem
	matched := make(map[string]bool) // candidate RelPath already represented

	if authored != nil {
		for _, loc := range authored.Locations {
			item := ui.WizardItem{Location: loc, Configured: true, TypeOptions: loc.Types}
			cleaned := filepath.Clean(loc.Location)
			for _, c := range cands {
				if c.RelPath == cleaned {
					item.Detected = true
					// Offer types detected in the folder since the location was
					// written. They stay unticked: adding one changes how this
					// location's existing commands are labelled.
					item.TypeOptions = unionTypes(loc.Types, c.Types)
					matched[c.RelPath] = true
					break
				}
			}
			items = append(items, item)
		}
	}

	var fresh []string
	for _, c := range cands {
		if !matched[c.RelPath] {
			fresh = append(fresh, c.RelPath)
		}
	}
	names := assignWizardNames(fresh, authored)

	for _, c := range cands {
		if matched[c.RelPath] {
			continue
		}
		items = append(items, ui.WizardItem{
			Location: config.Location{
				Name:     names[c.RelPath],
				Location: c.RelPath,
				Types:    c.Types,
			},
			TypeOptions: c.Types,
			Detected:    true,
		})
	}

	return items
}

// unionTypes merges detected types into an authored list, keeping the authored
// order first (a hand-written type whose detect file is missing stays offered)
// and appending anything newly detected.
func unionTypes(authored config.Types, detected []string) []string {
	out := make([]string, 0, len(authored)+len(detected))
	seen := make(map[string]bool, len(authored)+len(detected))
	for _, t := range authored {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	for _, t := range detected {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// assignWizardNames picks a display name for each newly detected path: the
// folder's own name when nothing else claims it, otherwise the relative path.
//
// Two folders called "web" would otherwise produce two locations named "web",
// which makes an @web:build reference ambiguous (internal/config/alias.go) and
// merges their frecency history, which is keyed by display name. A path is a
// legal name — the alias charset allows '/' and references resolve by path tail.
func assignWizardNames(relPaths []string, authored *config.Config) map[string]string {
	// Names already spoken for: an authored location's name, and its folder name,
	// which alias resolution indexes whether or not the location is named.
	taken := make(map[string]int)
	if authored != nil {
		for _, loc := range authored.Locations {
			if loc.Name != "" {
				taken[loc.Name]++
			}
			if !strings.Contains(loc.Location, "*") {
				taken[locationName(filepath.Clean(loc.Location))]++
			}
		}
	}
	for _, rel := range relPaths {
		taken[locationName(rel)]++
	}

	names := make(map[string]string, len(relPaths))
	for _, rel := range relPaths {
		base := locationName(rel)
		if taken[base] > 1 {
			names[rel] = rel // the path always identifies exactly one folder
			continue
		}
		names[rel] = base
	}
	return names
}

// locationName derives a display name from a relative path. The scan root (".")
// becomes "root"; otherwise the base directory name is used.
func locationName(relPath string) string {
	if relPath == "." || relPath == "" {
		return "root"
	}
	return filepath.Base(relPath)
}

// DefaultConfigTemplate returns the default .pltrc template
func DefaultConfigTemplate() string {
	return `# paleta configuration file
# This file defines locations and commands for your project

locations:
  # Example: NPM/Yarn/PNPM project with automatic script detection
  - name: "frontend"
    location: "packages/frontend"
    type: "npm"  # Automatically discovers package.json scripts
    commands:
      # Additional custom commands beyond package.json scripts
      - "npm run dev"

  # Example: Backend service with manual commands
  - name: "backend"
    location: "packages/backend"
    commands:
      - "npm start"
      - "npm test"
      - "npm run build"

  # Example: A folder that is both an npm package and a Docker service.
  # Commands from every type are merged; each is labelled with its type, e.g.
  # "svc: [npm] build" and "svc: [docker] build".
  # - name: "svc"
  #   location: "services/svc"
  #   type: [npm, docker]

  # Example: Filtering a location's commands
  # - name: "app"
  #   location: "app"
  #   type: "npm"
  #   include_commands:   # Only keep commands matching these patterns
  #     - "dev"
  #     - "build*"
  #     - "test"
  #   exclude_commands:   # Drop commands matching these patterns
  #     - "test:watch"

  # Example: Using glob patterns to match multiple directories
  # - location: "packages/*"
  #   type: "npm"

  # Example: Dropping folders from a glob, and refining one of them
  # - location: "services/*"
  #   type: "npm"
  #   exclude_locations:        # Folders to leave out of the expansion
  #     - "legacy"
  #     - "*-deprecated"
  #   overrides:                # Per-folder additions; keys match folder names
  #     frontend-next:
  #       commands:
  #         - name: "commandx"
  #           command: "npm run commandx"
  #       env:
  #         PORT: "3001"

  # Example: Scripts directory with shell scripts
  # - name: "scripts"
  #   location: "scripts"
  #   commands:
  #     - "./deploy.sh"
  #     - "./backup.sh"

# Configuration reference:
#
# location fields:
#   name:     (optional) Display name shown in selection UI
#   location: (optional) Path to project directory (supports glob patterns like "packages/*")
#                        If omitted, defaults to current directory "."
#   type:     (optional) Project type(s) for automatic command detection.
#             Single value (type: npm) or a list (type: [npm, docker]).
#   commands: (optional) List of commands to make available
#   include_commands:  (optional) Glob patterns to filter this location's commands (whitelist)
#   exclude_commands:  (optional) Glob patterns to drop commands (blacklist)
#   exclude_locations: (optional) Folder patterns to drop from a glob expansion
#   overrides:         (optional) Per-folder overrides for a glob location
#
# include_commands/exclude_commands notes:
#   - Applies to auto-discovered commands (from type) and to the ones you write here
#   - Patterns match a command's name when it has one (npm scripts: "dev", "test:ci"),
#     otherwise its full command string (e.g. "npm run legacy")
#   - Patterns support glob syntax (e.g., "test*" matches "test:ci")
#   - Include is applied first (whitelist), then exclude (blacklist)
#   - The older spellings "include:"/"exclude:" still work; 'plt lint --fix' renames them
#
# exclude_locations/overrides notes:
#   - Only apply to a glob location (e.g. "services/*")
#   - Patterns/keys match the expanded folder's own name ("api", "*-worker")
#   - An override adds its commands (a same-named one replaces) and merges its env;
#     name, type and the command filters replace the inherited value
#
# Supported project types:
#   npm, yarn, pnpm: Automatically discovers scripts from package.json
#   go: Discovers standard Go commands (planned)
#
# For more information, see: https://github.com/martinhrvn/paleta
`
}

// CreateDefaultConfigWithForce creates a default .pltrc file at the specified path
// If force is true, overwrites existing file
func CreateDefaultConfigWithForce(configPath string, force bool) error {
	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		if !force {
			return fmt.Errorf("config file already exists at %s (use --force to overwrite)", configPath)
		}
	}

	// Create the file with default template
	content := DefaultConfigTemplate()
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
