package ui

import (
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
	m := NewModel(cfg, nil)

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
	store := &FocusStore{
		List: func() ([]FocusEntry, error) {
			return []FocusEntry{
				{Key: "frontend", Label: "frontend", Focused: true},
				{Key: "backend", Label: "backend", Focused: false},
			}, nil
		},
		Save: func(focused map[string]bool) error {
			saved = focused
			return nil
		},
	}

	m := NewModel(cfg, store)
	m.loadCommands()

	// Ctrl+P opens the picker populated from the store.
	updated, _ := m.updateNormalMode(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = updated.(Model)
	if !m.focusPicking {
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
	if m.focusPicking {
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
	m := NewModel(focusTestConfig(), nil)
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
	m := NewModel(focusTestConfig(), nil)
	updated, _ := m.updateNormalMode(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = updated.(Model)
	if !m.reinit {
		t.Error("expected reinit to be set after Ctrl+N")
	}
	if !m.quitting {
		t.Error("expected quitting to be set after Ctrl+N")
	}
}
