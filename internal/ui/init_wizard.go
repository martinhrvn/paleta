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
	Location config.Location
	// TypeOptions are the project types offered for this location, in detection
	// order: what it already declares plus anything newly detected in its folder.
	// Location.Types is the subset that starts ticked. A location with fewer than
	// two options stays a single row — unticking its only type would just mean
	// unticking the location.
	TypeOptions []string
	Detected    bool // found by scanning the filesystem
	Configured  bool // already present in the existing .pltrc
}

// wizardRow is one visible line: a location, or one of its project types.
type wizardRow struct {
	item int    // index into WizardModel.items
	typ  string // "" for the location's own row
}

// WizardModel is the bubbletea model for the interactive `plt init` wizard. It
// mirrors the command palette (fzf_tui_selector.go): a fuzzy search box on top,
// a scrolling checkbox list, and the same keys (Tab toggle, Ctrl+A all, Ctrl+U
// clear, Enter confirm). Selection is keyed by the item's index in the full
// items slice so it survives filtering, while filtered holds the indices of the
// items currently visible under the query and rows holds what's actually drawn —
// each visible location, followed by its project types when it has more than one.
type WizardModel struct {
	items    []WizardItem
	filtered []int        // indices into items currently matching the query
	rows     []wizardRow  // visible rows: each filtered location, plus its type rows
	selected map[int]bool // keyed by original item index, so it survives filtering
	// types holds each location's ticked project types, keyed by item index. Like
	// selected, it is keyed by the original index so it survives filtering.
	types  map[int]map[string]bool
	cursor int // position within rows
	width  int
	height int

	searchInput textinput.Model

	viewportOffset int
	confirmed      bool
	quitting       bool
}

// NewWizardModel creates a wizard model with every item pre-selected, so the
// common cases are a single keystroke: Enter on a first run takes the whole
// detected set, and Enter on a repeat run (Ctrl+N) keeps the current config and
// adds what's new. Rows say which items were already configured and which are
// newly detected, since the ticks no longer distinguish them.
func NewWizardModel(items []WizardItem) WizardModel {
	si := textinput.New()
	si.Prompt = "> "
	si.PromptStyle = searchPromptStyle
	si.Focus()

	// Everything starts ticked: on a first run Enter accepts the whole detected
	// set, and on a repeat run (Ctrl+N) the already-configured locations stay put
	// while the newly detected ones come along, so the wizard reads as "add the
	// new projects". Unticking is how you leave something out.
	selected := make(map[int]bool, len(items))
	types := make(map[int]map[string]bool, len(items))
	for i, it := range items {
		selected[i] = true
		// A location starts with the types it already declares ticked. For a newly
		// detected location that is everything found in its folder; for one already
		// in the config, a type detected since is offered unticked — ticking it
		// changes how that location's existing commands are labelled.
		ticked := make(map[string]bool, len(it.Location.Types))
		for _, typeName := range it.Location.Types {
			ticked[typeName] = true
		}
		types[i] = ticked
	}

	m := WizardModel{
		items:       items,
		selected:    selected,
		types:       types,
		searchInput: si,
	}
	m.applyFilter()
	m.adjustViewport()
	return m
}

// applyFilter rebuilds the filtered index list and the visible rows from the
// current query, and resets the cursor to the top. Selection state is
// intentionally left untouched.
func (m *WizardModel) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))
	m.filtered = m.filtered[:0]
	m.rows = m.rows[:0]
	for i, it := range m.items {
		if query != "" && !fuzzySubsequence(strings.ToLower(itemSearchText(it)), query) {
			continue
		}
		m.filtered = append(m.filtered, i)
		m.rows = append(m.rows, wizardRow{item: i})
		// Type rows follow their location, so a filtered-in location brings its
		// options with it.
		if len(it.TypeOptions) > 1 {
			for _, typeName := range it.TypeOptions {
				m.rows = append(m.rows, wizardRow{item: i, typ: typeName})
			}
		}
	}
	m.cursor = 0
	m.viewportOffset = 0
}

// typeSelected reports whether a location's project type is ticked.
func (m WizardModel) typeSelected(item int, typeName string) bool {
	return m.types[item][typeName]
}

// selectedTypes returns a location's ticked types in TypeOptions order, so the
// written config keeps detection priority.
func (m WizardModel) selectedTypes(item int) config.Types {
	it := m.items[item]
	var out config.Types
	for _, typeName := range it.TypeOptions {
		if m.types[item][typeName] {
			out = append(out, typeName)
		}
	}
	return out
}

// itemSearchText is the haystack a query is matched against: display name, path,
// and every project type on offer — so searching "docker" also finds a folder
// where docker was detected but not yet ticked.
func itemSearchText(it WizardItem) string {
	types := it.TypeOptions
	if len(types) == 0 {
		types = it.Location.Types
	}
	return strings.Join(append([]string{it.Location.Name, it.Location.Location}, types...), " ")
}

// toggle flips the row at visible position pos: a location, or one of its
// project types. The two levels are kept consistent — unticking a location
// clears its types and re-ticking restores them, while unticking a location's
// last type removes the location (it would contribute nothing) and ticking a
// type brings it back.
func (m *WizardModel) toggle(pos int) {
	if pos < 0 || pos >= len(m.rows) {
		return
	}
	row := m.rows[pos]

	if row.typ == "" {
		m.setLocationSelected(row.item, !m.selected[row.item])
		return
	}

	if m.types[row.item][row.typ] {
		delete(m.types[row.item], row.typ)
		if len(m.types[row.item]) == 0 {
			delete(m.selected, row.item)
		}
		return
	}
	m.types[row.item][row.typ] = true
	m.selected[row.item] = true
}

// setLocationSelected ticks or unticks a location along with all of its types.
func (m *WizardModel) setLocationSelected(item int, on bool) {
	if !on {
		delete(m.selected, item)
		m.types[item] = map[string]bool{}
		return
	}
	m.selected[item] = true
	ticked := make(map[string]bool, len(m.items[item].TypeOptions))
	for _, typeName := range m.items[item].TypeOptions {
		ticked[typeName] = true
	}
	m.types[item] = ticked
}

// toggleAll selects every visible location (with its types), or deselects them
// all when the visible set is already fully selected.
func (m *WizardModel) toggleAll() {
	on := !m.allFilteredSelected()
	for _, orig := range m.filtered {
		m.setLocationSelected(orig, on)
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
		if !m.selected[i] {
			continue
		}
		loc := it.Location
		if len(it.TypeOptions) > 0 {
			loc.Types = m.selectedTypes(i)
		}
		locs = append(locs, loc)
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
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEsc:
			// Esc clears a non-empty search first; on an empty search it cancels.
			if m.searchInput.Value() != "" {
				m.searchInput.SetValue("")
				m.applyFilter()
				return m, nil
			}
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
	if len(m.rows) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
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
	if len(m.rows) == 0 {
		return "  No matches"
	}

	height := m.listHeight()
	start := m.viewportOffset
	end := start + height
	if end > len(m.rows) {
		end = len(m.rows)
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
	if pos < 0 || pos >= len(m.rows) {
		return ""
	}
	row := m.rows[pos]
	it := m.items[row.item]

	var line string
	if row.typ == "" {
		line = fmt.Sprintf("%s %s", checkMark(m.selected[row.item]), listCommandStyle.Render(it.Location.Location))
		// A location that lists its types as rows doesn't repeat them inline.
		if len(it.TypeOptions) < 2 && len(it.Location.Types) > 0 {
			line += " " + listLocationStyle.Render("("+strings.Join(it.Location.Types, ", ")+")")
		}
		switch {
		case it.Configured:
			line += " " + statusGreenStyle.Render("(configured)")
		case it.Detected:
			line += " " + statusYellowStyle.Render("(new)")
		}
	} else {
		line = fmt.Sprintf("   %s %s", checkMark(m.types[row.item][row.typ]), listCommandStyle.Render(row.typ))
		// On a location already in the config, a type detected since is an
		// addition the user opts into.
		if it.Configured && !containsString(it.Location.Types, row.typ) {
			line += " " + statusYellowStyle.Render("(new)")
		}
	}

	if pos == m.cursor {
		return cursorLineStyle.Render("› " + line)
	}
	return "  " + line
}

// checkMark renders a row's tick box.
func checkMark(on bool) string {
	if on {
		return selectedMarkStyle.Render("[x]")
	}
	return "[ ]"
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func (m WizardModel) helpLine() string {
	parts := []struct{ key, desc string }{
		{"tab", "toggle"},
		{"enter", "confirm"},
		{"^a", "all/none"},
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
