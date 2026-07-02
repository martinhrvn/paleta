package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/martinhrvn/paleta/internal/config"
)

// WizardItem is a single row presented in the interactive init wizard. It wraps
// a config.Location (synthesized from a scan candidate or read from an existing
// .pltrc) plus flags describing where it came from. It lives in this package so
// the wizard UI and the command layer can share it without an import cycle.
type WizardItem struct {
	Location   config.Location
	Detected   bool // found by scanning the filesystem
	Configured bool // already present in the existing .pltrc
}

// WizardModel is the bubbletea model for the interactive `plt init` wizard. It
// mirrors the command palette (fzf_tui_selector.go): a fuzzy search box on top,
// a scrolling checkbox list, and the same keys (Tab select, Ctrl+A all, Ctrl+U
// clear, Enter confirm). Selection is keyed by the item's index in the full
// items slice so it survives filtering, while filtered holds the indices of the
// items currently visible under the query.
type WizardModel struct {
	items    []WizardItem
	filtered []int        // indices into items currently matching the query
	selected map[int]bool // keyed by original item index, so it survives filtering
	cursor   int          // position within filtered
	width    int
	height   int

	searchInput textinput.Model

	viewportOffset int
	confirmed      bool
	quitting       bool
}

// NewWizardModel creates a wizard model with only the already-configured items
// pre-selected, so re-running the wizard defaults to the current config rather
// than sweeping in every newly detected project. The cursor starts on the first
// unselected item — the first candidate the user might want to add — falling
// back to the top when everything is already selected.
func NewWizardModel(items []WizardItem) WizardModel {
	si := textinput.New()
	si.Prompt = "> "
	si.PromptStyle = searchPromptStyle
	si.Focus()

	selected := make(map[int]bool, len(items))
	for i, it := range items {
		if it.Configured {
			selected[i] = true
		}
	}

	m := WizardModel{
		items:       items,
		selected:    selected,
		searchInput: si,
	}
	m.applyFilter()
	m.cursor = m.firstUnselectedPos()
	m.adjustViewport()
	return m
}

// applyFilter rebuilds the filtered index list from the current query and resets
// the cursor to the top. Selection state is intentionally left untouched.
func (m *WizardModel) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))
	m.filtered = m.filtered[:0]
	for i, it := range m.items {
		if query == "" || fuzzySubsequence(strings.ToLower(itemSearchText(it)), query) {
			m.filtered = append(m.filtered, i)
		}
	}
	m.cursor = 0
	m.viewportOffset = 0
}

// itemSearchText is the haystack a query is matched against: display name, path,
// and project types.
func itemSearchText(it WizardItem) string {
	parts := append([]string{it.Location.Name, it.Location.Location}, it.Location.Types...)
	return strings.Join(parts, " ")
}

// firstUnselectedPos returns the filtered position of the first unselected item,
// or 0 when everything visible is already selected.
func (m WizardModel) firstUnselectedPos() int {
	for pos, orig := range m.filtered {
		if !m.selected[orig] {
			return pos
		}
	}
	return 0
}

// toggle flips the selection state of the item at filtered position pos.
func (m *WizardModel) toggle(pos int) {
	if pos < 0 || pos >= len(m.filtered) {
		return
	}
	orig := m.filtered[pos]
	if m.selected[orig] {
		delete(m.selected, orig)
	} else {
		m.selected[orig] = true
	}
}

// toggleAll selects every visible item, or deselects them all when the visible
// set is already fully selected.
func (m *WizardModel) toggleAll() {
	if m.allFilteredSelected() {
		for _, orig := range m.filtered {
			delete(m.selected, orig)
		}
		return
	}
	for _, orig := range m.filtered {
		m.selected[orig] = true
	}
}

func (m WizardModel) allFilteredSelected() bool {
	if len(m.filtered) == 0 {
		return false
	}
	for _, orig := range m.filtered {
		if !m.selected[orig] {
			return false
		}
	}
	return true
}

// SelectedLocations returns the locations the user kept, in list order. It
// returns nil unless the selection was confirmed.
func (m WizardModel) SelectedLocations() []config.Location {
	if !m.confirmed {
		return nil
	}
	var locs []config.Location
	for i, it := range m.items {
		if m.selected[i] {
			locs = append(locs, it.Location)
		}
	}
	return locs
}

func (m WizardModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustViewport()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			m.confirmed = true
			m.quitting = true
			return m, tea.Quit
		case tea.KeyUp, tea.KeyCtrlK:
			m.moveCursor(-1)
			return m, nil
		case tea.KeyDown, tea.KeyCtrlJ:
			m.moveCursor(1)
			return m, nil
		case tea.KeyTab:
			m.toggle(m.cursor)
			m.moveCursor(1)
			return m, nil
		case tea.KeyCtrlA:
			m.toggleAll()
			return m, nil
		case tea.KeyCtrlL, tea.KeyCtrlU:
			m.searchInput.SetValue("")
			m.applyFilter()
			return m, nil
		}

		// Anything else is text entry into the search box; re-filter on change.
		prev := m.searchInput.Value()
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		if m.searchInput.Value() != prev {
			m.applyFilter()
		}
		return m, cmd
	}

	// Non-key messages (e.g. cursor blink) go to the search input.
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m *WizardModel) moveCursor(delta int) {
	if len(m.filtered) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	m.adjustViewport()
}

// listHeight is the number of list rows available after chrome (title, search,
// status, and help lines).
func (m WizardModel) listHeight() int {
	h := m.height - 4
	if h < 1 {
		return 10
	}
	return h
}

func (m *WizardModel) adjustViewport() {
	rows := m.listHeight()
	if m.cursor >= m.viewportOffset+rows {
		m.viewportOffset = m.cursor - rows + 1
	}
	if m.cursor < m.viewportOffset {
		m.viewportOffset = m.cursor
	}
	if m.viewportOffset < 0 {
		m.viewportOffset = 0
	}
}

func (m WizardModel) View() string {
	if m.quitting {
		return ""
	}

	sections := []string{
		previewTitleStyle.Render("Select projects to include in .pltrc"),
		m.searchInput.View(),
		m.renderStatus(),
		m.renderList(),
		m.helpLine(),
	}
	return strings.Join(sections, "\n")
}

func (m WizardModel) renderStatus() string {
	parts := []string{statusStyle.Render(fmt.Sprintf("%d/%d", len(m.filtered), len(m.items)))}
	if n := len(m.selected); n > 0 {
		parts = append(parts, statusGreenStyle.Render(fmt.Sprintf("%d selected", n)))
	}
	if q := m.searchInput.Value(); q != "" {
		parts = append(parts, statusYellowStyle.Render(fmt.Sprintf("'%s'", q)))
	}
	return "  " + strings.Join(parts, statusStyle.Render(" · "))
}

func (m WizardModel) renderList() string {
	if len(m.filtered) == 0 {
		return "  No matches"
	}

	height := m.listHeight()
	start := m.viewportOffset
	end := start + height
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	var lines []string
	for pos := start; pos < end; pos++ {
		lines = append(lines, m.formatRow(pos))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m WizardModel) formatRow(pos int) string {
	orig := m.filtered[pos]
	it := m.items[orig]

	mark := "[ ]"
	if m.selected[orig] {
		mark = selectedMarkStyle.Render("[x]")
	}

	line := fmt.Sprintf("%s %s", mark, listCommandStyle.Render(it.Location.Location))
	if len(it.Location.Types) > 0 {
		line += " " + listLocationStyle.Render("("+strings.Join(it.Location.Types, ", ")+")")
	}
	if it.Configured {
		line += " " + statusGreenStyle.Render("(configured)")
	}

	if pos == m.cursor {
		return cursorLineStyle.Render("› " + line)
	}
	return "  " + line
}

func (m WizardModel) helpLine() string {
	parts := []struct{ key, desc string }{
		{"tab", "select"},
		{"enter", "confirm"},
		{"^a", "all"},
		{"^u", "clear"},
		{"esc", "cancel"},
	}
	var sb strings.Builder
	sb.WriteString("  ")
	for i, p := range parts {
		if i > 0 {
			sb.WriteString(helpStyle.Render(" · "))
		}
		sb.WriteString(helpKeyStyle.Render(p.key))
		sb.WriteString(" ")
		sb.WriteString(helpStyle.Render(p.desc))
	}
	return sb.String()
}

// Run starts the wizard and returns the confirmed selection. The bool is false
// when the user canceled (Esc/Ctrl+C).
func (m *WizardModel) Run() ([]config.Location, bool, error) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	// Prefer a dedicated /dev/tty for both input and output. This keeps stdout
	// clean (it may carry the selection JSON when the wizard is launched from
	// within `plt select`) and gives a clean tty handoff after the selector
	// program has quit. Fall back to stderr output when there is no tty.
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if tty, terr := os.OpenFile("/dev/tty", os.O_RDWR, 0); terr == nil {
		defer tty.Close()
		opts = append(opts, tea.WithInput(tty), tea.WithOutput(tty))
	} else {
		opts = append(opts, tea.WithOutput(os.Stderr))
	}

	p := tea.NewProgram(*m, opts...)
	finalModel, err := p.Run()
	if err != nil {
		return nil, false, err
	}

	fm := finalModel.(WizardModel)
	if !fm.confirmed {
		return nil, false, nil
	}
	return fm.SelectedLocations(), true, nil
}
