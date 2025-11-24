package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	fuzzyfinder "github.com/ktr0731/go-fuzzyfinder"
	"github.com/martin/go-pm/internal/config"
)

// SelectionResult represents the result of a user selection
type SelectionResult struct {
	Directory   string // The actual directory path where command should be executed
	Command     string // The command to run
	DisplayName string // The display name shown in fzf (for reference)
}

// CommandInfo holds information about a command for display
type CommandInfo struct {
	Display       string
	Directory     string
	Command       string
	DisplayName   string
	Type          string  // Project type (npm, go, etc.)
	FrecencyScore float64 // Score for sorting
}

// EnhancedSelector provides command selection with location filtering
type EnhancedSelector struct {
	config            *config.Config
	selectedLocations []string // Empty means all locations
}

// NewEnhancedSelector creates a new enhanced selector
func NewEnhancedSelector(cfg *config.Config) *EnhancedSelector {
	return &EnhancedSelector{
		config:            cfg,
		selectedLocations: []string{}, // Start with all locations
	}
}

// Run starts the enhanced selector with location filtering support
func (s *EnhancedSelector) Run() (*SelectionResult, error) {
	for {
		// Run command selector
		result, err := s.runCommandSelector()
		if err != nil {
			// Check if it's a special error for location selection
			if err.Error() == "LOCATION_SELECT" {
				// Run location selector
				if err := s.selectLocations(); err != nil {
					return nil, err
				}
				// Continue the loop to show command selector again
				continue
			}
			return nil, err
		}
		return result, nil
	}
}

// runCommandSelector runs the command selector with current filters
func (s *EnhancedSelector) runCommandSelector() (*SelectionResult, error) {
	// Prepare filtered command information
	commandInfos := s.prepareFilteredCommands()
	if len(commandInfos) == 0 {
		return nil, fmt.Errorf("no commands available for selected locations")
	}

	// Add a special entry for location selection
	locationEntry := CommandInfo{
		Display:     ">>> Change Location Filter <<<",
		Directory:   "",
		Command:     "",
		DisplayName: "",
	}
	commandInfos = append([]CommandInfo{locationEntry}, commandInfos...)

	// Use fuzzyfinder to let user select
	idx, err := fuzzyfinder.Find(
		commandInfos,
		func(i int) string {
			return commandInfos[i].Display
		},
		fuzzyfinder.WithPromptString(s.getPromptString()),
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 || i == 0 { // First entry is location selector
				return "Select this to change location filter\n\nCurrently selected: " + s.getLocationString()
			}
			info := commandInfos[i]
			preview := fmt.Sprintf("Location: %s\nPath:     %s\nCommand:  %s",
				info.DisplayName,
				info.Directory,
				info.Command)

			// Add type if available
			if info.Type != "" {
				preview = fmt.Sprintf("%s\nType:     %s", preview, info.Type)
			}

			return preview
		}),
		fuzzyfinder.WithHeader(s.getHeaderString()),
	)

	if err != nil {
		if err == fuzzyfinder.ErrAbort {
			return nil, fmt.Errorf("selection canceled")
		}
		return nil, fmt.Errorf("fuzzy finder error: %w", err)
	}

	// Check if location selector was chosen
	if idx == 0 {
		return nil, fmt.Errorf("LOCATION_SELECT")
	}

	selected := commandInfos[idx]
	return &SelectionResult{
		Directory:   selected.Directory,
		Command:     selected.Command,
		DisplayName: selected.DisplayName,
	}, nil
}

// prepareFilteredCommands prepares command list filtered by selected locations
func (s *EnhancedSelector) prepareFilteredCommands() []CommandInfo {
	var infos []CommandInfo

	for _, location := range s.config.Locations {
		// Skip if location is not in selected locations (unless all are selected)
		if len(s.selectedLocations) > 0 && !s.isLocationSelected(location) {
			continue
		}

		displayName := location.Name
		if displayName == "" {
			displayName = location.Location
		}

		for _, command := range location.Commands {
			// Use command name if available, otherwise use full command
			cmdDisplay := command.Name
			if cmdDisplay == "" {
				cmdDisplay = command.Command
			}

			info := CommandInfo{
				Display:     fmt.Sprintf("%s: %s", displayName, cmdDisplay),
				Directory:   location.Location,
				Command:     command.Command,
				DisplayName: displayName,
				Type:        location.Type,
			}
			infos = append(infos, info)
		}
	}

	return infos
}

// isLocationSelected checks if a location is in the selected list
func (s *EnhancedSelector) isLocationSelected(location config.Location) bool {
	displayName := location.Name
	if displayName == "" {
		displayName = location.Location
	}

	for _, selected := range s.selectedLocations {
		if selected == displayName {
			return true
		}
	}
	return false
}

// getPromptString returns the prompt string
func (s *EnhancedSelector) getPromptString() string {
	return "Select command: "
}

// getHeaderString returns the header string for fuzzyfinder
func (s *EnhancedSelector) getHeaderString() string {
	return fmt.Sprintf("=== Locations: %s ===\nTip: Select first entry to change location filter", s.getLocationString())
}

// getLocationString returns a string representation of selected locations
func (s *EnhancedSelector) getLocationString() string {
	if len(s.selectedLocations) == 0 {
		return "All"
	}
	return strings.Join(s.selectedLocations, ", ")
}

// selectLocations shows a location selector and updates selected locations
func (s *EnhancedSelector) selectLocations() error {
	// Clear screen before location selection
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Run()

	// Prepare location names for selection
	var locations []string
	for _, loc := range s.config.Locations {
		displayName := loc.Name
		if displayName == "" {
			displayName = loc.Location
		}
		locations = append(locations, displayName)
	}

	// Use fuzzyfinder for multi-selection
	selected, err := fuzzyfinder.FindMulti(
		locations,
		func(i int) string {
			return locations[i]
		},
		fuzzyfinder.WithPromptString("Select locations (Tab to toggle, Enter to confirm): "),
		fuzzyfinder.WithPreselected(func(i int) bool {
			// Preselect currently selected locations
			return s.isLocationInList(locations[i], s.selectedLocations)
		}),
		fuzzyfinder.WithHeader("=== Location Filter ===\nSelect which locations to include in command search"),
	)

	if err != nil {
		if err == fuzzyfinder.ErrAbort {
			// User canceled, keep current selection
			return nil
		}
		return fmt.Errorf("location selection error: %w", err)
	}

	// Update selected locations
	if len(selected) == 0 {
		// If nothing selected, show all locations
		s.selectedLocations = []string{}
	} else {
		s.selectedLocations = make([]string, len(selected))
		for i, idx := range selected {
			s.selectedLocations[i] = locations[idx]
		}
	}

	return nil
}

// isLocationInList checks if a location is in a list
func (s *EnhancedSelector) isLocationInList(location string, list []string) bool {
	for _, item := range list {
		if item == location {
			return true
		}
	}
	return false
}
