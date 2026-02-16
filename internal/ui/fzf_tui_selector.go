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

// FzfTUISelector provides an fzf-style TUI interface for command selection with multi-select
type FzfTUISelector struct {
	config           *config.Config
	app              *tview.Application
	commandList      *tview.List
	previewPanel     *tview.TextView
	searchInput      *tview.InputField
	statusText       *tview.TextView
	helpText         *tview.TextView
	editInput        *tview.InputField
	rootLayout       *tview.Flex
	mainPanel        *tview.Flex
	commands         []CommandInfo
	filteredCommands []CommandInfo
	selectedIndices  map[int]bool // Track selected items by filtered index
	currentIndex     int          // Current cursor position
	searchQuery      string
	results          []SelectionResult
	history          *history.History
	frecencyEnabled  bool
	editing          bool   // Whether we're in edit mode
	editCommand      string // Command being edited
	editDirectory    string // Directory for the command being edited
	editDisplayName  string // Display name for the command being edited
}

// NewFzfTUISelector creates a new fzf-style TUI selector
func NewFzfTUISelector(cfg *config.Config) *FzfTUISelector {
	selector := &FzfTUISelector{
		config:          cfg,
		app:             tview.NewApplication(),
		selectedIndices: make(map[int]bool),
		frecencyEnabled: cfg.Frecency.Enabled,
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

// Run starts the fzf-style TUI selector and returns selected commands
func (s *FzfTUISelector) Run() ([]SelectionResult, error) {
	// Initialize UI
	s.initUI()

	// Load and display commands
	s.loadCommands()
	s.updateFilteredCommands()

	// Run the application
	if err := s.app.Run(); err != nil {
		return nil, err
	}

	if len(s.results) == 0 {
		return nil, fmt.Errorf("selection canceled")
	}

	return s.results, nil
}

func (s *FzfTUISelector) initUI() {
	// Create search input (fzf-like, at the bottom)
	s.searchInput = tview.NewInputField().
		SetLabel("> ").
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetLabelColor(tcell.ColorBlue)

	// Create command list
	s.commandList = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextColor(tcell.ColorWhite).
		SetSelectedBackgroundColor(tcell.ColorBlue).
		SetSelectedTextColor(tcell.ColorBlack)

	// Create preview panel
	s.previewPanel = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	s.previewPanel.SetBorder(true).
		SetBorderColor(tcell.ColorDarkGray).
		SetTitle(" Preview ")

	// Create status text
	s.statusText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextColor(tcell.ColorDarkGray)

	// Create help text
	s.helpText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextColor(tcell.ColorDarkGray).
		SetText("  [::d]Tab[-]: select  [::d]Enter[-]: run  [::d]Ctrl+E[-]: edit  [::d]Ctrl+A[-]: all  [::d]Esc[-]: cancel")

	// Set up search input handler
	s.searchInput.SetChangedFunc(func(text string) {
		s.searchQuery = text
		s.updateFilteredCommands()
	})

	// Set up search input key handler
	s.searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyDown, tcell.KeyCtrlJ:
			// Move down in list
			s.moveCursorDown()
			return nil
		case tcell.KeyUp, tcell.KeyCtrlK:
			// Move up in list
			s.moveCursorUp()
			return nil
		case tcell.KeyEnter:
			// Confirm selection
			s.confirmSelection()
			return nil
		case tcell.KeyCtrlE:
			// Confirm selection for editing (place on prompt without executing)
			s.enterEditMode()
			return nil
		case tcell.KeyTab:
			// Toggle selection on current item
			s.toggleSelection(s.currentIndex)
			s.moveCursorDown() // Move to next item after toggle
			return nil
		}
		return event
	})

	// Set up command list selection handler (for mouse/direct selection)
	s.commandList.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		s.currentIndex = index
		s.updatePreview()
	})

	// Set up command list key handler
	s.commandList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyDown, tcell.KeyCtrlJ:
			s.moveCursorDown()
			return nil
		case tcell.KeyUp, tcell.KeyCtrlK:
			s.moveCursorUp()
			return nil
		case tcell.KeyEnter:
			s.confirmSelection()
			return nil
		case tcell.KeyCtrlE:
			s.enterEditMode()
			return nil
		case tcell.KeyTab:
			s.toggleSelection(s.currentIndex)
			s.moveCursorDown()
			return nil
		case tcell.KeyRune:
			// Redirect typing to search input
			s.app.SetFocus(s.searchInput)
			currentText := s.searchInput.GetText()
			s.searchInput.SetText(currentText + string(event.Rune()))
			return nil
		}
		return event
	})

	// Create edit input (shown when editing a command)
	s.editInput = tview.NewInputField().
		SetLabel("Edit> ").
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetLabelColor(tcell.ColorYellow)

	s.editInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			s.editCommand = s.editInput.GetText()
			s.confirmEdit()
			return nil
		case tcell.KeyEscape:
			s.cancelEdit()
			return nil
		}
		return event
	})

	// Create layout: command list (70%) + preview panel (30%)
	s.mainPanel = tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(s.commandList, 0, 7, true).
		AddItem(s.previewPanel, 0, 3, false)

	// Root layout: search input, status, main panel, help
	s.rootLayout = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(s.searchInput, 1, 0, false).
		AddItem(s.statusText, 1, 0, false).
		AddItem(s.mainPanel, 0, 1, true).
		AddItem(s.helpText, 1, 0, false)

	// Global key handlers
	s.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlA:
			// Select/deselect all
			s.toggleSelectAll()
			return nil
		case tcell.KeyCtrlL:
			// Clear search
			s.searchInput.SetText("")
			s.searchQuery = ""
			s.updateFilteredCommands()
			s.app.SetFocus(s.searchInput)
			return nil
		case tcell.KeyCtrlU:
			// Clear search input
			s.searchInput.SetText("")
			s.searchQuery = ""
			s.updateFilteredCommands()
			s.app.SetFocus(s.searchInput)
			return nil
		case tcell.KeyCtrlF:
			// Toggle frecency
			s.frecencyEnabled = !s.frecencyEnabled
			s.updateFilteredCommands()
			return nil
		case tcell.KeyEscape, tcell.KeyCtrlC:
			s.app.Stop()
			return nil
		}
		return event
	})

	s.app.SetRoot(s.rootLayout, true).SetFocus(s.searchInput)
}

func (s *FzfTUISelector) loadCommands() {
	s.commands = []CommandInfo{}

	for _, location := range s.config.Locations {
		displayName := location.Name
		if displayName == "" {
			displayName = location.Location
		}

		for _, command := range location.Commands {
			cmdDisplay := command.Name
			if cmdDisplay == "" {
				cmdDisplay = command.Command
			}

			var score float64
			if s.history != nil && s.frecencyEnabled {
				score = s.history.GetScore(displayName, command.Command)
			}

			info := CommandInfo{
				Display:       fmt.Sprintf("%s: %s", displayName, cmdDisplay),
				Directory:     location.Location,
				Command:       command.Command,
				DisplayName:   displayName,
				Type:          location.Type,
				FrecencyScore: score,
			}
			s.commands = append(s.commands, info)
		}
	}
}

func (s *FzfTUISelector) updateFilteredCommands() {
	// Recalculate frecency scores if enabled
	if s.frecencyEnabled && s.history != nil {
		for i := range s.commands {
			s.commands[i].FrecencyScore = s.history.GetScore(
				s.commands[i].DisplayName,
				s.commands[i].Command,
			)
		}
	}

	// Clear selections when filter changes (indices won't match)
	s.clearSelections()

	// Apply fuzzy filter
	if s.searchQuery == "" {
		s.filteredCommands = make([]CommandInfo, len(s.commands))
		copy(s.filteredCommands, s.commands)
	} else {
		s.filteredCommands = s.fuzzyFilter(s.commands, s.searchQuery)
	}

	// Sort by frecency if enabled
	if s.frecencyEnabled && s.history != nil {
		sort.Slice(s.filteredCommands, func(i, j int) bool {
			return s.filteredCommands[i].FrecencyScore > s.filteredCommands[j].FrecencyScore
		})
	}

	// Update command list display
	s.updateCommandListDisplay()

	// Reset cursor to top
	s.currentIndex = 0
	if s.commandList != nil && s.commandList.GetItemCount() > 0 {
		s.commandList.SetCurrentItem(0)
	}

	// Update status and preview
	s.updateStatus()
	s.updatePreview()
}

func (s *FzfTUISelector) updateCommandListDisplay() {
	if s.commandList == nil {
		return // Skip UI update in tests
	}
	s.commandList.Clear()
	for i, cmd := range s.filteredCommands {
		isSelected := s.selectedIndices[i]
		text := s.formatListItem(i, isSelected)
		// Store index but don't use the text directly since we format it
		s.commandList.AddItem(text, "", 0, nil)
		_ = cmd // Use cmd to avoid unused warning
	}
}

func (s *FzfTUISelector) formatListItem(index int, selected bool) string {
	if index < 0 || index >= len(s.filteredCommands) {
		return ""
	}
	prefix := "  "
	if selected {
		prefix = "* "
	}
	return prefix + s.filteredCommands[index].Display
}

func (s *FzfTUISelector) fuzzyFilter(commands []CommandInfo, query string) []CommandInfo {
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

func (s *FzfTUISelector) fuzzyMatch(text, query string) bool {
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

func (s *FzfTUISelector) toggleSelection(index int) {
	if index < 0 || index >= len(s.filteredCommands) {
		return
	}
	s.selectedIndices[index] = !s.selectedIndices[index]
	// Remove false entries to keep map clean
	if !s.selectedIndices[index] {
		delete(s.selectedIndices, index)
	}
	s.updateCommandListDisplay()
	s.updateStatus()
	// Restore cursor position after display update
	if s.commandList != nil && s.commandList.GetItemCount() > 0 && index < s.commandList.GetItemCount() {
		s.commandList.SetCurrentItem(index)
	}
}

func (s *FzfTUISelector) toggleSelectAll() {
	// If all are selected, deselect all; otherwise select all
	if s.getSelectedCount() == len(s.filteredCommands) {
		s.clearSelections()
	} else {
		for i := range s.filteredCommands {
			s.selectedIndices[i] = true
		}
	}
	s.updateCommandListDisplay()
	s.updateStatus()
	// Restore cursor position
	if s.commandList != nil && s.commandList.GetItemCount() > 0 && s.currentIndex < s.commandList.GetItemCount() {
		s.commandList.SetCurrentItem(s.currentIndex)
	}
}

func (s *FzfTUISelector) clearSelections() {
	s.selectedIndices = make(map[int]bool)
}

func (s *FzfTUISelector) getSelectedCount() int {
	count := 0
	for _, selected := range s.selectedIndices {
		if selected {
			count++
		}
	}
	return count
}

func (s *FzfTUISelector) getSelectedCommands() []SelectionResult {
	var results []SelectionResult

	// Collect in order of appearance in filtered list
	for i := 0; i < len(s.filteredCommands); i++ {
		if s.selectedIndices[i] {
			cmd := s.filteredCommands[i]
			results = append(results, SelectionResult{
				Directory:   cmd.Directory,
				Command:     cmd.Command,
				DisplayName: cmd.DisplayName,
			})
		}
	}

	// If nothing explicitly selected, return current item
	if len(results) == 0 && s.currentIndex >= 0 && s.currentIndex < len(s.filteredCommands) {
		cmd := s.filteredCommands[s.currentIndex]
		results = append(results, SelectionResult{
			Directory:   cmd.Directory,
			Command:     cmd.Command,
			DisplayName: cmd.DisplayName,
		})
	}

	return results
}

func (s *FzfTUISelector) confirmSelection() {
	s.results = s.getSelectedCommands()
	s.app.Stop()
}

func (s *FzfTUISelector) enterEditMode() {
	if len(s.filteredCommands) == 0 || s.currentIndex < 0 || s.currentIndex >= len(s.filteredCommands) {
		return
	}

	cmd := s.filteredCommands[s.currentIndex]
	s.editing = true
	s.editCommand = cmd.Command
	s.editDirectory = cmd.Directory
	s.editDisplayName = cmd.DisplayName

	if s.editInput != nil {
		s.editInput.SetText(cmd.Command)
		s.showEditLayout()
		s.app.SetFocus(s.editInput)
	}
}

func (s *FzfTUISelector) confirmEdit() {
	s.results = []SelectionResult{
		{
			Directory:   s.editDirectory,
			Command:     s.editCommand,
			DisplayName: s.editDisplayName,
			Action:      "edit",
		},
	}
	s.app.Stop()
}

func (s *FzfTUISelector) cancelEdit() {
	s.editing = false
	s.editCommand = ""
	s.editDirectory = ""
	s.editDisplayName = ""

	if s.editInput != nil {
		s.showNormalLayout()
		s.app.SetFocus(s.searchInput)
	}
}

func (s *FzfTUISelector) showEditLayout() {
	s.rootLayout.Clear()
	s.rootLayout.
		AddItem(s.editInput, 1, 0, true).
		AddItem(s.statusText, 1, 0, false).
		AddItem(s.mainPanel, 0, 1, false).
		AddItem(s.helpText, 1, 0, false)
	s.helpText.SetText("  [::d]Enter[-]: confirm  [::d]Esc[-]: cancel")
}

func (s *FzfTUISelector) showNormalLayout() {
	s.rootLayout.Clear()
	s.rootLayout.
		AddItem(s.searchInput, 1, 0, false).
		AddItem(s.statusText, 1, 0, false).
		AddItem(s.mainPanel, 0, 1, true).
		AddItem(s.helpText, 1, 0, false)
	s.helpText.SetText("  [::d]Tab[-]: select  [::d]Enter[-]: run  [::d]Ctrl+E[-]: edit  [::d]Ctrl+A[-]: all  [::d]Esc[-]: cancel")
}

func (s *FzfTUISelector) moveCursorDown() {
	if len(s.filteredCommands) == 0 {
		return
	}
	s.currentIndex++
	if s.currentIndex >= len(s.filteredCommands) {
		s.currentIndex = len(s.filteredCommands) - 1
	}
	if s.commandList != nil {
		s.commandList.SetCurrentItem(s.currentIndex)
	}
	s.updatePreview()
}

func (s *FzfTUISelector) moveCursorUp() {
	if len(s.filteredCommands) == 0 {
		return
	}
	s.currentIndex--
	if s.currentIndex < 0 {
		s.currentIndex = 0
	}
	if s.commandList != nil {
		s.commandList.SetCurrentItem(s.currentIndex)
	}
	s.updatePreview()
}

func (s *FzfTUISelector) updatePreview() {
	if s.previewPanel == nil {
		return // Skip UI update in tests
	}
	if s.currentIndex < 0 || s.currentIndex >= len(s.filteredCommands) {
		s.previewPanel.SetText("")
		return
	}

	cmd := s.filteredCommands[s.currentIndex]
	preview := s.generatePreview(cmd)
	s.previewPanel.SetText(preview)
}

func (s *FzfTUISelector) generatePreview(cmd CommandInfo) string {
	var lines []string

	lines = append(lines, fmt.Sprintf("[yellow]Location:[white]  %s", cmd.DisplayName))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("[yellow]Path:[white]      %s", cmd.Directory))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("[yellow]Command:[white]   %s", cmd.Command))

	if cmd.Type != "" {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("[yellow]Type:[white]      %s", cmd.Type))
	}

	if s.frecencyEnabled && cmd.FrecencyScore > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("[yellow]Score:[white]     %.2f", cmd.FrecencyScore))
	}

	return strings.Join(lines, "\n")
}

func (s *FzfTUISelector) updateStatus() {
	if s.statusText == nil {
		return // Skip UI update in tests
	}
	var parts []string

	// Count
	parts = append(parts, fmt.Sprintf("[::d]%d/%d[-]", len(s.filteredCommands), len(s.commands)))

	// Selected count
	selectedCount := s.getSelectedCount()
	if selectedCount > 0 {
		parts = append(parts, fmt.Sprintf("[green]%d selected[-]", selectedCount))
	}

	// Search query
	if s.searchQuery != "" {
		queryText := s.searchQuery
		if len(queryText) > 20 {
			queryText = queryText[:17] + "..."
		}
		parts = append(parts, fmt.Sprintf("[yellow]'%s'[-]", queryText))
	}

	// Frecency indicator
	if s.frecencyEnabled {
		parts = append(parts, "[blue]frecency[-]")
	}

	s.statusText.SetText("  " + strings.Join(parts, "  "))
}
