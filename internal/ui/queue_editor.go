package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// SaveStore lets the selector persist a queued chain of commands into .pltrc
// without the ui package depending on the commands package. It is nil when there
// is no writable local .pltrc (e.g. a global-fallback config), in which case
// saving is disabled. Save receives the target project's display name and
// directory, the new command's name, and the joined command string (a && b && c).
type SaveStore struct {
	Save func(displayName, directory, name, command string) error
}

// SetSaveStore wires the .pltrc save hook. Called by the commands layer after
// constructing the model.
func (m *Model) SetSaveStore(s *SaveStore) {
	m.saveCommand = s
}

// enterQueueEditor switches into the queue editor. It is a no-op when the queue
// is empty (nothing to edit).
func (m *Model) enterQueueEditor() {
	if len(m.queue) == 0 {
		return
	}
	m.queueEditing = true
	m.queueSaving = false
	m.queueCursor = 0
	m.queueHint = ""
	m.searchInput.Blur()
}

func (m *Model) exitQueueEditor() {
	m.queueEditing = false
	m.queueSaving = false
	m.queueHint = ""
	m.saveInput.Blur()
	m.searchInput.Focus()
}

func (m Model) updateQueueEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.queueSaving {
		return m.updateQueueSaveMode(msg)
	}

	switch msg.Type {
	case tea.KeyUp, tea.KeyCtrlK:
		m.moveQueueCursor(-1)
		return m, nil

	case tea.KeyDown, tea.KeyCtrlJ:
		m.moveQueueCursor(1)
		return m, nil

	case tea.KeyShiftUp:
		m.moveQueueItem(-1)
		return m, nil

	case tea.KeyShiftDown:
		m.moveQueueItem(1)
		return m, nil

	case tea.KeyDelete, tea.KeyBackspace:
		m.removeQueueItem()
		return m, nil

	case tea.KeyEnter:
		// Run the queue in its current order.
		m.confirmSelection()
		m.quitting = true
		return m, tea.Quit

	case tea.KeyEscape:
		m.exitQueueEditor()
		return m, nil

	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "j":
			m.moveQueueCursor(1)
		case "k":
			m.moveQueueCursor(-1)
		case "x":
			m.removeQueueItem()
		case "s":
			m.startQueueSave()
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) moveQueueCursor(delta int) {
	if len(m.queue) == 0 {
		return
	}
	m.queueCursor += delta
	if m.queueCursor < 0 {
		m.queueCursor = 0
	}
	if m.queueCursor >= len(m.queue) {
		m.queueCursor = len(m.queue) - 1
	}
}

// moveQueueItem swaps the item under the cursor with its neighbor, keeping the
// cursor on the moved item so repeated presses keep dragging it.
func (m *Model) moveQueueItem(delta int) {
	to := m.queueCursor + delta
	if m.queueCursor < 0 || m.queueCursor >= len(m.queue) || to < 0 || to >= len(m.queue) {
		return
	}
	m.queue[m.queueCursor], m.queue[to] = m.queue[to], m.queue[m.queueCursor]
	m.queueCursor = to
	m.queueHint = ""
}

// removeQueueItem drops the item under the cursor; if the queue empties, it
// leaves the editor.
func (m *Model) removeQueueItem() {
	if m.queueCursor < 0 || m.queueCursor >= len(m.queue) {
		return
	}
	m.queue = append(m.queue[:m.queueCursor], m.queue[m.queueCursor+1:]...)
	m.queueHint = ""
	if len(m.queue) == 0 {
		m.exitQueueEditor()
		return
	}
	if m.queueCursor >= len(m.queue) {
		m.queueCursor = len(m.queue) - 1
	}
}

// updateQueueSaveMode handles the "name this command" text prompt.
func (m Model) updateQueueSaveMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.confirmQueueSave()
		return m, nil
	case tea.KeyEscape:
		m.cancelQueueSave()
		return m, nil
	}
	var cmd tea.Cmd
	m.saveInput, cmd = m.saveInput.Update(msg)
	return m, cmd
}

// startQueueSave enters the save prompt, provided saving is available and the
// queue is savable (all commands in one project).
func (m *Model) startQueueSave() {
	if m.saveCommand == nil || m.saveCommand.Save == nil {
		m.queueHint = "saving to .pltrc is unavailable here"
		return
	}
	if _, ok := m.queueProject(); !ok {
		m.queueHint = "save needs all commands in one project"
		return
	}
	m.queueSaving = true
	m.queueHint = ""
	m.saveInput.SetValue("")
	m.saveInput.Focus()
}

func (m *Model) cancelQueueSave() {
	m.queueSaving = false
	m.saveInput.Blur()
}

// confirmQueueSave joins the queued commands and persists them under the shared
// project, then reloads so the new command appears.
func (m *Model) confirmQueueSave() {
	name := strings.TrimSpace(m.saveInput.Value())
	if name == "" {
		m.queueHint = "enter a name for the command"
		return
	}
	displayName, ok := m.queueProject()
	if !ok {
		m.queueHint = "save needs all commands in one project"
		m.queueSaving = false
		return
	}
	directory := m.queue[0].Directory
	joined := m.buildQueueCommand()

	if err := m.saveCommand.Save(displayName, directory, name, joined); err != nil {
		m.queueHint = "save failed: " + err.Error()
		m.queueSaving = false
		return
	}

	// Reflect the new command in the list immediately.
	m.reloadConfig()
	m.loadCommands()
	m.updateFilteredCommands()
	m.exitQueueEditor()
}

// buildQueueCommand joins the queued commands for saving, preferring reference
// tokens (@project[type]:name) over raw command strings so the saved command
// tracks the referenced commands rather than freezing their current text. It
// falls back to the raw string when a command can't be referenced safely (no
// name, or a project whose name is missing/ambiguous).
func (m Model) buildQueueCommand() string {
	parts := make([]string, len(m.queue))
	for i, c := range m.queue {
		if tok, ok := m.aliasToken(c); ok {
			parts[i] = tok
		} else {
			parts[i] = c.Command
		}
	}
	return strings.Join(parts, " && ")
}

// aliasToken builds a @project[type]:name reference for a queued command, or
// reports ok=false when it can't be referenced safely. The [type] is included
// only when the command name is ambiguous within the project (multi-type).
func (m Model) aliasToken(c CommandInfo) (string, bool) {
	if c.Name == "" || !m.projectReferenceable(c.DisplayName) {
		return "", false
	}
	tok := "@" + c.DisplayName
	if c.Type != "" && m.commandNameAmbiguous(c.DisplayName, c.Name) {
		tok += "[" + c.Type + "]"
	}
	tok += ":" + c.Name
	return tok, true
}

// projectReferenceable reports whether exactly one location carries this display
// name, so `@name:...` resolves unambiguously at load time.
func (m Model) projectReferenceable(displayName string) bool {
	count := 0
	for i := range m.config.Locations {
		if m.config.Locations[i].Name == displayName {
			count++
		}
	}
	return count == 1
}

// commandNameAmbiguous reports whether a project has more than one command with
// the given name (i.e. a multi-type project), so a reference needs a [type].
func (m Model) commandNameAmbiguous(displayName, name string) bool {
	count := 0
	for i := range m.config.Locations {
		loc := &m.config.Locations[i]
		if loc.Name != displayName {
			continue
		}
		for _, cmd := range loc.Commands {
			if cmd.Name == name {
				count++
			}
		}
	}
	return count > 1
}

// queueProject returns the shared project display name when every queued command
// belongs to the same project (same directory and display name), else ok=false.
func (m Model) queueProject() (string, bool) {
	if len(m.queue) == 0 {
		return "", false
	}
	dir := m.queue[0].Directory
	name := m.queue[0].DisplayName
	for _, c := range m.queue[1:] {
		if c.Directory != dir || c.DisplayName != name {
			return "", false
		}
	}
	return name, true
}

func (m Model) renderQueueEditor() string {
	var lines []string
	lines = append(lines, previewTitleStyle.Render("Queue — runs in order"))
	lines = append(lines, statusStyle.Render("Reorder or remove commands before running."))
	lines = append(lines, "")

	for i, c := range m.queue {
		text := fmt.Sprintf("%d. %s", i+1, c.Display)
		if i == m.queueCursor {
			lines = append(lines, "  "+cursorLineStyle.Render(text))
		} else {
			lines = append(lines, "  "+listCommandStyle.Render(text))
		}
	}

	lines = append(lines, "")
	if m.queueSaving {
		lines = append(lines, m.saveInput.View())
		lines = append(lines, m.renderHelp([][2]string{
			{"Enter", "save"},
			{"Esc", "back"},
		}))
	} else {
		if m.queueHint != "" {
			lines = append(lines, statusYellowStyle.Render("  "+m.queueHint))
		}
		lines = append(lines, m.renderHelp([][2]string{
			{"shift+↑/↓", "move"},
			{"x", "remove"},
			{"s", "save"},
			{"Enter", "run"},
			{"Esc", "back"},
		}))
	}

	return strings.Join(lines, "\n")
}
