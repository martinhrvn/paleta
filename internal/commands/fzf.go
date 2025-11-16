package commands

import (
	"fmt"
	"regexp"
	"strings"

	fuzzyfinder "github.com/ktr0731/go-fuzzyfinder"
	"github.com/martin/go-pm/internal/config"
	"github.com/martin/go-pm/internal/ui"
)

// SelectionResult represents the result of a user selection from fzf
type SelectionResult struct {
	Directory   string // The actual directory path where command should be executed
	Command     string // The command to run
	DisplayName string // The display name shown in fzf (for reference)
}

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

	return &SelectionResult{
		Directory:   location.Location,
		Command:     command,
		DisplayName: displayName,
	}, nil
}

// CommandInfo holds information about a command for display
type CommandInfo struct {
	Display     string
	Directory   string
	Command     string
	DisplayName string
}

// PrepareCommandInfo prepares command information for fuzzy finder
func PrepareCommandInfo(cfg *config.Config) []CommandInfo {
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
			info := CommandInfo{
				Display:     fmt.Sprintf("%s: %s", displayName, command),
				Directory:   location.Location,
				Command:     command,
				DisplayName: displayName,
			}
			infos = append(infos, info)
		}
	}

	return infos
}

// RunFzf executes fuzzy finder with the given options and returns the user's selection
func RunFzf(cfg *config.Config) (*SelectionResult, error) {
	// Prepare command information
	commandInfos := PrepareCommandInfo(cfg)
	if len(commandInfos) == 0 {
		return nil, fmt.Errorf("no commands available")
	}

	// Use fuzzyfinder to let user select
	idx, err := fuzzyfinder.Find(
		commandInfos,
		func(i int) string {
			return commandInfos[i].Display
		},
		fuzzyfinder.WithPromptString("Select command: "),
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			info := commandInfos[i]
			preview := fmt.Sprintf("Directory: %s\nCommand: %s", info.Directory, info.Command)
			return preview
		}),
	)

	if err != nil {
		// Check if user canceled
		if err == fuzzyfinder.ErrAbort {
			return nil, fmt.Errorf("selection canceled")
		}
		return nil, fmt.Errorf("fuzzy finder error: %w", err)
	}

	selected := commandInfos[idx]
	return &SelectionResult{
		Directory:   selected.Directory,
		Command:     selected.Command,
		DisplayName: selected.DisplayName,
	}, nil
}

// RunEnhancedFzf executes the enhanced fuzzy finder with location filtering support
func RunEnhancedFzf(cfg *config.Config) (*SelectionResult, error) {
	selector := ui.NewTUISelector(cfg)
	result, err := selector.Run()
	if err != nil {
		return nil, err
	}
	// Convert ui.SelectionResult to commands.SelectionResult
	return &SelectionResult{
		Directory:   result.Directory,
		Command:     result.Command,
		DisplayName: result.DisplayName,
	}, nil
}
