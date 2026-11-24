package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// enterWorktreePicker opens the Ctrl+W list of the repository's checkouts with
// the cursor on the current one. It is a no-op when there is nothing to switch
// to (the backend lists worktrees only when there are at least two).
func (m *Model) enterWorktreePicker() {
	if len(m.backend.Worktrees) == 0 {
		return
	}
	m.worktreeCursor = 0
	for i, wt := range m.backend.Worktrees {
		if wt.Current {
			m.worktreeCursor = i
			break
		}
	}
	m.mode = modeWorktreePick
	m.searchInput.Blur()
}

func (m Model) updateWorktreePickMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp, tea.KeyCtrlK:
		m.worktreeCursor = moveCursor(m.worktreeCursor, -1, len(m.backend.Worktrees))
		return m, nil

	case tea.KeyDown, tea.KeyCtrlJ:
		m.worktreeCursor = moveCursor(m.worktreeCursor, 1, len(m.backend.Worktrees))
		return m, nil

	case tea.KeyEnter:
		return m, m.confirmWorktreePicker()

	case tea.KeyRunes:
		if string(msg.Runes) == "c" {
			return m, m.confirmWorktreeCd()
		}
		return m, nil

	case tea.KeyEscape:
		m.leaveMode()
		return m, nil
	}
	return m, nil
}

// confirmWorktreePicker switches to the checkout under the cursor: the program
// ends and the caller re-enters the selector with the config re-rooted there.
// Choosing the current checkout just returns to the palette.
func (m *Model) confirmWorktreePicker() tea.Cmd {
	if m.worktreeCursor < 0 || m.worktreeCursor >= len(m.backend.Worktrees) {
		m.leaveMode()
		return nil
	}
	chosen := m.backend.Worktrees[m.worktreeCursor]
	if chosen.Current {
		m.leaveMode()
		return nil
	}
	m.worktree = chosen.Path
	m.quitting = true
	return tea.Quit
}

// confirmWorktreeCd ends the program with a "cd" result for the checkout under
// the cursor, so the shell wrapper changes the shell's directory there (the
// caller keeps your place inside the repository). Choosing the current checkout
// just returns to the palette.
func (m *Model) confirmWorktreeCd() tea.Cmd {
	if m.worktreeCursor < 0 || m.worktreeCursor >= len(m.backend.Worktrees) {
		m.leaveMode()
		return nil
	}
	chosen := m.backend.Worktrees[m.worktreeCursor]
	if chosen.Current {
		m.leaveMode()
		return nil
	}
	m.results = []SelectionResult{{Directory: chosen.Path, DisplayName: chosen.Label, Action: "cd"}}
	m.quitting = true
	return tea.Quit
}

func (m Model) renderWorktreePicker() string {
	var lines []string
	lines = append(lines, previewTitleStyle.Render("Switch worktree"))
	lines = append(lines, statusStyle.Render("Enter shows this project's commands in another checkout; c moves your shell there."))
	lines = append(lines, "")

	for i, wt := range m.backend.Worktrees {
		mark := " "
		if wt.Current {
			mark = "*"
		}
		plain := mark + " " + wt.Label + "  " + wt.Path
		row := mark + " " + listCommandStyle.Render(wt.Label) + "  " + listLocationStyle.Render(wt.Path)
		if i == m.worktreeCursor {
			row = cursorLineStyle.Render(plain)
		}
		lines = append(lines, "  "+row)
	}

	lines = append(lines, "")
	lines = append(lines, m.renderHelp([][2]string{
		{"Enter", "switch"},
		{"c", "cd there"},
		{"Esc", "cancel"},
	}))

	return strings.Join(lines, "\n")
}
