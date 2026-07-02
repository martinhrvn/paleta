package commands

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/history"
	"github.com/martinhrvn/paleta/internal/ui"
)

// SelectionResult represents the result of a user selection from fzf.
// It is an alias for ui.SelectionResult so the two packages share one
// definition and no conversion is needed between them.
type SelectionResult = ui.SelectionResult

// ParseFzfSelection parses a fzf selection in format "location: command" and returns command and location
func ParseFzfSelection(selection string) (string, string, error) {
	if selection == "" {
		return "", "", fmt.Errorf("empty selection")
	}

	// Match pattern: location: command
	re := regexp.MustCompile(`^([^:]+):\s*(.+)$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(selection))

	if len(matches) != 3 {
		return "", "", fmt.Errorf("invalid selection format: expected 'location: command', got: %q", selection)
	}

	location := matches[1]
	command := matches[2]

	return command, location, nil
}

// FindLocationByDisplayName finds a location in the config by its display name (name or location field)
func FindLocationByDisplayName(cfg *config.Config, displayName string) (*config.Location, error) {
	for _, location := range cfg.Locations {
		// Check if display name matches the name field
		if location.Name == displayName {
			return &location, nil
		}
		// Check if display name matches the location field (when no name is set)
		if location.Name == "" && location.Location == displayName {
			return &location, nil
		}
		// Handle "(root)" as a display name for "." location
		if location.Name == "" && location.Location == "." && displayName == "(root)" {
			return &location, nil
		}
	}

	return nil, fmt.Errorf("location not found: %q", displayName)
}

// ProcessFzfSelection processes a fzf selection and returns a SelectionResult
func ProcessFzfSelection(cfg *config.Config, fzfSelection string) (*SelectionResult, error) {
	// Parse the fzf selection
	command, displayName, err := ParseFzfSelection(fzfSelection)
	if err != nil {
		return nil, err
	}

	// Find the location in config
	location, err := FindLocationByDisplayName(cfg, displayName)
	if err != nil {
		return nil, err
	}

	// Resolve env for the matched command (location-level env still applies
	// even when the exact command can't be matched by text).
	var matched config.Command
	for _, c := range location.Commands {
		if c.Name == command || c.Command == command {
			matched = c
			break
		}
	}

	return &SelectionResult{
		Directory:   location.Location,
		Command:     command,
		DisplayName: displayName,
		Env:         config.EffectiveEnv(*location, matched),
	}, nil
}

// CommandInfo holds information about a command for display
type CommandInfo struct {
	Display       string
	Directory     string
	Command       string
	DisplayName   string
	Env           map[string]string // Resolved environment variables for this command
	FrecencyScore float64           // Score for sorting
}

// PrepareCommandInfo prepares command information for fuzzy finder
func PrepareCommandInfo(cfg *config.Config) []CommandInfo {
	return PrepareCommandInfoWithHistory(cfg, nil, false)
}

// PrepareCommandInfoWithHistory prepares command information with optional frecency sorting
func PrepareCommandInfoWithHistory(cfg *config.Config, hist *history.History, enableFrecency bool) []CommandInfo {
	var infos []CommandInfo

	for _, location := range cfg.Locations {
		displayName := location.Name
		if displayName == "" {
			displayName = location.Location
			// Use a more user-friendly name for root directory
			if displayName == "." {
				displayName = "(root)"
			}
		}

		for _, command := range location.Commands {
			cmdDisplay := config.CommandLabel(location, command)

			// Calculate frecency score if history is available
			var score float64
			if hist != nil && enableFrecency {
				score = hist.GetScore(displayName, command.Command)
			}

			info := CommandInfo{
				Display:       fmt.Sprintf("%s: %s", displayName, cmdDisplay),
				Directory:     location.Location,
				Command:       command.Command,
				DisplayName:   displayName,
				Env:           config.EffectiveEnv(location, command),
				FrecencyScore: score,
			}
			infos = append(infos, info)
		}
	}

	// Sort by frecency score if enabled
	if enableFrecency && hist != nil {
		sort.Slice(infos, func(i, j int) bool {
			// Higher scores first
			return infos[i].FrecencyScore > infos[j].FrecencyScore
		})
	}

	return infos
}

// RunFzfTUI executes the fzf-style TUI with multi-select support. configPath is
// the discovered .pltrc path (may be empty for a global-fallback config); it
// enables focus persistence and in-app project adding. When the user requests
// adding projects (Ctrl+N), the init wizard runs and the selector re-enters with
// the reloaded config.
func RunFzfTUI(cfg *config.Config, configPath string) ([]SelectionResult, error) {
	for {
		model := ui.NewModel(cfg, focusStore(configPath))
		model.SetSaveStore(saveStore(configPath))
		results, reinit, err := model.Run()
		if reinit {
			if _, ierr := RunInitWizard(configPath); ierr != nil {
				return nil, ierr
			}
			if reloaded, rerr := config.LoadConfigFromDiscovery(); rerr == nil {
				cfg = reloaded
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		// ui.SelectionResult and commands.SelectionResult are the same type, so
		// the model's results can be returned directly with no conversion.
		return results, nil
	}
}

// focusStore builds the focus persistence hooks for the selector, or nil when
// there is no writable local config path to persist to.
func focusStore(configPath string) *ui.FocusStore {
	if configPath == "" {
		return nil
	}
	return &ui.FocusStore{
		List: func() ([]ui.FocusEntry, error) { return FocusEntries(configPath) },
		Save: func(focused map[string]bool) error { return SetFocused(configPath, focused) },
	}
}

// saveStore builds the queue-save hook for the selector, or nil when there is no
// writable local config path to persist to.
func saveStore(configPath string) *ui.SaveStore {
	if configPath == "" {
		return nil
	}
	return &ui.SaveStore{
		Save: func(displayName, directory, name, command string) error {
			return AddCommandToLocation(configPath, displayName, directory, name, command)
		},
	}
}

// AddCommandToLocation appends a composite command (e.g. "a && b && c") to the
// location identified by displayName in the authored .pltrc, then rewrites the
// file. directory is the resolved location path, used to match an authored entry
// or — when no entry matches (e.g. the display name came from a glob) — to create
// a new location. Like `plt init`, this rewrites the file, normalizing its
// formatting and dropping comments.
func AddCommandToLocation(configPath, displayName, directory, name, command string) error {
	authored, err := LoadAuthoredConfig(configPath)
	if err != nil {
		return err
	}
	if authored == nil {
		return fmt.Errorf("no config file at %s", configPath)
	}

	newCmd := config.Command{Name: name, Command: command}

	if idx := findAuthoredLocation(authored, displayName, directory); idx >= 0 {
		authored.Locations[idx].Commands = append(authored.Locations[idx].Commands, newCmd)
	} else {
		authored.Locations = append(authored.Locations, config.Location{
			Name:     displayName,
			Location: directory,
			Commands: []config.Command{newCmd},
		})
	}

	content := GenerateConfig(authored.Locations, authored)
	return WriteConfig(configPath, content)
}

// findAuthoredLocation returns the index of the authored location matching the
// display name (by name, or by path when unnamed), falling back to a directory
// match; -1 when none match.
func findAuthoredLocation(cfg *config.Config, displayName, directory string) int {
	for i, loc := range cfg.Locations {
		switch {
		case loc.Name == displayName:
			return i
		case loc.Name == "" && loc.Location == displayName:
			return i
		case loc.Name == "" && loc.Location == "." && displayName == "(root)":
			return i
		case directory != "" && loc.Location == directory:
			return i
		}
	}
	return -1
}
