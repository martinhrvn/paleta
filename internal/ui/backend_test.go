package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/history"
)

// The selector takes its history from the backend and never loads one itself.
func TestNewModel_UsesBackendHistory(t *testing.T) {
	h, err := history.NewHistory(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if m := NewModel(createTestConfig(), Backend{History: h}); m.history != h {
		t.Error("expected the backend's history to be used")
	}
	if m := NewModel(createTestConfig(), Backend{}); m.history != nil {
		t.Error("expected no history when the backend provides none")
	}
}

// A save inside the selector re-reads the configuration through the backend.
func TestModel_ReloadGoesThroughBackend(t *testing.T) {
	reloaded := createTestConfig()
	reloaded.Locations = reloaded.Locations[:1]
	calls := 0
	m := NewModel(createTestConfig(), Backend{
		ListFocus: func() ([]FocusEntry, error) { return []FocusEntry{{Key: "frontend"}}, nil },
		SaveFocus: func(map[string]bool) error { return nil },
		Reload:    func() (*config.Config, error) { calls++; return reloaded, nil },
	})
	m.loadCommands()

	updated, _ := m.updateNormalMode(key(tea.KeyCtrlP))
	m = updated.(Model)
	updated, _ = m.updateFocusPickMode(key(tea.KeyEnter))
	m = updated.(Model)

	if calls != 1 {
		t.Errorf("Reload called %d times, want 1", calls)
	}
	if m.config != reloaded || len(m.commands) != 3 {
		t.Errorf("selector still shows %d rows of the old config, want the reloaded one", len(m.commands))
	}
}

// Deferred project types are resolved through the backend; without a resolver
// nothing is scheduled and the placeholder rows simply stay.
func TestModel_ResolvesPendingThroughBackend(t *testing.T) {
	m := NewModel(pendingTestConfig(), Backend{
		ResolvePending: func(loc config.Location) ([]config.Command, []config.Warning) {
			return []config.Command{{Name: "deploy", Command: "make deploy"}}, nil
		},
	})
	m.loadCommands()
	cmds := m.resolvePendingCmds()
	if len(cmds) != 1 {
		t.Fatalf("scheduled %d resolutions, want 1", len(cmds))
	}
	msg, ok := cmds[0]().(pendingResolvedMsg)
	if !ok || msg.index != 1 || len(msg.commands) != 1 || msg.commands[0].Name != "deploy" {
		t.Errorf("resolution message = %+v", msg)
	}

	if n := len(NewModel(pendingTestConfig(), Backend{}).resolvePendingCmds()); n != 0 {
		t.Errorf("scheduled %d resolutions with no resolver, want 0", n)
	}
}
