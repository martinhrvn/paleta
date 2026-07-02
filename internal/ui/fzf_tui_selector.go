package ui

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/history"
)

// Muted "Slate" color palette
var (
	searchPromptStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("67"))
	selectedMarkStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	cursorLineStyle    = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))
	previewBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("238"))
	previewLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	previewValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	statusStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	statusGreenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	statusYellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("180"))
	statusBlueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("67"))
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	helpKeyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("67"))
	editPromptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("180"))
	listLocationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	listCommandStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	previewTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("67")).Bold(true)
)

// Model is the bubbletea model for the fzf-style TUI selector
type Model struct {
	config           *config.Config
	commands         []CommandInfo
	filteredCommands []CommandInfo
	queue            []CommandInfo // ordered queue of commands to run, in enqueue order
	currentIndex     int
	results          []SelectionResult
	history          *history.History
	frecencyEnabled  bool

	// Focus
	focus       *FocusStore // nil when focus persistence is unavailable
	focusActive bool        // session toggle: show only focused locations

	// Focus picker mode (Ctrl+P)
	focusPicking bool
	focusItems   []FocusEntry
	focusCursor  int

	// Queue editor mode (Ctrl+Q): reorder/remove/save the queued commands.
	queueEditing bool
	queueCursor  int
	// Save-to-.pltrc sub-mode within the queue editor.
	queueSaving bool
	saveInput   textinput.Model
	queueHint   string     // transient message shown in the editor (e.g. save constraints)
	saveCommand *SaveStore // nil when saving to .pltrc is unavailable

	// reinit is set when the user requests adding projects (Ctrl+N). The
	// selector quits and the caller runs the init wizard before re-entering.
	reinit bool

	// Edit mode
	editing         bool
	editCommand     string
	editDirectory   string
	editDisplayName string
	editEnv         map[string]string

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

// NewModel creates a new bubbletea Model for the TUI selector. focus may be nil
// when there is no writable local config to persist focus changes to.
func NewModel(cfg *config.Config, focus *FocusStore) Model {
	si := textinput.New()
	si.Prompt = "> "
	si.Focus()
	si.PromptStyle = searchPromptStyle

	ei := textinput.New()
	ei.Prompt = "Edit> "
	ei.PromptStyle = editPromptStyle

	sv := textinput.New()
	sv.Prompt = "Save as> "
	sv.PromptStyle = editPromptStyle

	m := Model{
		config:          cfg,
		frecencyEnabled: cfg.Frecency.Enabled,
		focus:           focus,
		focusActive:     cfg.AnyFocused(),
		searchInput:     si,
		editInput:       ei,
		saveInput:       sv,
	}

	// Load history regardless of frecency: frecencyEnabled only controls sorting,
	// but the preview pane shows run/recency stats in either mode.
	if projectRoot, err := history.FindProjectRoot("."); err == nil {
		m.history, _ = history.LoadOrCreateHistory(projectRoot)
		if m.history != nil {
			m.history.SetWeights(history.NewWeights(cfg.Frecency.FrequencyWeight, cfg.Frecency.RecencyWeight))
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
		if m.queueEditing {
			return m.updateQueueEditMode(msg)
		}
		if m.focusPicking {
			return m.updateFocusPickMode(msg)
		}
		if m.editing {
			return m.updateEditMode(msg)
		}
		return m.updateNormalMode(msg)
	}

	// Pass other messages to the active input
	var cmd tea.Cmd
	if m.focusPicking {
		return m, nil
	}
	if m.queueEditing {
		if m.queueSaving {
			m.saveInput, cmd = m.saveInput.Update(msg)
		}
		return m, cmd
	}
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

	case tea.KeyCtrlT:
		m.focusActive = !m.focusActive
		m.loadCommands()
		m.updateFilteredCommands()
		return m, nil

	case tea.KeyCtrlP:
		m.enterFocusPicker()
		return m, nil

	case tea.KeyCtrlQ:
		m.enterQueueEditor()
		return m, nil

	case tea.KeyCtrlN:
		m.reinit = true
		m.quitting = true
		return m, tea.Quit

	case tea.KeyEscape:
		// Esc clears a non-empty search first; on an empty search it quits.
		if m.searchInput.Value() != "" {
			m.searchInput.SetValue("")
			m.updateFilteredCommands()
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case tea.KeyCtrlC:
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

	if m.queueEditing {
		return m.renderQueueEditor()
	}

	if m.focusPicking {
		return m.renderFocusPicker()
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
		sections = append(sections, m.renderHelp([][2]string{
			{"Enter", "confirm"},
			{"Esc", "cancel"},
		}))
	} else {
		sections = append(sections, m.renderHelp([][2]string{
			{"Tab", "queue"},
			{"^Q", "edit queue"},
			{"Enter", "run"},
			{"^E", "edit"},
			{"^F", "frecency"},
			{"^T", "focus"},
			{"^P", "pick"},
			{"^N", "add"},
			{"Esc", "cancel"},
		}))
	}

	return strings.Join(sections, "\n")
}

func (m Model) renderStatus() string {
	var parts []string

	parts = append(parts, statusStyle.Render(fmt.Sprintf("%d/%d", len(m.filteredCommands), len(m.commands))))

	queuedCount := m.getSelectedCount()
	if queuedCount > 0 {
		parts = append(parts, statusGreenStyle.Render(fmt.Sprintf("%d queued", queuedCount)))
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

	if m.focusActive && m.config.AnyFocused() {
		parts = append(parts, statusGreenStyle.Render("focused"))
	}

	return "  " + strings.Join(parts, statusStyle.Render(" · "))
}

func (m Model) renderHelp(items [][2]string) string {
	var parts []string
	for _, item := range items {
		parts = append(parts, helpKeyStyle.Render(item[0])+helpStyle.Render(" "+item[1]))
	}
	return "  " + strings.Join(parts, helpStyle.Render(" · "))
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
		pos := m.queuePosAt(i)
		isCursor := i == m.currentIndex

		var line string
		if isCursor {
			// Use plain text for cursor line — cursorLineStyle overrides sub-colors
			plain := m.formatListItemPlain(i, pos)
			if len(plain) < width {
				plain = plain + strings.Repeat(" ", width-len(plain))
			}
			line = cursorLineStyle.Render(plain)
		} else {
			line = m.formatListItem(i, pos)
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

	// When the focus filter is active and any location is focused, hide the
	// non-focused ones. The AnyFocused guard means toggling focus on while
	// nothing is focused never blanks the list.
	filterFocus := m.focusActive && m.config.AnyFocused()

	for _, location := range m.config.Locations {
		if filterFocus && !location.Focused {
			continue
		}

		displayName := location.Name
		if displayName == "" {
			displayName = location.Location
		}

		for _, command := range location.Commands {
			cmdDisplay := config.CommandLabel(location, command)

			var score float64
			if m.history != nil && m.frecencyEnabled {
				score = m.history.GetScore(displayName, command.Command)
			}

			info := CommandInfo{
				Display:       fmt.Sprintf("%s: %s", displayName, cmdDisplay),
				Directory:     location.Location,
				Command:       command.Command,
				DisplayName:   displayName,
				Type:          command.Type,
				Env:           config.EffectiveEnv(location, command),
				FrecencyScore: score,
			}
			m.commands = append(m.commands, info)
		}
	}
}

// reloadConfig re-reads the discovered config from disk so focus changes written
// by the picker are reflected in the list. On failure the existing config is
// kept.
func (m *Model) reloadConfig() {
	if cfg, err := config.LoadConfigFromDiscovery(); err == nil {
		m.config = cfg
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

// queueBadgePlain renders the 2-column list prefix for a given queue position:
// two spaces when not queued, otherwise the 1-based position left-padded to two
// columns (e.g. "1 ", "10").
func queueBadgePlain(pos int) string {
	if pos <= 0 {
		return "  "
	}
	return fmt.Sprintf("%-2d", pos)
}

func (m Model) formatListItem(index, queuePos int) string {
	if index < 0 || index >= len(m.filteredCommands) {
		return ""
	}
	prefix := "  "
	if queuePos > 0 {
		prefix = selectedMarkStyle.Render(queueBadgePlain(queuePos))
	}
	// Split Display on first ": " to style location and command differently
	display := m.filteredCommands[index].Display
	if loc, rest, ok := strings.Cut(display, ": "); ok {
		return prefix + listLocationStyle.Render(loc+":") + " " + listCommandStyle.Render(rest)
	}
	return prefix + listCommandStyle.Render(display)
}

// formatListItemPlain returns an unstyled version for cursor line rendering,
// where cursorLineStyle overrides sub-colors anyway.
func (m Model) formatListItemPlain(index, queuePos int) string {
	if index < 0 || index >= len(m.filteredCommands) {
		return ""
	}
	return queueBadgePlain(queuePos) + m.filteredCommands[index].Display
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
	return fuzzySubsequence(text, query)
}

// fuzzySubsequence reports whether every character of query appears in text in
// order (a subsequence match). Both are expected to already be lowercased by the
// caller. Shared by the command palette and the init wizard.
func fuzzySubsequence(text, query string) bool {
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

// queueKey identifies a command by its execution target (directory + command),
// so the queue survives re-filtering where filtered indices shift.
func queueKey(c CommandInfo) string {
	return c.Directory + "\x00" + c.Command
}

// queuePos returns the 1-based position of a command in the queue, and whether
// it is queued at all.
func (m Model) queuePos(c CommandInfo) (int, bool) {
	key := queueKey(c)
	for i, q := range m.queue {
		if queueKey(q) == key {
			return i + 1, true
		}
	}
	return 0, false
}

// queuePosAt returns the 1-based queue position for the command at a filtered
// index (0 when the index is invalid or the command is not queued).
func (m Model) queuePosAt(index int) int {
	if index < 0 || index >= len(m.filteredCommands) {
		return 0
	}
	pos, _ := m.queuePos(m.filteredCommands[index])
	return pos
}

// toggleSelection enqueues the command at a filtered index, or removes it from
// the queue when already present. Enqueue order is preserved.
func (m *Model) toggleSelection(index int) {
	if index < 0 || index >= len(m.filteredCommands) {
		return
	}
	m.enqueueToggle(m.filteredCommands[index])
}

func (m *Model) enqueueToggle(c CommandInfo) {
	key := queueKey(c)
	for i, q := range m.queue {
		if queueKey(q) == key {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			return
		}
	}
	m.queue = append(m.queue, c)
}

// toggleSelectAll appends every not-yet-queued filtered command in list order,
// or clears the queue when everything visible is already queued.
func (m *Model) toggleSelectAll() {
	allQueued := true
	for i := range m.filteredCommands {
		if _, ok := m.queuePos(m.filteredCommands[i]); !ok {
			allQueued = false
			break
		}
	}
	if allQueued {
		m.clearSelections()
		return
	}
	for i := range m.filteredCommands {
		if _, ok := m.queuePos(m.filteredCommands[i]); !ok {
			m.queue = append(m.queue, m.filteredCommands[i])
		}
	}
}

func (m *Model) clearSelections() {
	m.queue = nil
}

func (m Model) getSelectedCount() int {
	return len(m.queue)
}

func (m Model) getSelectedCommands() []SelectionResult {
	var results []SelectionResult

	// Emit in queue (enqueue) order, so execution order is deterministic and
	// independent of the list's display order.
	for _, cmd := range m.queue {
		results = append(results, SelectionResult{
			Directory:   cmd.Directory,
			Command:     cmd.Command,
			DisplayName: cmd.DisplayName,
			Env:         cmd.Env,
		})
	}

	// If nothing queued, return the current item under the cursor.
	if len(results) == 0 && m.currentIndex >= 0 && m.currentIndex < len(m.filteredCommands) {
		cmd := m.filteredCommands[m.currentIndex]
		results = append(results, SelectionResult{
			Directory:   cmd.Directory,
			Command:     cmd.Command,
			DisplayName: cmd.DisplayName,
			Env:         cmd.Env,
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
	m.editEnv = cmd.Env

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
			Env:         m.editEnv,
		},
	}
}

func (m *Model) cancelEdit() {
	m.editing = false
	m.editCommand = ""
	m.editDirectory = ""
	m.editDisplayName = ""
	m.editEnv = nil

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

// sortedKeys returns the map keys in deterministic alphabetical order.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (m Model) generatePreview(cmd CommandInfo) string {
	var lines []string

	lines = append(lines, previewTitleStyle.Render("Details"))
	lines = append(lines, statusStyle.Render("─────────────────────"))
	lines = append(lines, previewLabelStyle.Render("Location  ")+previewValueStyle.Render(cmd.DisplayName))
	lines = append(lines, previewLabelStyle.Render("Path      ")+previewValueStyle.Render(cmd.Directory))
	lines = append(lines, previewLabelStyle.Render("Command   ")+previewValueStyle.Render(cmd.Command))

	if cmd.Type != "" {
		lines = append(lines, previewLabelStyle.Render("Type      ")+previewValueStyle.Render(cmd.Type))
	}

	if len(cmd.Env) > 0 {
		lines = append(lines, previewLabelStyle.Render("Env"))
		for _, k := range sortedKeys(cmd.Env) {
			lines = append(lines, previewValueStyle.Render(fmt.Sprintf("  %s=%s", k, cmd.Env[k])))
		}
	}

	// History / recency stats, shown whenever this command has been run before.
	if m.history != nil {
		if entry, ok := m.history.GetEntry(cmd.DisplayName, cmd.Command); ok {
			now := time.Now()
			lines = append(lines, previewLabelStyle.Render("Runs      ")+previewValueStyle.Render(strconv.Itoa(entry.Count)))
			lines = append(lines, previewLabelStyle.Render("Last used ")+previewValueStyle.Render(history.FormatSince(now.Sub(entry.LastAccess))))
			lines = append(lines, previewLabelStyle.Render("First run ")+previewValueStyle.Render(history.FormatSince(now.Sub(entry.FirstAccess))))
			score := m.history.GetScore(cmd.DisplayName, cmd.Command)
			lines = append(lines, previewLabelStyle.Render("Score     ")+previewValueStyle.Render(fmt.Sprintf("%.2f", score)))
		}
	}

	return strings.Join(lines, "\n")
}

// Run starts the TUI and returns selected commands. The returned bool is true
// when the user requested adding projects (Ctrl+N); the caller should run the
// init wizard and re-enter the selector.
func (m *Model) Run() ([]SelectionResult, bool, error) {
	m.loadCommands()
	m.updateFilteredCommands()

	// Open /dev/tty with O_RDWR for TUI rendering so stdout stays clean for
	// JSON output. Read access is needed for terminal capability queries.
	// The shell integration captures stdout, so bubbletea must not write there.
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, false, fmt.Errorf("failed to open /dev/tty: %w", err)
	}
	defer tty.Close()

	lipgloss.SetColorProfile(termenv.TrueColor)

	// Drive both input and output through /dev/tty so os.Stdin/os.Stdout are
	// left untouched. This keeps stdout clean for the JSON result and lets the
	// selector hand off cleanly to a second program (the in-app init wizard)
	// without leaving os.Stdin in a half-consumed state.
	p := tea.NewProgram(*m, tea.WithAltScreen(), tea.WithInput(tty), tea.WithOutput(tty))
	finalModel, err := p.Run()
	if err != nil {
		return nil, false, err
	}

	fm := finalModel.(Model)
	if fm.reinit {
		return nil, true, nil
	}
	if len(fm.results) == 0 {
		return nil, false, fmt.Errorf("selection canceled")
	}

	return fm.results, false, nil
}
