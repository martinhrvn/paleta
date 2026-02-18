package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/martin/go-pm/internal/config"
	"github.com/martin/go-pm/internal/history"
)

// Styles
var (
	searchPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	selectedMarkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	cursorLineStyle   = lipgloss.NewStyle().Background(lipgloss.Color("12")).Foreground(lipgloss.Color("0"))
	previewBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))
	previewLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	previewValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	statusStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusGreenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	statusYellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	statusBlueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	editPromptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

// Model is the bubbletea model for the fzf-style TUI selector
type Model struct {
	config           *config.Config
	commands         []CommandInfo
	filteredCommands []CommandInfo
	selectedIndices  map[int]bool
	currentIndex     int
	results          []SelectionResult
	history          *history.History
	frecencyEnabled  bool

	// Edit mode
	editing         bool
	editCommand     string
	editDirectory   string
	editDisplayName string

	// Bubbletea components
	searchInput textinput.Model
	editInput   textinput.Model

	// Terminal dimensions
	width  int
	height int

	// Viewport scrolling
	viewportOffset int

	// State
	quitting bool
}

// NewModel creates a new bubbletea Model for the TUI selector
func NewModel(cfg *config.Config) Model {
	si := textinput.New()
	si.Prompt = "> "
	si.Focus()
	si.PromptStyle = searchPromptStyle

	ei := textinput.New()
	ei.Prompt = "Edit> "
	ei.PromptStyle = editPromptStyle

	m := Model{
		config:          cfg,
		selectedIndices: make(map[int]bool),
		frecencyEnabled: cfg.Frecency.Enabled,
		searchInput:     si,
		editInput:       ei,
	}

	// Load history if frecency is enabled
	if m.frecencyEnabled {
		projectRoot, err := history.FindProjectRoot(".")
		if err == nil {
			m.history, _ = history.LoadOrCreateHistory(projectRoot)
		}
	}

	return m
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.editing {
			return m.updateEditMode(msg)
		}
		return m.updateNormalMode(msg)
	}

	// Pass other messages to the active input
	var cmd tea.Cmd
	if m.editing {
		m.editInput, cmd = m.editInput.Update(msg)
	} else {
		prevValue := m.searchInput.Value()
		m.searchInput, cmd = m.searchInput.Update(msg)
		if m.searchInput.Value() != prevValue {
			m.updateFilteredCommands()
		}
	}
	return m, cmd
}

func (m Model) updateNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyDown, tea.KeyCtrlJ:
		m.moveCursorDown()
		return m, nil

	case tea.KeyUp, tea.KeyCtrlK:
		m.moveCursorUp()
		return m, nil

	case tea.KeyEnter:
		m.confirmSelection()
		m.quitting = true
		return m, tea.Quit

	case tea.KeyCtrlE:
		m.enterEditMode()
		return m, nil

	case tea.KeyTab:
		m.toggleSelection(m.currentIndex)
		m.moveCursorDown()
		return m, nil

	case tea.KeyCtrlA:
		m.toggleSelectAll()
		return m, nil

	case tea.KeyCtrlL, tea.KeyCtrlU:
		m.searchInput.SetValue("")
		m.updateFilteredCommands()
		return m, nil

	case tea.KeyCtrlF:
		m.frecencyEnabled = !m.frecencyEnabled
		m.updateFilteredCommands()
		return m, nil

	case tea.KeyEscape, tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	}

	// Pass key to search input for text entry
	prevValue := m.searchInput.Value()
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	if m.searchInput.Value() != prevValue {
		m.updateFilteredCommands()
	}
	return m, cmd
}

func (m Model) updateEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.editCommand = m.editInput.Value()
		m.confirmEdit()
		m.quitting = true
		return m, tea.Quit

	case tea.KeyEscape:
		m.cancelEdit()
		return m, nil
	}

	// Pass to edit input
	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

// View implements tea.Model
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var sections []string

	// Search / edit input
	if m.editing {
		sections = append(sections, m.editInput.View())
	} else {
		sections = append(sections, m.searchInput.View())
	}

	// Status line
	sections = append(sections, m.renderStatus())

	// Main content: command list + preview panel
	sections = append(sections, m.renderMainContent())

	// Help line
	if m.editing {
		sections = append(sections, helpStyle.Render("  Enter: confirm  Esc: cancel"))
	} else {
		sections = append(sections, helpStyle.Render("  Tab: select  Enter: run  Ctrl+E: edit  Ctrl+A: all  Esc: cancel"))
	}

	return strings.Join(sections, "\n")
}

func (m Model) renderStatus() string {
	var parts []string

	parts = append(parts, statusStyle.Render(fmt.Sprintf("%d/%d", len(m.filteredCommands), len(m.commands))))

	selectedCount := m.getSelectedCount()
	if selectedCount > 0 {
		parts = append(parts, statusGreenStyle.Render(fmt.Sprintf("%d selected", selectedCount)))
	}

	if m.searchInput.Value() != "" {
		queryText := m.searchInput.Value()
		if len(queryText) > 20 {
			queryText = queryText[:17] + "..."
		}
		parts = append(parts, statusYellowStyle.Render(fmt.Sprintf("'%s'", queryText)))
	}

	if m.frecencyEnabled {
		parts = append(parts, statusBlueStyle.Render("frecency"))
	}

	return "  " + strings.Join(parts, "  ")
}

func (m Model) renderMainContent() string {
	// Calculate available height for the list (total height minus chrome: search + status + help = 3 lines)
	listHeight := m.height - 3
	if listHeight < 1 {
		listHeight = 10 // sensible default
	}

	// Calculate widths for list and preview
	listWidth := m.width * 7 / 10
	previewWidth := m.width - listWidth - 1 // -1 for spacing
	if listWidth < 20 {
		listWidth = 20
	}
	if previewWidth < 15 {
		previewWidth = 15
	}

	// Render command list
	listView := m.renderCommandList(listWidth, listHeight)

	// Render preview panel
	previewView := m.renderPreview(previewWidth, listHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, " ", previewView)
}

func (m Model) renderCommandList(width, height int) string {
	if len(m.filteredCommands) == 0 {
		return lipgloss.NewStyle().Width(width).Height(height).Render("  No matches")
	}

	// Calculate visible range
	visibleCount := height
	start := m.viewportOffset
	end := start + visibleCount
	if end > len(m.filteredCommands) {
		end = len(m.filteredCommands)
	}

	var lines []string
	for i := start; i < end; i++ {
		isSelected := m.selectedIndices[i]
		isCursor := i == m.currentIndex

		line := m.formatListItem(i, isSelected)

		if isCursor {
			// Pad to full width for cursor highlight
			padded := line
			if len(padded) < width {
				padded = padded + strings.Repeat(" ", width-len(padded))
			}
			line = cursorLineStyle.Render(padded)
		} else if isSelected {
			line = selectedMarkStyle.Render(line)
		}

		lines = append(lines, line)
	}

	// Pad remaining lines if list is shorter than available height
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderPreview(width, height int) string {
	var content string
	if m.currentIndex >= 0 && m.currentIndex < len(m.filteredCommands) {
		content = m.generatePreview(m.filteredCommands[m.currentIndex])
	}

	style := previewBorderStyle.
		Width(width - 2). // account for border
		Height(height - 2)

	return style.Render(content)
}

func (m *Model) loadCommands() {
	m.commands = []CommandInfo{}

	for _, location := range m.config.Locations {
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
			if m.history != nil && m.frecencyEnabled {
				score = m.history.GetScore(displayName, command.Command)
			}

			info := CommandInfo{
				Display:       fmt.Sprintf("%s: %s", displayName, cmdDisplay),
				Directory:     location.Location,
				Command:       command.Command,
				DisplayName:   displayName,
				Type:          location.Type,
				FrecencyScore: score,
			}
			m.commands = append(m.commands, info)
		}
	}
}

func (m *Model) updateFilteredCommands() {
	// Recalculate frecency scores if enabled
	if m.frecencyEnabled && m.history != nil {
		for i := range m.commands {
			m.commands[i].FrecencyScore = m.history.GetScore(
				m.commands[i].DisplayName,
				m.commands[i].Command,
			)
		}
	}

	// Clear selections when filter changes
	m.clearSelections()

	// Apply fuzzy filter
	query := m.searchInput.Value()
	if query == "" {
		m.filteredCommands = make([]CommandInfo, len(m.commands))
		copy(m.filteredCommands, m.commands)
	} else {
		m.filteredCommands = m.fuzzyFilter(m.commands, query)
	}

	// Sort by frecency if enabled
	if m.frecencyEnabled && m.history != nil {
		sort.Slice(m.filteredCommands, func(i, j int) bool {
			return m.filteredCommands[i].FrecencyScore > m.filteredCommands[j].FrecencyScore
		})
	}

	// Reset cursor and viewport
	m.currentIndex = 0
	m.viewportOffset = 0
}

func (m Model) formatListItem(index int, selected bool) string {
	if index < 0 || index >= len(m.filteredCommands) {
		return ""
	}
	prefix := "  "
	if selected {
		prefix = "* "
	}
	return prefix + m.filteredCommands[index].Display
}

func (m Model) fuzzyFilter(commands []CommandInfo, query string) []CommandInfo {
	if query == "" {
		return commands
	}

	query = strings.ToLower(query)
	var filtered []CommandInfo

	for _, cmd := range commands {
		if m.fuzzyMatch(strings.ToLower(cmd.Display), query) {
			filtered = append(filtered, cmd)
		}
	}

	return filtered
}

func (m Model) fuzzyMatch(text, query string) bool {
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

func (m *Model) toggleSelection(index int) {
	if index < 0 || index >= len(m.filteredCommands) {
		return
	}
	m.selectedIndices[index] = !m.selectedIndices[index]
	if !m.selectedIndices[index] {
		delete(m.selectedIndices, index)
	}
}

func (m *Model) toggleSelectAll() {
	if m.getSelectedCount() == len(m.filteredCommands) {
		m.clearSelections()
	} else {
		for i := range m.filteredCommands {
			m.selectedIndices[i] = true
		}
	}
}

func (m *Model) clearSelections() {
	m.selectedIndices = make(map[int]bool)
}

func (m Model) getSelectedCount() int {
	count := 0
	for _, selected := range m.selectedIndices {
		if selected {
			count++
		}
	}
	return count
}

func (m Model) getSelectedCommands() []SelectionResult {
	var results []SelectionResult

	for i := 0; i < len(m.filteredCommands); i++ {
		if m.selectedIndices[i] {
			cmd := m.filteredCommands[i]
			results = append(results, SelectionResult{
				Directory:   cmd.Directory,
				Command:     cmd.Command,
				DisplayName: cmd.DisplayName,
			})
		}
	}

	// If nothing explicitly selected, return current item
	if len(results) == 0 && m.currentIndex >= 0 && m.currentIndex < len(m.filteredCommands) {
		cmd := m.filteredCommands[m.currentIndex]
		results = append(results, SelectionResult{
			Directory:   cmd.Directory,
			Command:     cmd.Command,
			DisplayName: cmd.DisplayName,
		})
	}

	return results
}

func (m *Model) confirmSelection() {
	m.results = m.getSelectedCommands()
}

func (m *Model) enterEditMode() {
	if len(m.filteredCommands) == 0 || m.currentIndex < 0 || m.currentIndex >= len(m.filteredCommands) {
		return
	}

	cmd := m.filteredCommands[m.currentIndex]
	m.editing = true
	m.editCommand = cmd.Command
	m.editDirectory = cmd.Directory
	m.editDisplayName = cmd.DisplayName

	m.editInput.SetValue(cmd.Command)
	m.editInput.Focus()
	m.searchInput.Blur()
}

func (m *Model) confirmEdit() {
	m.results = []SelectionResult{
		{
			Directory:   m.editDirectory,
			Command:     m.editCommand,
			DisplayName: m.editDisplayName,
			Action:      "edit",
		},
	}
}

func (m *Model) cancelEdit() {
	m.editing = false
	m.editCommand = ""
	m.editDirectory = ""
	m.editDisplayName = ""

	m.editInput.Blur()
	m.searchInput.Focus()
}

func (m *Model) moveCursorDown() {
	if len(m.filteredCommands) == 0 {
		return
	}
	m.currentIndex++
	if m.currentIndex >= len(m.filteredCommands) {
		m.currentIndex = len(m.filteredCommands) - 1
	}
	m.adjustViewport()
}

func (m *Model) moveCursorUp() {
	if len(m.filteredCommands) == 0 {
		return
	}
	m.currentIndex--
	if m.currentIndex < 0 {
		m.currentIndex = 0
	}
	m.adjustViewport()
}

func (m *Model) adjustViewport() {
	visibleRows := m.height - 3
	if visibleRows < 1 {
		visibleRows = 10
	}

	// Scroll down if cursor is below viewport
	if m.currentIndex >= m.viewportOffset+visibleRows {
		m.viewportOffset = m.currentIndex - visibleRows + 1
	}
	// Scroll up if cursor is above viewport
	if m.currentIndex < m.viewportOffset {
		m.viewportOffset = m.currentIndex
	}
}

func (m Model) generatePreview(cmd CommandInfo) string {
	var lines []string

	lines = append(lines, previewLabelStyle.Render("Location:")+"  "+previewValueStyle.Render(cmd.DisplayName))
	lines = append(lines, "")
	lines = append(lines, previewLabelStyle.Render("Path:")+"      "+previewValueStyle.Render(cmd.Directory))
	lines = append(lines, "")
	lines = append(lines, previewLabelStyle.Render("Command:")+"   "+previewValueStyle.Render(cmd.Command))

	if cmd.Type != "" {
		lines = append(lines, "")
		lines = append(lines, previewLabelStyle.Render("Type:")+"      "+previewValueStyle.Render(cmd.Type))
	}

	if m.frecencyEnabled && cmd.FrecencyScore > 0 {
		lines = append(lines, "")
		lines = append(lines, previewLabelStyle.Render("Score:")+"     "+previewValueStyle.Render(fmt.Sprintf("%.2f", cmd.FrecencyScore)))
	}

	return strings.Join(lines, "\n")
}

// Run starts the TUI and returns selected commands
func (m *Model) Run() ([]SelectionResult, error) {
	m.loadCommands()
	m.updateFilteredCommands()

	// Open /dev/tty for TUI rendering so stdout stays clean for JSON output.
	// The shell integration captures stdout, so bubbletea must not write there.
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/tty: %w", err)
	}
	defer tty.Close()

	p := tea.NewProgram(*m, tea.WithAltScreen(), tea.WithOutput(tty))
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	fm := finalModel.(Model)
	if len(fm.results) == 0 {
		return nil, fmt.Errorf("selection canceled")
	}

	return fm.results, nil
}
