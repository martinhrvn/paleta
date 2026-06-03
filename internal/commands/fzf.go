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
			// Use command name if available, otherwise use full command
			cmdDisplay := command.Name
			if cmdDisplay == "" {
				cmdDisplay = command.Command
			}

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

// RunFzfTUI executes the fzf-style TUI with multi-select support
func RunFzfTUI(cfg *config.Config) ([]SelectionResult, error) {
	model := ui.NewModel(cfg)
	results, err := model.Run()
	if err != nil {
		return nil, err
	}

	// ui.SelectionResult and commands.SelectionResult are the same type, so
	// the model's results can be returned directly with no conversion.
	return results, nil
}
