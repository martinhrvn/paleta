package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// enterQueueEditor switches into the queue editor. It is a no-op when the queue
// is empty (nothing to edit).
func (m *Model) enterQueueEditor() {
	if len(m.queue) == 0 {
		return
	}
	m.mode = modeQueueEdit
	m.queueCursor = 0
	m.queueHint = ""
	m.searchInput.Blur()
}

func (m *Model) exitQueueEditor() {
	m.queueHint = ""
	m.leaveMode()
}

func (m Model) updateQueueEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	m.queueCursor = moveCursor(m.queueCursor, delta, len(m.queue))
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
		return m, m.confirmQueueSave()
	case tea.KeyEscape:
		m.cancelQueueSave()
		return m, nil
	}
	var cmd tea.Cmd
	m.saveInput, cmd = m.saveInput.Update(msg)
	return m, cmd
}

// startQueueSave enters the save prompt, provided saving is available. A queue
// spanning folders is allowed: each part is referenced by alias (which cd-wraps
// cross-project commands) or, when it can't be, cd-wrapped raw.
func (m *Model) startQueueSave() {
	if m.backend.SaveQueue == nil {
		m.queueHint = "saving to .pltrc is unavailable here"
		return
	}
	m.mode = modeQueueSave
	m.queueHint = ""
	m.saveInput.SetValue("")
	m.saveInput.Focus()
}

func (m *Model) cancelQueueSave() {
	m.mode = modeQueueEdit
	m.saveInput.Blur()
}

// confirmQueueSave joins the queued commands and persists them under the save
// target project, then reloads so the new command appears. If the saved chain
// won't re-expand cleanly it still saves, but the editor stays open with a
// warning rather than blocking. The returned command resumes any background
// type resolution the reload deferred.
func (m *Model) confirmQueueSave() tea.Cmd {
	name := strings.TrimSpace(m.saveInput.Value())
	if name == "" {
		m.queueHint = "enter a name for the command"
		return nil
	}
	displayName, directory := m.saveTarget()
	parts := m.buildQueueParts(directory)
	joined := strings.Join(parts, " && ")

	if err := m.backend.SaveQueue(displayName, directory, name, parts); err != nil {
		m.queueHint = "save failed: " + err.Error()
		m.cancelQueueSave()
		return nil
	}

	// Reflect the new command in the list immediately.
	cmd := m.reloadConfig()
	m.loadCommands()
	m.updateFilteredCommands()

	// Warn (without blocking) if the saved chain won't run cleanly, e.g. a raw
	// fallback is ambiguous or a referenced command has since changed.
	if err := m.config.ExpandCheck(joined, directory); err != nil {
		m.cancelQueueSave()
		m.queueHint = "saved, but may not run: " + err.Error()
		return cmd
	}
	m.exitQueueEditor()
	return cmd
}

// saveTarget picks where a saved queue is stored and the directory used as the
// expansion "home". A queue confined to one project saves there; a cross-folder
// queue saves under the root location so its per-project parts cd-wrap. It falls
// back to the first queued command's project when no root is known.
func (m Model) saveTarget() (displayName, directory string) {
	if name, ok := m.queueProject(); ok {
		return name, m.queue[0].Directory
	}
	if m.backend.RootDir != "" {
		for i := range m.config.Locations {
			loc := &m.config.Locations[i]
			if loc.Location != m.backend.RootDir {
				continue
			}
			return loc.DisplayName(), loc.Location
		}
	}
	return m.queue[0].DisplayName, m.queue[0].Directory
}

// buildQueueParts returns the queued commands as saveable segments, preferring
// reference tokens (@project[type]:name) over raw command strings so the saved
// command tracks the referenced commands rather than freezing their current
// text. A command that can't be referenced falls back to its raw string,
// cd-wrapped in its own directory when that differs from homeDir so it still runs
// in the right place (aliased cross-project commands cd-wrap automatically on
// expansion). The caller joins the parts with " && " for execution or stores
// them as a list.
func (m Model) buildQueueParts(homeDir string) []string {
	parts := make([]string, len(m.queue))
	for i, c := range m.queue {
		if tok, ok := m.aliasToken(c); ok {
			parts[i] = tok
			continue
		}
		raw := c.Command
		if c.Directory != homeDir {
			raw = "(cd '" + c.Directory + "' && " + raw + ")"
		}
		parts[i] = raw
	}
	return parts
}

// aliasToken builds a verified @project[type]:name reference for a queued
// command, or reports ok=false when it can't be referenced safely. It delegates
// to the config resolver so the token is guaranteed to expand back to this exact
// command (or the caller falls back to the raw string).
func (m Model) aliasToken(c CommandInfo) (string, bool) {
	if m.config == nil {
		return "", false
	}
	return m.config.ReferenceToken(c.Directory, c.Name, c.Type)
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
	if m.mode == modeQueueSave {
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
