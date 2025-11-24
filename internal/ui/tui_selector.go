package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/martin/go-pm/internal/config"
	"github.com/martin/go-pm/internal/history"
	"github.com/rivo/tview"
)

// TUISelector provides a TUI interface for command selection with location filtering
type TUISelector struct {
	config            *config.Config
	app               *tview.Application
	commandList       *tview.List
	locationList      *tview.List
	searchInput       *tview.InputField
	statusText        *tview.TextView
	helpText          *tview.TextView
	selectedLocations map[string]bool
	commands          []CommandInfo
	filteredCommands  []CommandInfo
	searchQuery       string
	result            *SelectionResult
	history           *history.History
	frecencyEnabled   bool
}

// NewTUISelector creates a new TUI selector
func NewTUISelector(cfg *config.Config) *TUISelector {
	selector := &TUISelector{
		config:            cfg,
		app:               tview.NewApplication(),
		selectedLocations: make(map[string]bool),
		frecencyEnabled:   cfg.Frecency.Enabled,
	}

	// Load history if frecency is enabled
	if selector.frecencyEnabled {
		projectRoot, err := history.FindProjectRoot(".")
		if err == nil {
			selector.history, _ = history.LoadOrCreateHistory(projectRoot)
		}
	}

	return selector
}

// Run starts the TUI selector
func (s *TUISelector) Run() (*SelectionResult, error) {
	// Initialize the UI
	s.initUI()

	// Load commands and update display
	s.loadCommands()
	s.updateFilteredCommands()

	// Run the application
	if err := s.app.Run(); err != nil {
		return nil, err
	}

	if s.result == nil {
		return nil, fmt.Errorf("selection canceled")
	}

	return s.result, nil
}

func (s *TUISelector) initUI() {
	// Create search input (fzf-like, at the bottom)
	s.searchInput = tview.NewInputField().
		SetLabel("> ").
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetLabelColor(tcell.ColorBlue)

	// Create command list (no border, minimal style)
	s.commandList = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextColor(tcell.ColorWhite).
		SetSelectedBackgroundColor(tcell.ColorBlue).
		SetSelectedTextColor(tcell.ColorBlack)

	// Create location list (minimal, no border)
	s.locationList = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextColor(tcell.ColorWhite).
		SetSelectedBackgroundColor(tcell.ColorDarkBlue).
		SetSelectedTextColor(tcell.ColorWhite)

	// Create status text (single line, minimal)
	s.statusText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextColor(tcell.ColorDarkGray)

	// Create help text (single line at bottom)
	s.helpText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextColor(tcell.ColorDarkGray).
		SetText("  [::d]Tab[-]: switch  [::d]Ctrl+L[-]: clear  [::d]Ctrl+F[-]: frecency  [::d]Esc[-]: cancel")

	// Set up search input handler
	s.searchInput.SetChangedFunc(func(text string) {
		s.searchQuery = text
		s.updateFilteredCommands()
	})

	// Set up search input key handler for arrow navigation
	s.searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyDown:
			// Move focus to command list and select first item
			s.app.SetFocus(s.commandList)
			if s.commandList.GetItemCount() > 0 {
				s.commandList.SetCurrentItem(0)
			}
			return nil
		case tcell.KeyUp:
			// Move focus to command list and select last item
			s.app.SetFocus(s.commandList)
			itemCount := s.commandList.GetItemCount()
			if itemCount > 0 {
				s.commandList.SetCurrentItem(itemCount - 1)
			}
			return nil
		case tcell.KeyEnter:
			// If there are filtered commands, select the first one
			if len(s.filteredCommands) > 0 {
				cmd := s.filteredCommands[0]
				s.result = &SelectionResult{
					Directory:   cmd.Directory,
					Command:     cmd.Command,
					DisplayName: cmd.DisplayName,
				}
				s.app.Stop()
			}
			return nil
		}
		return event
	})

	// Populate location list
	for _, loc := range s.config.Locations {
		displayName := loc.Name
		if displayName == "" {
			displayName = loc.Location
		}
		s.locationList.AddItem(displayName, "", 0, nil)
		// Select all locations by default
		s.selectedLocations[displayName] = true
	}

	// Update location display
	s.updateLocationDisplay()

	// Set up command selection handler
	s.commandList.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(s.filteredCommands) {
			cmd := s.filteredCommands[index]
			s.result = &SelectionResult{
				Directory:   cmd.Directory,
				Command:     cmd.Command,
				DisplayName: cmd.DisplayName,
			}
			s.app.Stop()
		}
	})

	// Set up command list key handler for navigation back to search
	s.commandList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			// If at the top of the list, go back to search input
			if s.commandList.GetCurrentItem() == 0 {
				s.app.SetFocus(s.searchInput)
				return nil
			}
		case tcell.KeyRune:
			// If typing a character, go back to search input and add the character
			if event.Rune() != ' ' { // Don't interfere with space for location toggling
				s.app.SetFocus(s.searchInput)
				// Add the character to the search input
				currentText := s.searchInput.GetText()
				s.searchInput.SetText(currentText + string(event.Rune()))
				return nil
			}
		}
		return event
	})

	// Set up location toggle handler
	s.locationList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && event.Rune() == ' ' {
			currentIndex := s.locationList.GetCurrentItem()
			if currentIndex >= 0 && currentIndex < len(s.config.Locations) {
				// Get the actual location display name
				location := s.config.Locations[currentIndex]
				displayName := location.Name
				if displayName == "" {
					displayName = location.Location
				}

				// Toggle selection
				s.selectedLocations[displayName] = !s.selectedLocations[displayName]

				// Update display
				s.updateLocationDisplay()
				s.updateFilteredCommands()
			}

			return nil // Consume the event
		}
		return event
	})

	// Create layout (fzf-like: status at top, list in middle, search/help at bottom)
	// Create a separator
	separator := tview.NewBox().
		SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
			// Draw vertical line
			for row := y; row < y+height; row++ {
				screen.SetContent(x, row, '│', nil, tcell.StyleDefault.Foreground(tcell.ColorDarkGray))
			}
			return x + 1, y, width - 1, height
		})

	// Left side: compact location list with title
	locationTitle := tview.NewTextView().
		SetText("  LOCATIONS").
		SetTextColor(tcell.ColorDarkGray)

	leftPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(locationTitle, 1, 0, false).
		AddItem(s.locationList, 0, 1, false)

	// Right side: commands with search at bottom
	rightPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(s.commandList, 0, 1, false).
		AddItem(s.searchInput, 1, 0, false)

	// Main area: location list (narrow) + separator + commands
	mainPanel := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(leftPanel, 24, 0, true).
		AddItem(separator, 1, 0, false).
		AddItem(rightPanel, 0, 1, false)

	// Root: status at top, main area, help at bottom
	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(s.statusText, 1, 0, false).
		AddItem(mainPanel, 0, 1, true).
		AddItem(s.helpText, 1, 0, false)

	// Set up global key handlers
	s.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			// Switch focus between panels
			current := s.app.GetFocus()
			if current == s.locationList {
				s.app.SetFocus(s.searchInput)
			} else if current == s.searchInput {
				s.app.SetFocus(s.commandList)
			} else {
				s.app.SetFocus(s.locationList)
			}
			return nil
		case tcell.KeyCtrlL:
			// Clear all location filters and search
			for key := range s.selectedLocations {
				s.selectedLocations[key] = true
			}
			s.searchInput.SetText("")
			s.searchQuery = ""
			s.updateLocationDisplay()
			s.updateFilteredCommands()
			// Return focus to search input
			s.app.SetFocus(s.searchInput)
			return nil
		case tcell.KeyCtrlU:
			// Clear search input (like fzf)
			s.searchInput.SetText("")
			s.searchQuery = ""
			s.updateFilteredCommands()
			s.app.SetFocus(s.searchInput)
			return nil
		case tcell.KeyCtrlF:
			// Toggle frecency sorting
			s.frecencyEnabled = !s.frecencyEnabled
			s.updateFilteredCommands()
			return nil
		case tcell.KeyEscape, tcell.KeyCtrlC:
			s.app.Stop()
			return nil
		}
		return event
	})

	s.app.SetRoot(root, true).SetFocus(s.searchInput)
}

func (s *TUISelector) loadCommands() {
	s.commands = []CommandInfo{}

	for _, location := range s.config.Locations {
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

			// Calculate frecency score if history is available
			var score float64
			if s.history != nil && s.frecencyEnabled {
				score = s.history.GetScore(displayName, command.Command)
			}

			info := CommandInfo{
				Display:       fmt.Sprintf("%s: %s", displayName, cmdDisplay),
				Directory:     location.Location,
				Command:       command.Command,
				DisplayName:   displayName,
				FrecencyScore: score,
			}
			s.commands = append(s.commands, info)
		}
	}
}

func (s *TUISelector) updateFilteredCommands() {
	// Recalculate frecency scores if frecency was just enabled
	if s.frecencyEnabled && s.history != nil {
		for i := range s.commands {
			s.commands[i].FrecencyScore = s.history.GetScore(
				s.commands[i].DisplayName,
				s.commands[i].Command,
			)
		}
	}

	s.filteredCommands = []CommandInfo{}
	s.commandList.Clear()

	// Check if any locations are selected
	anySelected := false
	for _, selected := range s.selectedLocations {
		if selected {
			anySelected = true
			break
		}
	}

	// Filter by selected locations first
	var locationFiltered []CommandInfo
	if !anySelected {
		locationFiltered = s.commands
	} else {
		for _, cmd := range s.commands {
			if s.selectedLocations[cmd.DisplayName] {
				locationFiltered = append(locationFiltered, cmd)
			}
		}
	}

	// Then apply fuzzy search filter
	if s.searchQuery == "" {
		s.filteredCommands = locationFiltered
	} else {
		s.filteredCommands = s.fuzzyFilter(locationFiltered, s.searchQuery)
	}

	// Sort by frecency score if enabled
	if s.frecencyEnabled && s.history != nil {
		sort.Slice(s.filteredCommands, func(i, j int) bool {
			// Higher scores first
			return s.filteredCommands[i].FrecencyScore > s.filteredCommands[j].FrecencyScore
		})
	}

	// Update command list display
	for _, cmd := range s.filteredCommands {
		s.commandList.AddItem(cmd.Display, "", 0, nil)
	}

	// Update status
	s.updateStatus()
}

// fuzzyFilter performs fuzzy matching on commands
func (s *TUISelector) fuzzyFilter(commands []CommandInfo, query string) []CommandInfo {
	if query == "" {
		return commands
	}

	query = strings.ToLower(query)
	var filtered []CommandInfo

	for _, cmd := range commands {
		if s.fuzzyMatch(strings.ToLower(cmd.Display), query) {
			filtered = append(filtered, cmd)
		}
	}

	return filtered
}

// fuzzyMatch checks if query matches text with fuzzy matching
func (s *TUISelector) fuzzyMatch(text, query string) bool {
	if query == "" {
		return true
	}

	textIdx := 0
	queryIdx := 0

	for textIdx < len(text) && queryIdx < len(query) {
		if text[textIdx] == query[queryIdx] {
			queryIdx++
		}
		textIdx++
	}

	return queryIdx == len(query)
}

func (s *TUISelector) updateLocationDisplay() {
	s.locationList.Clear()

	for _, loc := range s.config.Locations {
		displayName := loc.Name
		if displayName == "" {
			displayName = loc.Location
		}

		// Minimal indicator (like fzf multi-select)
		prefix := "  "
		if s.selectedLocations[displayName] {
			prefix = "• "
		}

		s.locationList.AddItem(fmt.Sprintf("%s%s", prefix, displayName), "", 0, nil)
	}
}

func (s *TUISelector) updateStatus() {
	selectedCount := 0
	var selectedNames []string

	for name, selected := range s.selectedLocations {
		if selected {
			selectedCount++
			selectedNames = append(selectedNames, name)
		}
	}

	// Compact single-line status like fzf
	var parts []string

	// Count
	parts = append(parts, fmt.Sprintf("[::d]%d/%d[-]", len(s.filteredCommands), len(s.commands)))

	// Location filter (if not all)
	if selectedCount > 0 && selectedCount < len(s.config.Locations) {
		locationText := strings.Join(selectedNames, ",")
		if len(locationText) > 30 {
			locationText = locationText[:27] + "..."
		}
		parts = append(parts, fmt.Sprintf("[green]%s[-]", locationText))
	}

	// Search query (if any)
	if s.searchQuery != "" {
		queryText := s.searchQuery
		if len(queryText) > 20 {
			queryText = queryText[:17] + "..."
		}
		parts = append(parts, fmt.Sprintf("[yellow]'%s'[-]", queryText))
	}

	// Frecency indicator
	if s.frecencyEnabled {
		parts = append(parts, "[blue]⚡[-]")
	}

	s.statusText.SetText("  " + strings.Join(parts, "  "))
}
