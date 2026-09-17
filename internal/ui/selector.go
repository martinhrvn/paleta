package ui

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/history"
	"github.com/martinhrvn/paleta/internal/mux"
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

	// backend is the selector's seam to the rest of the program (see Backend).
	backend Backend

	focusActive bool // session toggle: show only focused locations

	// mode is the sub-mode the selector is in; see selectorMode. Exactly one at a
	// time, so the key, message and view dispatch all switch on it (handler).
	mode selectorMode

	// Focus picker mode (Ctrl+P)
	focusItems  []FocusEntry
	focusCursor int

	// Queue editor mode (Ctrl+Q): reorder/remove/save the queued commands.
	queueCursor int
	saveInput   textinput.Model // the "Save as" prompt of the save sub-mode
	queueHint   string          // transient message shown in the editor (e.g. save constraints)

	// reinit is set when the user requests adding projects (Ctrl+N). The
	// selector quits and the caller runs the init wizard before re-entering.
	reinit bool

	// Edit mode
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

	// spinner animates the placeholder rows of locations still being resolved.
	spinner spinner.Model

	// State
	quitting bool
}

// selectorMode is the sub-mode the selector is in. The palette (modeNormal)
// hands the keyboard to one sub-mode at a time; each sub-mode returns to the
// palette on Esc.
type selectorMode int

const (
	modeNormal    selectorMode = iota // the palette: search, cursor, queue
	modeEdit                          // Ctrl+E: edit the command under the cursor before running
	modeFocusPick                     // Ctrl+P: choose which locations are focused
	modeQueueEdit                     // Ctrl+Q: reorder, remove or save the queued commands
	modeQueueSave                     // "s" in the queue editor: name the command to save
)

func (s selectorMode) String() string {
	switch s {
	case modeNormal:
		return "normal"
	case modeEdit:
		return "edit"
	case modeFocusPick:
		return "focus-pick"
	case modeQueueEdit:
		return "queue-edit"
	case modeQueueSave:
		return "queue-save"
	}
	return fmt.Sprintf("selectorMode(%d)", int(s))
}

// modeHandler is what a sub-mode contributes: its key handler, the text input
// that owns non-key messages (cursor blink) if it has one, and its view (nil
// means the palette view). handler is the one place a mode is dispatched, so
// adding a sub-mode is one new case there plus one entry in the enum.
type modeHandler struct {
	key   func(Model, tea.KeyMsg) (tea.Model, tea.Cmd)
	input func(*Model) *textinput.Model
	view  func(Model) string
}

func (m Model) handler() modeHandler {
	switch m.mode {
	case modeEdit:
		return modeHandler{
			key:   Model.updateEditMode,
			input: func(m *Model) *textinput.Model { return &m.editInput },
		}
	case modeFocusPick:
		return modeHandler{
			key:  Model.updateFocusPickMode,
			view: Model.renderFocusPicker,
		}
	case modeQueueEdit:
		return modeHandler{
			key:  Model.updateQueueEditMode,
			view: Model.renderQueueEditor,
		}
	case modeQueueSave:
		return modeHandler{
			key:   Model.updateQueueSaveMode,
			input: func(m *Model) *textinput.Model { return &m.saveInput },
			view:  Model.renderQueueEditor,
		}
	default:
		return modeHandler{
			key:   Model.updateNormalMode,
			input: func(m *Model) *textinput.Model { return &m.searchInput },
		}
	}
}

// leaveMode returns to the palette from any sub-mode and gives the search box
// the keyboard back.
func (m *Model) leaveMode() {
	m.mode = modeNormal
	m.editInput.Blur()
	m.saveInput.Blur()
	m.searchInput.Focus()
}

// NewModel creates the selector for cfg. be supplies everything outside the
// terminal; see Backend for what each nil field disables.
func NewModel(cfg *config.Config, be Backend) Model {
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

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = listLocationStyle

	m := Model{
		config:          cfg,
		frecencyEnabled: cfg.Frecency.Enabled,
		backend:         be,
		history:         be.History,
		focusActive:     cfg.AnyFocused(),
		searchInput:     si,
		editInput:       ei,
		saveInput:       sv,
		spinner:         sp,
		mux:             mux.DetectEnv(),
	}

	return m
}

// Init implements tea.Model. Besides the cursor blink it kicks off one background
// command per location whose project types still have to be resolved, so the
// palette paints immediately and those rows fill in as they arrive.
func (m Model) Init() tea.Cmd {
	cmds := m.resolvePendingCmds()
	if len(cmds) > 0 {
		cmds = append(cmds, m.spinner.Tick)
	}
	return tea.Batch(append(cmds, textinput.Blink)...)
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case pendingResolvedMsg:
		m.applyPendingResolved(msg)
		return m, nil

	case spinner.TickMsg:
		// Keep ticking only while something is still resolving.
		if !m.config.HasPendingTypes() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		return m.handler().key(m, msg)
	}

	// Anything else (cursor blink) goes to the mode's text input, if it has one.
	h := m.handler()
	if h.input == nil {
		return m, nil
	}
	in := h.input(&m)
	prev := in.Value()
	var cmd tea.Cmd
	*in, cmd = in.Update(msg)
	if m.mode == modeNormal && in.Value() != prev {
		m.updateFilteredCommands()
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

	if view := m.handler().view; view != nil {
		return view(m)
	}

	var sections []string

	// Search / edit input
	if m.mode == modeEdit {
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
	if m.mode == modeEdit {
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

// listHeight is the number of list rows the palette can draw: the terminal
// height minus the chrome (search line, status line, help line, and the warning
// banner when shown). The viewport and the renderer both use it, so the cursor
// can never scroll onto a row that isn't drawn.
func (m Model) listHeight() int {
	chrome := 3
	if len(m.config.Warnings) > 0 {
		chrome++
	}
	if h := m.height - chrome; h >= 1 {
		return h
	}
	return 10 // no size yet: a sensible default
}

func (m Model) renderMainContent() string {
	listHeight := m.listHeight()

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

	start, end := visibleWindow(m.viewportOffset, height, len(m.filteredCommands))

	var lines []string
	for i := start; i < end; i++ {
		pos := m.queuePosAt(i)
		matched := matchedFrom(m.filteredCommands[i].matched)

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

	return strings.Join(padLines(lines, height), "\n")
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

	for _, row := range m.config.Rows(m.focusActive) {
		info := CommandInfo{
			// The row shows the plain label; the project type is rendered separately
			// as a trailing badge (see rowContent), which is what keeps same-named
			// commands across types apart.
			Display:       row.Label(),
			Directory:     row.Directory,
			Command:       row.Command,
			DisplayName:   row.DisplayName,
			Type:          row.Type,
			Env:           row.Env,
			Invalid:       row.Invalid != "",
			InvalidReason: row.Invalid,
			IsTool:        row.IsTool,
		}
		switch {
		case len(row.Pending) > 0:
			// A location whose types are still resolving gets a placeholder so the
			// palette shows that something is on its way rather than looking empty.
			// The marker is swapped for the live spinner frame at render time.
			info.Display = fmt.Sprintf("%s: %s %s…", row.DisplayName, loadingRowMarker,
				strings.Join(row.Pending, ", "))
			info.Loading = true
		case row.IsTool:
			// A tool row keeps no command name: tools can't be referenced as
			// @project:command aliases, so a saved queue stores them verbatim.
		default:
			info.Name = row.Name
			if m.history != nil && m.frecencyEnabled {
				info.FrecencyScore = m.history.GetScore(row.DisplayName, row.Command)
			}
		}
		m.commands = append(m.commands, info)
	}
}

// reloadConfig re-reads the config through the backend after a save so the list
// reflects it; the types the reload defers are resolved again in the background
// by the returned command. Without a Reload, or on failure, the existing config
// is kept.
func (m *Model) reloadConfig() tea.Cmd {
	if m.backend.Reload == nil {
		return nil
	}
	cfg, err := m.backend.Reload()
	if err != nil {
		return nil
	}
	m.config = cfg
	cmds := m.resolvePendingCmds()
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(append(cmds, m.spinner.Tick)...)
}

// updateFilteredCommands rebuilds the visible list from the loaded rows: ordered
// by frecency when that is on (tool rows pinned last), then narrowed and ranked
// by the query. The loaded rows themselves are never reordered, so the base
// order is always there to fall back to. The cursor returns to the top.
func (m *Model) updateFilteredCommands() {
	ordered := append([]CommandInfo(nil), m.commands...)
	if m.frecencyEnabled && m.history != nil {
		for i := range ordered {
			ordered[i].FrecencyScore = m.history.GetScore(ordered[i].DisplayName, ordered[i].Command)
		}
		// fuzzyFilter's ranking is stable, so among equally good textual matches
		// the more frequently used command stays first: match quality first,
		// frecency as the tiebreak.
		sort.SliceStable(ordered, func(i, j int) bool {
			if ordered[i].IsTool != ordered[j].IsTool {
				return !ordered[i].IsTool
			}
			return ordered[i].FrecencyScore > ordered[j].FrecencyScore
		})
	}

	m.filteredCommands = m.fuzzyFilter(ordered, m.searchInput.Value())
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
	display := m.rowDisplay(index)
	return prefix + rowContent(display, matched, listLocationStyle, listCommandStyle, matchStyle, m.filteredCommands[index].Invalid, m.filteredCommands[index].Type)
}

// renderCursorRow renders the selected list row: a lavender accent bar followed
// by a surface-filled line with fuzzy matches highlighted. Every inner segment
// carries the surface background so the fill has no gaps.
func (m Model) renderCursorRow(index, queuePos int, matched map[int]bool, width int) string {
	if index < 0 || index >= len(m.filteredCommands) {
		return ""
	}
	display := m.rowDisplay(index)
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
	display := m.rowDisplay(index)
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
		// Display is ASCII apart from the loading marker, so offsets into the
		// lowercased text index the original too.
		if score, positions, ok := fuzzyScore(strings.ToLower(cmd.Display), query); ok {
			cmd.matched = positions
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

// matchedFrom turns cached match offsets into the set the renderer highlights,
// or nil when the row has none.
func matchedFrom(idx []int) map[int]bool {
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
	// A placeholder for a still-resolving location has nothing to queue.
	if m.filteredCommands[index].Loading {
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
		if m.filteredCommands[i].Loading {
			continue
		}
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
		if m.filteredCommands[i].Loading {
			continue
		}
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

	// If nothing queued, return the current item under the cursor — unless it is a
	// placeholder for a location still being resolved, which has no command.
	if len(results) == 0 && m.currentIndex >= 0 && m.currentIndex < len(m.filteredCommands) &&
		!m.filteredCommands[m.currentIndex].Loading {
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
	m.mode = modeEdit
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
	m.editCommand = ""
	m.editDirectory = ""
	m.editDisplayName = ""
	m.editEnv = nil
	m.leaveMode()
}

func (m *Model) moveCursorDown() {
	m.currentIndex = moveCursor(m.currentIndex, 1, len(m.filteredCommands))
	m.adjustViewport()
}

func (m *Model) moveCursorUp() {
	m.currentIndex = moveCursor(m.currentIndex, -1, len(m.filteredCommands))
	m.adjustViewport()
}

// adjustViewport scrolls the list so the cursor row is drawn.
func (m *Model) adjustViewport() {
	m.viewportOffset = scrollToCursor(m.currentIndex, m.viewportOffset, m.listHeight())
}

// placeCursorOn moves the cursor to the row with the given queue key, if it is
// still in the list, and scrolls to it.
func (m *Model) placeCursorOn(key string) {
	for i := range m.filteredCommands {
		if queueKey(m.filteredCommands[i]) == key {
			m.currentIndex = i
			m.adjustViewport()
			return
		}
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

// loadingRowMarker stands in for the spinner frame inside a placeholder row's
// Display. The row text is built once in loadCommands, but the frame changes on
// every tick, so rowDisplay substitutes the current one at render time. It is a
// single character, matching a spinner frame's width, so the fuzzy-match indices
// computed against Display still line up with what gets rendered.
const loadingRowMarker = "\x00"

// pendingResolvedMsg carries the outcome of resolving one location's deferred
// project types (see config.ResolvePendingTypes). index is the location's
// position in the config, so results land in the location's own slot no matter
// what order they arrive in.
type pendingResolvedMsg struct {
	index    int
	commands []config.Command
	warnings []config.Warning
}

// resolvePendingCmds returns one background command per location with types left
// to resolve. They run concurrently and each sends a pendingResolvedMsg when it
// finishes, so a slow `./gradlew tasks --all` never delays the palette opening —
// its rows just appear a moment later.
func (m Model) resolvePendingCmds() []tea.Cmd {
	resolve := m.backend.ResolvePending
	if resolve == nil {
		return nil
	}

	var cmds []tea.Cmd
	for i := range m.config.Locations {
		if len(m.config.Locations[i].PendingTypes) == 0 {
			continue
		}
		index := i
		loc := m.config.Locations[i]
		cmds = append(cmds, func() tea.Msg {
			commands, warnings := resolve(loc)
			return pendingResolvedMsg{index: index, commands: commands, warnings: warnings}
		})
	}
	return cmds
}

// applyPendingResolved merges a resolved location's commands into the config and
// rebuilds the list, keeping the cursor on the row it was on.
func (m *Model) applyPendingResolved(msg pendingResolvedMsg) {
	if msg.index < 0 || msg.index >= len(m.config.Locations) {
		return
	}
	var current string
	if m.currentIndex >= 0 && m.currentIndex < len(m.filteredCommands) {
		current = queueKey(m.filteredCommands[m.currentIndex])
	}

	if msg.commands != nil {
		m.config.Locations[msg.index].Commands = msg.commands
	}
	m.config.Locations[msg.index].PendingTypes = nil
	m.config.Warnings = append(m.config.Warnings, msg.warnings...)

	m.loadCommands()
	m.updateFilteredCommands()
	if current != "" {
		m.placeCursorOn(current)
	}
}

// rowDisplay is the text for a list row. A placeholder for a location whose
// types are still resolving gets the live spinner frame, so the row reads
// "infra: ⠋ make…" and animates until the real commands replace it.
func (m Model) rowDisplay(index int) string {
	if index < 0 || index >= len(m.filteredCommands) {
		return ""
	}
	row := m.filteredCommands[index]
	if !row.Loading {
		return row.Display
	}
	return strings.Replace(row.Display, loadingRowMarker, m.spinner.View(), 1)
}
