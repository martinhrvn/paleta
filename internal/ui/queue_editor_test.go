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

// saveQueueAndCapture queues the given filtered indices, opens the editor, saves
// under the name "chain", and returns the command string handed to the store.
func saveQueueAndCapture(t *testing.T, cfg *config.Config, indices ...int) string {
	t.Helper()
	m := createTestModel(cfg)
	for _, i := range indices {
		m.toggleSelection(i)
	}
	var got string
	m.saveCommand = &SaveStore{Save: func(_, _, _, command string) error {
		got = command
		return nil
	}}
	m.enterQueueEditor()

	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("s")},
		{Type: tea.KeyRunes, Runes: []rune("chain")},
		{Type: tea.KeyEnter},
	} {
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}
	return got
}

func TestModel_QueueSave_EmitsAliasesForNamedCommands(t *testing.T) {
	cfg := &config.Config{Locations: []config.Location{{
		Name: "web", Location: "/path/web", Types: config.Types{"pnpm"},
		Commands: []config.Command{
			{Name: "build", Command: "pnpm run build", Type: "pnpm"},
			{Name: "dev", Command: "pnpm run dev", Type: "pnpm"},
		},
	}}}

	if got, want := saveQueueAndCapture(t, cfg, 0, 1), "@web:build && @web:dev"; got != want {
		t.Errorf("saved %q, want %q", got, want)
	}
}

func TestModel_QueueSave_IncludesTypeWhenAmbiguous(t *testing.T) {
	cfg := &config.Config{Locations: []config.Location{{
		Name: "svc", Location: "/path/svc", Types: config.Types{"npm", "docker"},
		Commands: []config.Command{
			{Name: "build", Command: "npm run build", Type: "npm"},
			{Name: "build", Command: "docker build .", Type: "docker"},
		},
	}}}

	// Queue the docker build (index 1); its name collides with the npm build, so
	// the reference must carry the [docker] type.
	if got, want := saveQueueAndCapture(t, cfg, 1), "@svc[docker]:build"; got != want {
		t.Errorf("saved %q, want %q", got, want)
	}
}

func TestModel_QueueSave_FallsBackToRawForUnnamedCommand(t *testing.T) {
	cfg := &config.Config{Locations: []config.Location{{
		Name: "web", Location: "/path/web",
		Commands: []config.Command{
			{Name: "", Command: "make thing"}, // no name -> can't be referenced
		},
	}}}

	if got, want := saveQueueAndCapture(t, cfg, 0), "make thing"; got != want {
		t.Errorf("saved %q, want %q", got, want)
	}
}

func TestModel_QueueSave_CrossFolderSavesToRoot(t *testing.T) {
	// A queue spanning the root project and a sub-project now saves (to the root
	// location); each part is referenced by alias, which cd-wraps cross-project.
	cfg := &config.Config{Locations: []config.Location{
		{
			Name: "root", Location: "/repo", Types: config.Types{"go"},
			Commands: []config.Command{{Name: "build", Command: "go build ./...", Type: "go"}},
		},
		{
			Name: "web", Location: "/repo/web", Types: config.Types{"pnpm"},
			Commands: []config.Command{{Name: "dev", Command: "pnpm dev", Type: "pnpm"}},
		},
	}}
	m := createTestModel(cfg)
	m.toggleSelection(0) // root: build
	m.toggleSelection(1) // web: dev

	var gotDisplay, gotDir, gotCmd string
	m.saveCommand = &SaveStore{
		RootDir: "/repo",
		Save: func(displayName, directory, _, command string) error {
			gotDisplay, gotDir, gotCmd = displayName, directory, command
			return nil
		},
	}
	m.enterQueueEditor()

	// 's' opens the save prompt even though the queue spans folders.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)
	if !m.queueSaving {
		t.Fatalf("expected save prompt to open for a cross-folder queue; hint=%q", m.queueHint)
	}
	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("all")},
		{Type: tea.KeyEnter},
	} {
		updated, _ = m.Update(msg)
		m = updated.(Model)
	}

	if gotDisplay != "root" || gotDir != "/repo" {
		t.Errorf("save target = %q/%q, want root//repo", gotDisplay, gotDir)
	}
	if want := "@root:build && @web:dev"; gotCmd != want {
		t.Errorf("joined = %q, want %q", gotCmd, want)
	}
}

func TestModel_QueueSave_CdWrapsNonAliasableCrossFolder(t *testing.T) {
	// A cross-folder command with a space in its name can't be aliased, so it is
	// cd-wrapped raw to still run in its own directory.
	cfg := &config.Config{Locations: []config.Location{
		{
			Name: "root", Location: "/repo",
			Commands: []config.Command{{Name: "build", Command: "make"}},
		},
		{
			Name: "web", Location: "/repo/web",
			Commands: []config.Command{{Name: "dev server", Command: "pnpm dev"}},
		},
	}}
	m := createTestModel(cfg)
	m.toggleSelection(0) // root: build (unnamed-ref? has name "build")
	m.toggleSelection(1) // web: "dev server" (space -> not aliasable)

	var gotCmd string
	m.saveCommand = &SaveStore{
		RootDir: "/repo",
		Save:    func(_, _, _, command string) error { gotCmd = command; return nil },
	}
	m.enterQueueEditor()
	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("s")},
		{Type: tea.KeyRunes, Runes: []rune("all")},
		{Type: tea.KeyEnter},
	} {
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}

	if want := "@root:build && (cd '/repo/web' && pnpm dev)"; gotCmd != want {
		t.Errorf("joined = %q, want %q", gotCmd, want)
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
