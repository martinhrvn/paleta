package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// FocusEntry is one project shown in the Ctrl+P focus picker. Key is the stable
// identity used when persisting the focus set (the location's authored name or
// path); Label is what the user sees.
type FocusEntry struct {
	Key     string
	Label   string
	Focused bool
}

// enterFocusPicker loads the current focus set and switches into picker mode.
// It is a no-op when the backend offers no focus persistence.
func (m *Model) enterFocusPicker() {
	if m.backend.ListFocus == nil {
		return
	}
	entries, err := m.backend.ListFocus()
	if err != nil || len(entries) == 0 {
		return
	}
	m.focusItems = entries
	m.focusCursor = 0
	m.mode = modeFocusPick
	m.searchInput.Blur()
}

func (m Model) updateFocusPickMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp, tea.KeyCtrlK:
		m.focusCursor = moveCursor(m.focusCursor, -1, len(m.focusItems))
		return m, nil

	case tea.KeyDown, tea.KeyCtrlJ:
		m.focusCursor = moveCursor(m.focusCursor, 1, len(m.focusItems))
		return m, nil

	case tea.KeySpace, tea.KeyTab:
		if m.focusCursor >= 0 && m.focusCursor < len(m.focusItems) {
			m.focusItems[m.focusCursor].Focused = !m.focusItems[m.focusCursor].Focused
		}
		return m, nil

	case tea.KeyCtrlA:
		m.toggleFocusAll()
		return m, nil

	case tea.KeyEnter:
		return m, m.confirmFocusPicker()

	case tea.KeyEscape:
		m.exitFocusPicker()
		return m, nil
	}
	return m, nil
}

// toggleFocusAll focuses every item, or unfocuses all when they are already all
// focused (matching the main list's Ctrl+A behavior).
func (m *Model) toggleFocusAll() {
	allFocused := true
	for _, it := range m.focusItems {
		if !it.Focused {
			allFocused = false
			break
		}
	}
	for i := range m.focusItems {
		m.focusItems[i].Focused = !allFocused
	}
}

// confirmFocusPicker persists the chosen focus set, reloads the config so the
// list reflects it, and leaves picker mode. The returned command resumes any
// background type resolution the reload deferred.
func (m *Model) confirmFocusPicker() tea.Cmd {
	var cmd tea.Cmd
	if m.backend.SaveFocus != nil {
		focused := make(map[string]bool, len(m.focusItems))
		for _, it := range m.focusItems {
			focused[it.Key] = it.Focused
		}
		if err := m.backend.SaveFocus(focused); err == nil {
			cmd = m.reloadConfig()
			// Make the effect visible immediately: show the focused view when
			// anything is focused, otherwise fall back to showing everything.
			m.focusActive = m.config.AnyFocused()
			m.loadCommands()
			m.updateFilteredCommands()
		}
	}
	m.exitFocusPicker()
	return cmd
}

func (m *Model) exitFocusPicker() {
	m.focusItems = nil
	m.leaveMode()
}

func (m Model) renderFocusPicker() string {
	var lines []string
	lines = append(lines, previewTitleStyle.Render("Focus projects"))
	lines = append(lines, statusStyle.Render("Choose which projects appear by default."))
	lines = append(lines, "")

	for i, it := range m.focusItems {
		box := "[ ]"
		if it.Focused {
			box = selectedMarkStyle.Render("[x]")
		}
		row := box + " " + listCommandStyle.Render(it.Label)
		if i == m.focusCursor {
			plain := "[ ] " + it.Label
			if it.Focused {
				plain = "[x] " + it.Label
			}
			row = cursorLineStyle.Render(plain)
		}
		lines = append(lines, "  "+row)
	}

	lines = append(lines, "")
	lines = append(lines, m.renderHelp([][2]string{
		{"Space", "toggle"},
		{"^A", "all"},
		{"Enter", "save"},
		{"Esc", "cancel"},
	}))

	return strings.Join(lines, "\n")
}
