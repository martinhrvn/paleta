package ui

import (
	"fmt"
	"os"
	"strings"

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

// wizardMode is the wizard's current view. Only listMode exists today; a future
// per-location command include/exclude view will add a detail mode here.
type wizardMode int

const (
	listMode wizardMode = iota
)

// WizardModel is the bubbletea model for the interactive `plt init` wizard.
type WizardModel struct {
	items    []WizardItem
	selected map[int]bool
	cursor   int
	mode     wizardMode
	width    int
	height   int

	confirmed bool
	quitting  bool
}

// NewWizardModel creates a wizard model with every detected or already-configured
// item pre-selected.
func NewWizardModel(items []WizardItem) WizardModel {
	selected := make(map[int]bool, len(items))
	for i, it := range items {
		if it.Detected || it.Configured {
			selected[i] = true
		}
	}
	return WizardModel{
		items:    items,
		selected: selected,
		mode:     listMode,
	}
}

// toggle flips the selection state of the item at index i.
func (m *WizardModel) toggle(i int) {
	if i < 0 || i >= len(m.items) {
		return
	}
	if m.selected[i] {
		delete(m.selected, i)
	} else {
		m.selected[i] = true
	}
}

// toggleAll selects every item when any is unselected, otherwise deselects all.
func (m *WizardModel) toggleAll() {
	allSelected := len(m.selected) == len(m.items)
	if allSelected {
		m.selected = make(map[int]bool)
		return
	}
	for i := range m.items {
		m.selected[i] = true
	}
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
	return nil
}

func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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
		case tea.KeyTab, tea.KeySpace:
			m.toggle(m.cursor)
			return m, nil
		case tea.KeyCtrlA:
			m.toggleAll()
			return m, nil
		case tea.KeyRunes:
			switch string(msg.Runes) {
			case "j":
				m.moveCursor(1)
			case "k":
				m.moveCursor(-1)
			case " ":
				m.toggle(m.cursor)
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *WizardModel) moveCursor(delta int) {
	if len(m.items) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
}

func (m WizardModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(previewTitleStyle.Render("Select projects to include in .pltrc"))
	b.WriteString("\n\n")

	for i, it := range m.items {
		mark := "[ ]"
		if m.selected[i] {
			mark = selectedMarkStyle.Render("[x]")
		}

		path := it.Location.Location
		line := fmt.Sprintf("%s %s", mark, listCommandStyle.Render(path))
		if len(it.Location.Types) > 0 {
			line += " " + listLocationStyle.Render("("+strings.Join(it.Location.Types, ", ")+")")
		}
		if it.Configured {
			line += " " + statusGreenStyle.Render("(configured)")
		}

		if i == m.cursor {
			line = cursorLineStyle.Render("› " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.helpLine())
	return b.String()
}

func (m WizardModel) helpLine() string {
	parts := []struct{ key, desc string }{
		{"↑/↓", "move"},
		{"space/tab", "toggle"},
		{"ctrl+a", "all"},
		{"enter", "confirm"},
		{"esc", "cancel"},
	}
	var sb strings.Builder
	for i, p := range parts {
		if i > 0 {
			sb.WriteString(helpStyle.Render("  •  "))
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
