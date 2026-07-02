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

// FocusStore lets the selector read and persist the focus set without the ui
// package depending on the commands package. It is nil when there is no writable
// local .pltrc (e.g. a global-fallback config), in which case the picker is
// disabled.
type FocusStore struct {
	List func() ([]FocusEntry, error)
	Save func(focused map[string]bool) error
}

// enterFocusPicker loads the current focus set and switches into picker mode.
// It is a no-op when no writable focus store is available.
func (m *Model) enterFocusPicker() {
	if m.focus == nil {
		return
	}
	entries, err := m.focus.List()
	if err != nil || len(entries) == 0 {
		return
	}
	m.focusItems = entries
	m.focusCursor = 0
	m.focusPicking = true
	m.searchInput.Blur()
}

func (m Model) updateFocusPickMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp, tea.KeyCtrlK:
		if m.focusCursor > 0 {
			m.focusCursor--
		}
		return m, nil

	case tea.KeyDown, tea.KeyCtrlJ:
		if m.focusCursor < len(m.focusItems)-1 {
			m.focusCursor++
		}
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
		m.confirmFocusPicker()
		return m, nil

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
// list reflects it, and leaves picker mode.
func (m *Model) confirmFocusPicker() {
	if m.focus != nil && m.focus.Save != nil {
		focused := make(map[string]bool, len(m.focusItems))
		for _, it := range m.focusItems {
			focused[it.Key] = it.Focused
		}
		if err := m.focus.Save(focused); err == nil {
			m.reloadConfig()
			// Make the effect visible immediately: show the focused view when
			// anything is focused, otherwise fall back to showing everything.
			m.focusActive = m.config.AnyFocused()
			m.loadCommands()
			m.updateFilteredCommands()
		}
	}
	m.exitFocusPicker()
}

func (m *Model) exitFocusPicker() {
	m.focusPicking = false
	m.focusItems = nil
	m.searchInput.Focus()
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
