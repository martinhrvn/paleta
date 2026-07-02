package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/martinhrvn/paleta/internal/config"
)

// queueTestModel returns a model with commands loaded and a queue seeded from the
// given filtered indices, in that order.
func queueTestModel(indices ...int) Model {
	m := createTestModel(createTestConfig())
	for _, i := range indices {
		m.toggleSelection(i)
	}
	return m
}

func TestModel_EnterQueueEditor_NoopWhenEmpty(t *testing.T) {
	m := createTestModel(createTestConfig())
	m.enterQueueEditor()
	if m.queueEditing {
		t.Error("expected queue editor not to open with an empty queue")
	}
}

func TestModel_KeyCtrlQ_OpensEditorWhenQueued(t *testing.T) {
	m := queueTestModel(0, 2)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	um := updated.(Model)
	if !um.queueEditing {
		t.Error("expected Ctrl+Q to open the queue editor")
	}
	if um.queueCursor != 0 {
		t.Errorf("expected editor cursor at 0, got %d", um.queueCursor)
	}
}

func TestModel_QueueEditor_ReorderKeepsCursorOnItem(t *testing.T) {
	// Queue: [npm start (0), npm build (2)]
	m := queueTestModel(0, 2)
	m.enterQueueEditor()

	// Shift+Down on the top item swaps it below and the cursor follows it.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	m = updated.(Model)

	if m.queue[0].Command != "npm build" || m.queue[1].Command != "npm start" {
		t.Errorf("expected order [npm build, npm start], got [%s, %s]", m.queue[0].Command, m.queue[1].Command)
	}
	if m.queueCursor != 1 {
		t.Errorf("expected cursor to follow moved item to 1, got %d", m.queueCursor)
	}
}

func TestModel_QueueEditor_ReorderClampsAtEdges(t *testing.T) {
	m := queueTestModel(0, 2)
	m.enterQueueEditor()

	// Shift+Up at the top edge is a no-op (order unchanged).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftUp})
	m = updated.(Model)
	if m.queue[0].Command != "npm start" || m.queueCursor != 0 {
		t.Errorf("expected no-op at top edge, got cursor %d order [%s, %s]", m.queueCursor, m.queue[0].Command, m.queue[1].Command)
	}
}

func TestModel_QueueEditor_Remove(t *testing.T) {
	// Queue: [npm start (0), npm test (1), npm build (2)]
	m := queueTestModel(0, 1, 2)
	m.enterQueueEditor()

	// Remove the first item with 'x'.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)

	if len(m.queue) != 2 {
		t.Fatalf("expected 2 queued after remove, got %d", len(m.queue))
	}
	if m.queue[0].Command != "npm test" {
		t.Errorf("expected 'npm test' first after removing head, got %q", m.queue[0].Command)
	}
}

func TestModel_QueueEditor_RemoveLastExitsEditor(t *testing.T) {
	m := queueTestModel(0)
	m.enterQueueEditor()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	m = updated.(Model)

	if m.queueEditing {
		t.Error("expected editor to close when the last item is removed")
	}
	if len(m.queue) != 0 {
		t.Errorf("expected empty queue, got %d", len(m.queue))
	}
}

func TestModel_QueueEditor_EnterRunsInQueueOrder(t *testing.T) {
	// Enqueue build before start so queue order differs from list order.
	m := queueTestModel(2, 0)
	m.enterQueueEditor()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if !m.quitting {
		t.Error("expected Enter in the editor to run and quit")
	}
	if len(m.results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(m.results))
	}
	if m.results[0].Command != "npm build" || m.results[1].Command != "npm start" {
		t.Errorf("expected queue order [npm build, npm start], got [%s, %s]", m.results[0].Command, m.results[1].Command)
	}
}

func TestModel_QueueEditor_EscReturnsToNormal(t *testing.T) {
	m := queueTestModel(0, 1)
	m.enterQueueEditor()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)

	if m.queueEditing {
		t.Error("expected Esc to leave the queue editor")
	}
	// Queue is preserved on the way out.
	if len(m.queue) != 2 {
		t.Errorf("expected queue preserved after Esc, got %d", len(m.queue))
	}
}

func TestModel_QueueProject_SameAndCrossProject(t *testing.T) {
	// createTestConfig has two locations: frontend (indices 0-2) and backend (3-4).
	same := queueTestModel(0, 1)
	if name, ok := same.queueProject(); !ok || name != "frontend" {
		t.Errorf("expected same-project 'frontend', got %q ok=%v", name, ok)
	}

	cross := queueTestModel(0, 3)
	if _, ok := cross.queueProject(); ok {
		t.Error("expected cross-project queue to report not savable")
	}
}

func TestModel_QueueSave_CallsStoreWithJoinedCommand(t *testing.T) {
	// frontend commands (same project): npm start (0), npm test (1)
	m := queueTestModel(0, 1)

	var gotDisplay, gotDir, gotName, gotCmd string
	called := false
	m.saveCommand = &SaveStore{Save: func(displayName, directory, name, command string) error {
		called = true
		gotDisplay, gotDir, gotName, gotCmd = displayName, directory, name, command
		return nil
	}}

	m.enterQueueEditor()

	// Press 's' to open the save prompt.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)
	if !m.queueSaving {
		t.Fatalf("expected save prompt to open; hint=%q", m.queueHint)
	}

	// Type a name, then confirm.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("chain")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if !called {
		t.Fatal("expected the save store to be called")
	}
	if gotDisplay != "frontend" {
		t.Errorf("display name = %q, want frontend", gotDisplay)
	}
	if gotDir != "/path/to/frontend" {
		t.Errorf("directory = %q, want /path/to/frontend", gotDir)
	}
	if gotName != "chain" {
		t.Errorf("name = %q, want chain", gotName)
	}
	if gotCmd != "npm start && npm test" {
		t.Errorf("command = %q, want 'npm start && npm test'", gotCmd)
	}
}

func TestModel_QueueSave_CrossProjectShowsHint(t *testing.T) {
	// frontend (0) + backend (3) span two projects.
	m := queueTestModel(0, 3)
	m.saveCommand = &SaveStore{Save: func(_, _, _, _ string) error { return nil }}
	m.enterQueueEditor()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)

	if m.queueSaving {
		t.Error("expected save to be blocked for a cross-project queue")
	}
	if m.queueHint == "" {
		t.Error("expected a hint explaining the one-project constraint")
	}
}

func TestModel_QueueSave_UnavailableShowsHint(t *testing.T) {
	m := queueTestModel(0, 1) // same project, but no save store wired
	m.enterQueueEditor()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)

	if m.queueSaving {
		t.Error("expected save to be unavailable without a save store")
	}
	if m.queueHint == "" {
		t.Error("expected a hint that saving is unavailable")
	}
}

// Ensure the seeded config shape assumptions hold for these tests.
var _ = config.Command{}
