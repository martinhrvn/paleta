package commands

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	fuzzyfinder "github.com/ktr0731/go-fuzzyfinder"
	"github.com/martin/go-pm/internal/config"
	"github.com/martin/go-pm/internal/history"
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
	Display       string
	Directory     string
	Command       string
	DisplayName   string
	FrecencyScore float64 // Score for sorting
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

// RunFzf executes fuzzy finder with multi-select support and returns the user's selections
func RunFzf(cfg *config.Config) ([]SelectionResult, error) {
	// Load history if frecency is enabled
	var hist *history.History
	enableFrecency := cfg.Frecency.Enabled

	if enableFrecency {
		// Find project root
		projectRoot, err := history.FindProjectRoot(".")
		if err == nil {
			// Load or create history
			hist, _ = history.LoadOrCreateHistory(projectRoot)
		}
	}

	// Prepare command information with frecency sorting
	commandInfos := PrepareCommandInfoWithHistory(cfg, hist, enableFrecency)
	if len(commandInfos) == 0 {
		return nil, fmt.Errorf("no commands available")
	}

	// Use fuzzyfinder with multi-select (Tab to toggle, Enter to confirm)
	indices, err := fuzzyfinder.FindMulti(
		commandInfos,
		func(i int) string {
			return commandInfos[i].Display
		},
		fuzzyfinder.WithPromptString("Select commands (Tab to toggle): "),
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

	if len(indices) == 0 {
		return nil, fmt.Errorf("no commands selected")
	}

	// Convert indices to SelectionResults (preserving selection order)
	results := make([]SelectionResult, len(indices))
	for i, idx := range indices {
		selected := commandInfos[idx]
		results[i] = SelectionResult{
			Directory:   selected.Directory,
			Command:     selected.Command,
			DisplayName: selected.DisplayName,
		}
	}

	return results, nil
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

// RunFzfTUI executes the new fzf-style TUI with multi-select support
func RunFzfTUI(cfg *config.Config) ([]SelectionResult, error) {
	selector := ui.NewFzfTUISelector(cfg)
	results, err := selector.Run()
	if err != nil {
		return nil, err
	}

	// Convert ui.SelectionResult slice to commands.SelectionResult slice
	cmdResults := make([]SelectionResult, len(results))
	for i, r := range results {
		cmdResults[i] = SelectionResult{
			Directory:   r.Directory,
			Command:     r.Command,
			DisplayName: r.DisplayName,
		}
	}

	return cmdResults, nil
}
