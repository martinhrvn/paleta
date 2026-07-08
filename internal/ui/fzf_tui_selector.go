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
	"github.com/martinhrvn/paleta/internal/mux"
)

// Catppuccin Mocha palette (truecolor). Forced on via
// lipgloss.SetColorProfile(termenv.TrueColor) in Run(), so these hex colors
// render even though the shell wrapper captures stdout as a pipe.
const (
	ccBase     = "#1e1e2e"
	ccSurface0 = "#313244"
	ccOverlay0 = "#6c7086"
	ccText     = "#cdd6f4"
	ccLavender = "#b4befe"
	ccBlue     = "#89b4fa"
	ccGreen    = "#a6e3a1"
	ccYellow   = "#f9e2af"
	ccPeach    = "#fab387"
)

var (
	searchPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccLavender)).Bold(true)
	selectedMarkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccGreen)).Bold(true)
	// cursorLineStyle is the selected-row highlight shared by the focus picker,
	// queue editor, and init wizard: a plain surface fill (no inner styling, so
	// the background never gets punched out by ANSI resets).
	cursorLineStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(ccSurface0)).
			Foreground(lipgloss.Color(ccText)).
			Bold(true)
	// Selected-row segments for the main palette list, which supports the
	// lavender accent bar and per-character fuzzy-match highlighting. Every
	// segment carries the surface background so no gaps appear between them.
	selBarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(ccLavender)).Background(lipgloss.Color(ccBase))
	selBaseStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(ccText)).Background(lipgloss.Color(ccSurface0)).Bold(true)
	selHlStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(ccLavender)).Background(lipgloss.Color(ccSurface0)).Bold(true)
	selBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccGreen)).Background(lipgloss.Color(ccSurface0)).Bold(true)
	// Checked (queued) rows that are not under the cursor: a subtle surface fill
	// with lavender accent text and a green position badge, so checked commands
	// stand out from the list without competing with the cursor row's accent bar.
	// Matches highlight in bright text so they still pop against the lavender base.
	queuedBaseStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(ccLavender)).Background(lipgloss.Color(ccSurface0))
	queuedHlStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(ccText)).Background(lipgloss.Color(ccSurface0)).Bold(true)
	queuedBadgeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(ccGreen)).Background(lipgloss.Color(ccSurface0)).Bold(true)
	previewBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(ccOverlay0))
	previewLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccBlue))
	previewValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccText))
	statusStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color(ccOverlay0)).Faint(true)
	statusGreenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(ccGreen)).Bold(true)
	statusYellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccYellow)).Bold(true)
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color(ccOverlay0)).Faint(true)
	helpKeyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(ccLavender)).Bold(true)
	editPromptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(ccPeach)).Bold(true)
	listLocationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccOverlay0)).Faint(true)
	listCommandStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(ccText))
	previewTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccLavender)).Bold(true)
	// matchStyle highlights fuzzy-matched characters in list rows.
	matchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ccLavender)).Bold(true)
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

	// mux is the terminal multiplexer plt is running inside (tmux/zellij), or
	// mux.None. When active, the selector offers Ctrl+O to run the selection in a
	// new multiplexer tab/window.
	mux mux.Multiplexer

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
	si.Prompt = searchPromptGlyph()
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
		mux:             mux.DetectEnv(),
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

	case tea.KeyCtrlO:
		// Run the selection in a new multiplexer tab/window. Only meaningful when
		// running inside tmux/zellij; otherwise swallow the key silently.
		if m.mux.Active() {
			m.confirmPaneSelection()
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

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

	// Warning banner for names that can't be used as aliases
	if banner := m.renderWarningBanner(); banner != "" {
		sections = append(sections, banner)
	}

	// Main content: command list + preview panel
	sections = append(sections, m.renderMainContent())

	// Help line
	if m.editing {
		sections = append(sections, m.renderHelp([][2]string{
			{"Enter", "confirm"},
			{"Esc", "cancel"},
		}))
	} else {
		parts := []string{
			helpItem("Tab", "queue"),
			helpItem("^Q", "edit queue"),
			helpItem("Enter", "run"),
		}
		if m.mux.Active() {
			parts = append(parts, helpItem("^O", m.mux.Label()))
		}
		parts = append(parts,
			helpItem("^E", "edit"),
			renderToggleHelp("^F", "frecency", m.frecencyEnabled),
		)
		if m.config.AnyFocused() {
			parts = append(parts, renderToggleHelp("^T", "focus", m.focusActive))
		}
		parts = append(parts,
			helpItem("^P", "pick"),
			helpItem("^N", "add"),
			helpItem("Esc", "cancel"),
		)
		sections = append(sections, "  "+strings.Join(parts, helpStyle.Render(" · ")))
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

	return "  " + strings.Join(parts, statusStyle.Render(" · "))
}

// renderWarningBanner renders a one-line warning when the config has issues
// (names that can't be used as aliases, or unresolved @project:command
// references). Returns "" when there are no warnings.
func (m Model) renderWarningBanner() string {
	n := len(m.config.Warnings)
	if n == 0 {
		return ""
	}
	noun := "issue"
	if n > 1 {
		noun = "issues"
	}
	return "  " + statusYellowStyle.Render(fmt.Sprintf("⚠ %d config %s — run 'plt lint' for details", n, noun))
}

// renderToggleHelp renders a keyboard-shortcut toggle for the help line, e.g.
// "^F frecency ON" with the state highlighted when on and muted when off.
func renderToggleHelp(key, label string, on bool) string {
	state := helpStyle.Render("OFF")
	if on {
		state = statusGreenStyle.Render("ON")
	}
	return helpKeyStyle.Render(key) + helpStyle.Render(" "+label+" ") + state
}

// helpItem renders a single "key label" hint for the help line.
func helpItem(key, label string) string {
	return helpKeyStyle.Render(key) + helpStyle.Render(" "+label)
}

func (m Model) renderHelp(items [][2]string) string {
	var parts []string
	for _, item := range items {
		parts = append(parts, helpItem(item[0], item[1]))
	}
	return "  " + strings.Join(parts, helpStyle.Render(" · "))
}

func (m Model) renderMainContent() string {
	// Calculate available height for the list (total height minus chrome: search +
	// status + help = 3 lines, plus the warning banner line when shown).
	chrome := 3
	if len(m.config.Warnings) > 0 {
		chrome++
	}
	listHeight := m.height - chrome
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

	query := strings.ToLower(m.searchInput.Value())

	var lines []string
	for i := start; i < end; i++ {
		pos := m.queuePosAt(i)
		matched := matchedSet(m.filteredCommands[i].Display, query)

		var line string
		switch {
		case i == m.currentIndex:
			line = m.renderCursorRow(i, pos, matched, width)
		case pos > 0:
			// Checked (queued) but not under the cursor: subtle background + accent.
			line = m.renderQueuedRow(i, pos, matched, width)
		default:
			// Leading space aligns non-selected rows with the selected row's
			// accent bar column.
			line = " " + m.formatListItem(i, pos, matched)
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
			// The row shows the bare name; the project type is rendered separately
			// as a trailing badge (see rowContent), so we don't fold it into the
			// label here — that also disambiguates same-named commands across types
			// the way CommandLabel's "[type]" prefix used to.
			cmdDisplay := command.Name
			if cmdDisplay == "" {
				cmdDisplay = command.Command
			}

			var score float64
			if m.history != nil && m.frecencyEnabled {
				score = m.history.GetScore(displayName, command.Command)
			}

			// Flag the row when its name is un-aliasable (space, '*', …) or its
			// @project:command reference couldn't be resolved. Prefer the command's
			// name reason, then its unresolved-alias error, then the location's.
			invalidReason := command.NameError
			if invalidReason == "" {
				invalidReason = command.Error
			}
			if invalidReason == "" {
				invalidReason = location.NameError
			}

			info := CommandInfo{
				Display:       fmt.Sprintf("%s: %s", displayName, cmdDisplay),
				Directory:     location.Location,
				Command:       command.Command,
				DisplayName:   displayName,
				Name:          command.Name,
				Type:          command.Type,
				Env:           config.EffectiveEnv(location, command),
				FrecencyScore: score,
				Invalid:       invalidReason != "",
				InvalidReason: invalidReason,
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
	// Recalculate frecency scores if enabled, then pre-sort the base list by
	// frecency. fuzzyFilter's sort is stable, so among equally good textual
	// matches the more frequently used command stays first — giving "match
	// quality first, frecency as tiebreak" without threading scores through.
	if m.frecencyEnabled && m.history != nil {
		for i := range m.commands {
			m.commands[i].FrecencyScore = m.history.GetScore(
				m.commands[i].DisplayName,
				m.commands[i].Command,
			)
		}
		sort.SliceStable(m.commands, func(i, j int) bool {
			return m.commands[i].FrecencyScore > m.commands[j].FrecencyScore
		})
	}

	// Apply fuzzy filter. With no query the pre-sorted base order is kept as-is;
	// with a query, results are ordered best-match-first by fuzzyFilter.
	query := m.searchInput.Value()
	if query == "" {
		m.filteredCommands = make([]CommandInfo, len(m.commands))
		copy(m.filteredCommands, m.commands)
	} else {
		m.filteredCommands = m.fuzzyFilter(m.commands, query)
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

func (m Model) formatListItem(index, queuePos int, matched map[int]bool) string {
	if index < 0 || index >= len(m.filteredCommands) {
		return ""
	}
	prefix := "  "
	if queuePos > 0 {
		prefix = selectedMarkStyle.Render(queueBadgePlain(queuePos))
	}
	display := m.filteredCommands[index].Display
	return prefix + rowContent(display, matched, listLocationStyle, listCommandStyle, matchStyle, m.filteredCommands[index].Invalid, m.filteredCommands[index].Type)
}

// renderCursorRow renders the selected list row: a lavender accent bar followed
// by a surface-filled line with fuzzy matches highlighted. Every inner segment
// carries the surface background so the fill has no gaps.
func (m Model) renderCursorRow(index, queuePos int, matched map[int]bool, width int) string {
	if index < 0 || index >= len(m.filteredCommands) {
		return ""
	}
	display := m.filteredCommands[index].Display
	typ := m.filteredCommands[index].Type
	badgePlain := queueBadgePlain(queuePos)
	badgeStyle := selBaseStyle
	if queuePos > 0 {
		badgeStyle = selBadgeStyle
	}
	content := badgeStyle.Render(badgePlain) + rowContent(display, matched, selBaseStyle, selBaseStyle, selHlStyle, m.filteredCommands[index].Invalid, typ)
	// Pad the surface fill to width-1; the accent bar occupies the first column.
	if pad := (width - 1) - lipgloss.Width(badgePlain+rowPlain(display, typ)); pad > 0 {
		content += selBaseStyle.Render(strings.Repeat(" ", pad))
	}
	return selBarStyle.Render("▌") + content
}

// renderQueuedRow renders a checked (queued) row that is not under the cursor:
// a subtle surface-filled line with a green position badge and lavender accent
// text, so checked commands read as selected without stealing the cursor row's
// accent bar. A leading space keeps it aligned with that bar's column.
func (m Model) renderQueuedRow(index, queuePos int, matched map[int]bool, width int) string {
	if index < 0 || index >= len(m.filteredCommands) {
		return ""
	}
	display := m.filteredCommands[index].Display
	typ := m.filteredCommands[index].Type
	badgePlain := queueBadgePlain(queuePos)
	content := queuedBadgeStyle.Render(badgePlain) + rowContent(display, matched, queuedBaseStyle, queuedBaseStyle, queuedHlStyle, m.filteredCommands[index].Invalid, typ)
	if pad := (width - 1) - lipgloss.Width(badgePlain+rowPlain(display, typ)); pad > 0 {
		content += queuedBaseStyle.Render(strings.Repeat(" ", pad))
	}
	return " " + content
}

// rowContent styles a list row's text (location + command), highlighting the
// fuzzy-matched characters. baseLoc/baseCmd style the location and command
// segments; hl styles matches. matched is keyed on byte offsets into display.
// typ, when non-empty, is appended as a dim "[type]" badge so the project type
// shows in the list (not just the preview); it is not part of display, so
// fuzzy-match offsets are unaffected. When invalid, the row is drawn in peach
// with an underline so an un-aliasable name (space, '*', …) stands out; the
// style copies keep each state's background, so cursor/queued fills are preserved.
func rowContent(display string, matched map[int]bool, baseLoc, baseCmd, hl lipgloss.Style, invalid bool, typ string) string {
	if invalid {
		baseLoc = baseLoc.Foreground(lipgloss.Color(ccPeach)).Underline(true)
		baseCmd = baseCmd.Foreground(lipgloss.Color(ccPeach)).Underline(true)
		hl = hl.Underline(true)
	}
	badge := ""
	if typ != "" {
		// Dim overlay foreground, keeping baseCmd's background so the badge blends
		// into cursor/queued row fills.
		badge = baseCmd.Foreground(lipgloss.Color(ccOverlay0)).Underline(false).Render(typeBadgePlain(typ))
	}
	if loc, rest, ok := strings.Cut(display, ": "); ok {
		sep := len(loc) + len(": ")
		var b strings.Builder
		b.WriteString(baseLoc.Render(locIcon()))
		b.WriteString(highlightMatches(loc, shiftMatched(matched, 0, len(loc)), baseLoc, hl))
		b.WriteString(baseLoc.Render(": "))
		b.WriteString(highlightMatches(rest, shiftMatched(matched, sep, len(display)), baseCmd, hl))
		b.WriteString(badge)
		return b.String()
	}
	return highlightMatches(display, matched, baseCmd, hl) + badge
}

// typeBadgePlain renders the trailing project-type tag (e.g. " [npm]"), or "" for
// an empty type. Kept in sync with rowContent so rowPlain measures the same text.
func typeBadgePlain(typ string) string {
	if typ == "" {
		return ""
	}
	return " [" + typ + "]"
}

// rowPlain returns the visible (unstyled) text of a row, matching rowContent's
// layout, for column-width measurement.
func rowPlain(display, typ string) string {
	if loc, rest, ok := strings.Cut(display, ": "); ok {
		return locIcon() + loc + ": " + rest + typeBadgePlain(typ)
	}
	return display + typeBadgePlain(typ)
}

// Nerd Font glyphs. Set PLT_NO_ICONS to fall back to plain ASCII for terminals
// without a patched font.
func iconsEnabled() bool { return os.Getenv("PLT_NO_ICONS") == "" }

func locIcon() string {
	if iconsEnabled() {
		return " " // nf-fa-folder
	}
	return ""
}

// searchPromptGlyph is the textinput prompt for the fuzzy search line.
func searchPromptGlyph() string {
	if iconsEnabled() {
		return "❯ "
	}
	return "> "
}

// fuzzyFilter keeps the commands whose Display is a subsequence match for query
// and orders them best-match-first. query need not be lowercased. Ties in match
// quality keep the input order, so a frecency-presorted input surfaces the more
// frequently used command among equally good textual matches (see
// updateFilteredCommands).
func (m Model) fuzzyFilter(commands []CommandInfo, query string) []CommandInfo {
	if query == "" {
		return commands
	}

	query = strings.ToLower(query)

	type scoredCommand struct {
		cmd   CommandInfo
		score int
	}
	var matches []scoredCommand
	for _, cmd := range commands {
		if score, _, ok := fuzzyScore(strings.ToLower(cmd.Display), query); ok {
			matches = append(matches, scoredCommand{cmd: cmd, score: score})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	filtered := make([]CommandInfo, len(matches))
	for i, mt := range matches {
		filtered[i] = mt.cmd
	}
	return filtered
}

// Scoring weights for fuzzyScore. Bonuses reward matches that read as whole
// words (a match at a word boundary) and as contiguous runs, so typing a word
// prefix floats the intended command to the top. The gap and leading penalties
// are deliberately small relative to the bonuses, acting as tiebreakers that
// prefer tighter, earlier matches without overriding a genuine word match.
const (
	boundaryBonus    = 16 // matched char starts a word (index 0 or preceded by a separator)
	consecutiveBonus = 16 // matched char immediately follows the previous match
	gapPenalty       = 3  // per skipped char between two consecutive matches
	leadingPenalty   = 1  // per skipped char before the first match
)

// isWordBoundary reports whether the char at index i in text begins a word: the
// start of the string, or immediately after a separator. text is expected to be
// lowercased already.
func isWordBoundary(text string, i int) bool {
	if i == 0 {
		return true
	}
	switch text[i-1] {
	case ' ', ':', '-', '_', '/', '.':
		return true
	}
	return false
}

// fuzzyScore matches query as a subsequence of text (both expected to be
// lowercased by the caller) and returns a quality score together with the byte
// offsets of the matched characters. ok is false when query is not a subsequence
// of text. Higher scores are better matches.
//
// Unlike a greedy left-most match, this considers every valid alignment via a
// small dynamic program and keeps the highest-scoring one, so it prefers
// word-boundary and contiguous matches over scattered ones. text and query are
// short (one display line, a few keystrokes), so the O(len(query)*len(text)^2)
// cost is negligible.
func fuzzyScore(text, query string) (int, []int, bool) {
	if query == "" {
		return 0, nil, true
	}
	n := len(text)
	m := len(query)

	// best[j][t]: best score matching query[:j+1] with query[j] placed at text
	// position t; prev[j][t]: the position chosen for query[j-1] on that path
	// (-1 for j==0), or -2 when the cell is unreachable.
	const unreachable = -1 << 30
	best := make([][]int, m)
	prev := make([][]int, m)
	for j := range best {
		best[j] = make([]int, n)
		prev[j] = make([]int, n)
		for t := range best[j] {
			best[j][t] = unreachable
			prev[j][t] = -2
		}
	}

	for t := 0; t < n; t++ {
		if text[t] != query[0] {
			continue
		}
		score := 0
		if isWordBoundary(text, t) {
			score += boundaryBonus
		}
		score -= leadingPenalty * t
		best[0][t] = score
		prev[0][t] = -1
	}

	for j := 1; j < m; j++ {
		for t := j; t < n; t++ {
			if text[t] != query[j] {
				continue
			}
			for p := j - 1; p < t; p++ {
				if best[j-1][p] == unreachable {
					continue
				}
				score := best[j-1][p]
				if t == p+1 {
					score += consecutiveBonus
				} else {
					score -= gapPenalty * (t - p - 1)
				}
				if isWordBoundary(text, t) {
					score += boundaryBonus
				}
				if score > best[j][t] {
					best[j][t] = score
					prev[j][t] = p
				}
			}
		}
	}

	// Pick the best endpoint for the final query char.
	bestEnd := -1
	bestScore := unreachable
	for t := m - 1; t < n; t++ {
		if best[m-1][t] > bestScore {
			bestScore = best[m-1][t]
			bestEnd = t
		}
	}
	if bestEnd == -1 {
		return 0, nil, false
	}

	positions := make([]int, m)
	t := bestEnd
	for j := m - 1; j >= 0; j-- {
		positions[j] = t
		t = prev[j][t]
	}
	return bestScore, positions, true
}

// fuzzySubsequence reports whether every character of query appears in text in
// order (a subsequence match). Both are expected to already be lowercased by the
// caller. Used by the init wizard's boolean filter, where no ranking is needed.
func fuzzySubsequence(text, query string) bool {
	_, _, ok := fuzzyScore(text, query)
	return ok
}

// fuzzySubsequenceIndices returns the byte offsets in text of the characters
// matched for query, choosing the same best alignment fuzzyScore ranks by so the
// highlighted characters reflect the ranked match. query is expected to already
// be lowercased; text is matched case-insensitively against its lowercased form,
// and the returned offsets index into the ORIGINAL text. This assumes ASCII (all
// authored commands are), where lowercasing preserves byte positions. Returns nil
// when query is empty or does not match.
func fuzzySubsequenceIndices(text, query string) []int {
	if query == "" {
		return nil
	}
	_, positions, ok := fuzzyScore(strings.ToLower(text), query)
	if !ok {
		return nil
	}
	return positions
}

// matchedSet returns the set of byte offsets in display matched by query
// (already lowercased), or nil when there is no active query or no match.
func matchedSet(display, query string) map[int]bool {
	if query == "" {
		return nil
	}
	idx := fuzzySubsequenceIndices(display, query)
	if len(idx) == 0 {
		return nil
	}
	set := make(map[int]bool, len(idx))
	for _, i := range idx {
		set[i] = true
	}
	return set
}

// shiftMatched returns the subset of matched offsets within [start,end),
// re-based to start at 0 — used to split a whole-display match set across the
// location and command segments of a row.
func shiftMatched(matched map[int]bool, start, end int) map[int]bool {
	if len(matched) == 0 {
		return nil
	}
	out := make(map[int]bool)
	for i := range matched {
		if i >= start && i < end {
			out[i-start] = true
		}
	}
	return out
}

// highlightMatches renders text with the characters at the given byte offsets
// styled with hl and everything else with base. The visible (ANSI-stripped)
// output is identical to text — only styling differs.
func highlightMatches(text string, matched map[int]bool, base, hl lipgloss.Style) string {
	if len(matched) == 0 {
		return base.Render(text)
	}
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		ch := text[i : i+1]
		if matched[i] {
			b.WriteString(hl.Render(ch))
		} else {
			b.WriteString(base.Render(ch))
		}
	}
	return b.String()
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

// confirmPaneSelection resolves the current selection (queue or cursor) and tags
// every result with the "pane" action, so the shell wrapper opens the command(s)
// in a new tmux window / zellij tab rather than running them in the current shell.
func (m *Model) confirmPaneSelection() {
	m.results = m.getSelectedCommands()
	for i := range m.results {
		m.results[i].Action = "pane"
	}
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
