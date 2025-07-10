package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/martin/go-pm/internal/config"
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
}

// NewTUISelector creates a new TUI selector
func NewTUISelector(cfg *config.Config) *TUISelector {
	return &TUISelector{
		config:            cfg,
		app:               tview.NewApplication(),
		selectedLocations: make(map[string]bool),
	}
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
	// Create search input
	s.searchInput = tview.NewInputField().
		SetLabel("Search: ").
		SetFieldWidth(0).
		SetPlaceholder("Type to filter commands...")

	s.searchInput.SetBorder(true).
		SetBorderColor(tcell.ColorDarkGray)

	// Create command list
	s.commandList = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetSelectedBackgroundColor(tcell.ColorDarkCyan).
		SetSelectedTextColor(tcell.ColorWhite)

	s.commandList.SetBorder(true).
		SetBorderColor(tcell.ColorDarkGray)

	// Create location list
	s.locationList = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetSelectedBackgroundColor(tcell.ColorDarkBlue).
		SetSelectedTextColor(tcell.ColorWhite)

	s.locationList.SetBorder(true).
		SetBorderColor(tcell.ColorDarkGray)

	// Create status text
	s.statusText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	s.statusText.SetBorder(true).
		SetBorderColor(tcell.ColorDarkGray)

	// Create help text
	s.helpText = tview.NewTextView().
		SetDynamicColors(true).
		SetText("[yellow]↑/↓[white]: Navigate | [yellow]Tab[white]: Switch panels | [yellow]Enter[white]: Select | [yellow]Ctrl+U[white]: Clear search | [yellow]Ctrl+L[white]: Clear all | [yellow]Esc/Ctrl+C[white]: Cancel")

	s.helpText.SetBorder(true).
		SetBorderColor(tcell.ColorDarkGray)

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

	// Create layout
	leftPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(s.locationList, 0, 1, false).
		AddItem(s.statusText, 3, 0, false)

	rightPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(s.searchInput, 3, 0, false).
		AddItem(s.commandList, 0, 1, false)

	mainPanel := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(leftPanel, 30, 0, true).
		AddItem(rightPanel, 0, 1, false)

	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(mainPanel, 0, 1, true).
		AddItem(s.helpText, 3, 0, false)

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
			info := CommandInfo{
				Display:     fmt.Sprintf("%s: %s", displayName, command),
				Directory:   location.Location,
				Command:     command,
				DisplayName: displayName,
			}
			s.commands = append(s.commands, info)
		}
	}
}

func (s *TUISelector) updateFilteredCommands() {
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

		prefix := "[ ]"
		if s.selectedLocations[displayName] {
			prefix = "[✓]"
		}

		s.locationList.AddItem(fmt.Sprintf("%s %s", prefix, displayName), "", 0, nil)
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

	var locationStatus string
	if selectedCount == 0 || selectedCount == len(s.config.Locations) {
		locationStatus = "[green]All locations[white]"
	} else {
		locationStatus = fmt.Sprintf("[green]%s[white]", strings.Join(selectedNames, ", "))
	}

	searchStatus := ""
	if s.searchQuery != "" {
		searchStatus = fmt.Sprintf("\nSearch: [yellow]%s[white]", s.searchQuery)
	}

	commandCount := fmt.Sprintf("\nShowing: [cyan]%d[white] commands", len(s.filteredCommands))

	s.statusText.SetText(fmt.Sprintf("Active: %s%s%s", locationStatus, searchStatus, commandCount))
}
