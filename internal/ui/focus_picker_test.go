package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhrvn/paleta/internal/config"
)

func focusTestConfig() *config.Config {
	return &config.Config{
		Locations: []config.Location{
			{
				Name:     "frontend",
				Location: "/path/to/frontend",
				Focused:  true,
				Commands: stringsToCommands([]string{"npm start", "npm test"}),
			},
			{
				Name:     "backend",
				Location: "/path/to/backend",
				Commands: stringsToCommands([]string{"go run main.go"}),
			},
		},
	}
}

func TestModel_FocusFilter(t *testing.T) {
	cfg := focusTestConfig()
	m := NewModel(cfg, Backend{})

	// With a focused location present, the selector defaults to focused-only.
	if !m.focusActive {
		t.Fatal("expected focusActive to default true when a location is focused")
	}
	m.loadCommands()
	if len(m.commands) != 2 {
		t.Fatalf("expected only frontend's 2 commands when focused, got %d", len(m.commands))
	}
	for _, c := range m.commands {
		if c.DisplayName != "frontend" {
			t.Errorf("expected only frontend commands, got %q", c.DisplayName)
		}
	}

	// Toggling focus off shows everything (3 commands total).
	m.focusActive = false
	m.loadCommands()
	if len(m.commands) != 3 {
		t.Fatalf("expected all 3 commands with focus off, got %d", len(m.commands))
	}
}

func TestModel_FocusPickerSaves(t *testing.T) {
	cfg := focusTestConfig()

	var saved map[string]bool
	store := Backend{
		ListFocus: func() ([]FocusEntry, error) {
			return []FocusEntry{
				{Key: "frontend", Label: "frontend", Focused: true},
				{Key: "backend", Label: "backend", Focused: false},
			}, nil
		},
		SaveFocus: func(focused map[string]bool) error {
			saved = focused
			return nil
		},
	}

	m := NewModel(cfg, store)
	m.loadCommands()

	// Ctrl+P opens the picker populated from the store.
	updated, _ := m.updateNormalMode(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = updated.(Model)
	if m.mode != modeFocusPick {
		t.Fatal("expected focusPicking to be true after Ctrl+P")
	}
	if len(m.focusItems) != 2 {
		t.Fatalf("expected 2 focus items, got %d", len(m.focusItems))
	}

	// Move to backend and toggle it on with space.
	updated, _ = m.updateFocusPickMode(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, _ = m.updateFocusPickMode(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)

	// Enter confirms and persists via the store.
	updated, _ = m.updateFocusPickMode(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.mode == modeFocusPick {
		t.Error("expected picker to close after Enter")
	}
	if saved == nil {
		t.Fatal("expected Save to be called on confirm")
	}
	if !saved["frontend"] || !saved["backend"] {
		t.Errorf("expected both projects focused after toggle, got %+v", saved)
	}
}

func TestModel_FocusPickerToggleAll(t *testing.T) {
	m := NewModel(focusTestConfig(), Backend{})
	m.focusItems = []FocusEntry{
		{Key: "a", Focused: false},
		{Key: "b", Focused: false},
	}

	// Ctrl+A focuses all when not all focused.
	m.toggleFocusAll()
	if !m.focusItems[0].Focused || !m.focusItems[1].Focused {
		t.Error("expected all items focused after first toggle-all")
	}
	// Ctrl+A again unfocuses all.
	m.toggleFocusAll()
	if m.focusItems[0].Focused || m.focusItems[1].Focused {
		t.Error("expected all items unfocused after second toggle-all")
	}
}

func TestModel_ReinitOnCtrlN(t *testing.T) {
	m := NewModel(focusTestConfig(), Backend{})
	updated, _ := m.updateNormalMode(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = updated.(Model)
	if !m.reinit {
		t.Error("expected reinit to be set after Ctrl+N")
	}
	if !m.quitting {
		t.Error("expected quitting to be set after Ctrl+N")
	}
}

// A focus save reloads the config from disk. The reload must be as complete as
// the initial load: tool rows stay in the list and the config keeps its path.
func TestModel_FocusPickerSaveKeepsToolRows(t *testing.T) {
	dir := t.TempDir()
	pltrc := filepath.Join(dir, config.ConfigFileName)
	src := "locations:\n  - name: app\n    location: .\n    commands: [\"echo hi\"]\ntools:\n  - lazygit\n"
	if err := os.WriteFile(pltrc, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(config.LoadOptions{Workdir: dir, Defer: true})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	store := Backend{
		Reload: cfg.Reload,
		ListFocus: func() ([]FocusEntry, error) {
			return []FocusEntry{{Key: "app", Label: "app"}}, nil
		},
		SaveFocus: func(map[string]bool) error {
			return os.WriteFile(pltrc, []byte("focused: [app]\n"+src), 0644)
		},
	}
	hasToolRow := func(rows []CommandInfo) bool {
		for _, r := range rows {
			if r.IsTool {
				return true
			}
		}
		return false
	}

	m := NewModel(cfg, store)
	m.loadCommands()
	if !hasToolRow(m.commands) {
		t.Fatal("precondition: expected a tool row before the save")
	}

	updated, _ := m.updateNormalMode(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = updated.(Model)
	updated, _ = m.updateFocusPickMode(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if !hasToolRow(m.commands) {
		t.Error("tool rows disappeared after the focus save reloaded the config")
	}
	if m.config.Path != pltrc {
		t.Errorf("config path after reload = %q, want %q", m.config.Path, pltrc)
	}
	if !m.config.AnyFocused() {
		t.Error("expected the reloaded config to carry the saved focus")
	}
}
