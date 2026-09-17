package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/history"
)

func key(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// The selector is in exactly one mode at a time, and every sub-mode returns to
// the palette on Esc with the search box focused again.
func TestModel_ModeTransitions(t *testing.T) {
	m := queueTestModel(0, 1)
	m.backend.SaveQueue = func(string, string, string, []string) error { return nil }
	m.backend.ListFocus = func() ([]FocusEntry, error) {
		return []FocusEntry{{Key: "frontend", Label: "frontend"}}, nil
	}
	if m.mode != modeNormal {
		t.Fatalf("initial mode = %v, want normal", m.mode)
	}

	step := func(msg tea.KeyMsg, want selectorMode) {
		t.Helper()
		updated, _ := m.Update(msg)
		m = updated.(Model)
		if m.mode != want {
			t.Fatalf("after %v: mode = %v, want %v", msg, m.mode, want)
		}
	}

	step(key(tea.KeyCtrlQ), modeQueueEdit)
	step(runes("s"), modeQueueSave)
	step(key(tea.KeyEscape), modeQueueEdit)
	step(key(tea.KeyEscape), modeNormal)
	if !m.searchInput.Focused() {
		t.Error("search box should regain focus after leaving the queue editor")
	}

	step(key(tea.KeyCtrlE), modeEdit)
	step(key(tea.KeyEscape), modeNormal)

	step(key(tea.KeyCtrlP), modeFocusPick)
	if m.searchInput.Focused() {
		t.Error("search box should be blurred while picking focus")
	}
	step(key(tea.KeyEscape), modeNormal)
	if !m.searchInput.Focused() {
		t.Error("search box should regain focus after the focus picker")
	}
}

// The viewport and the renderer share one row count, so with the warning
// banner taking a line the cursor never scrolls onto a row that isn't drawn.
func TestModel_ViewportAccountsForBanner(t *testing.T) {
	cfg := createTestConfig()
	cfg.Warnings = []config.Warning{{Kind: "name", Scope: "command", Context: "x", Reason: "contains a space"}}
	m := createTestModel(cfg)
	m.width, m.height = 80, 7 // search + status + banner + help leave 3 rows

	rows := m.listHeight()
	if rows != 3 {
		t.Fatalf("listHeight = %d, want 3 with the banner shown", rows)
	}
	for i := 0; i < 3; i++ {
		m.moveCursorDown()
	}
	if m.currentIndex < m.viewportOffset || m.currentIndex >= m.viewportOffset+rows {
		t.Fatalf("cursor %d outside the drawn window [%d, %d)", m.currentIndex, m.viewportOffset, m.viewportOffset+rows)
	}
	drawn := ansi.Strip(m.renderCommandList(60, rows))
	if want := m.filteredCommands[m.currentIndex].Command; !strings.Contains(drawn, want) {
		t.Errorf("cursor row %q not drawn:\n%s", want, drawn)
	}
}

// A location resolving in the background must not yank the cursor away from
// the row the user is on.
func TestModel_PendingResolvedKeepsCursor(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{
			{Name: "infra", Location: "/repo/infra", Types: config.Types{"make"}, PendingTypes: []string{"make"}},
			{Name: "web", Location: "/repo/web", Commands: []config.Command{{Name: "build", Command: "npm run build"}}},
		},
	}
	m := NewModel(cfg, Backend{})
	m.loadCommands()
	m.updateFilteredCommands()
	m.moveCursorDown() // onto "web: build"
	if m.filteredCommands[m.currentIndex].Display != "web: build" {
		t.Fatalf("precondition: cursor on %q", m.filteredCommands[m.currentIndex].Display)
	}

	updated, _ := m.Update(pendingResolvedMsg{index: 0, commands: []config.Command{
		{Name: "plan", Command: "make plan"}, {Name: "apply", Command: "make apply"},
	}})
	m = updated.(Model)
	if got := m.filteredCommands[m.currentIndex].Display; got != "web: build" {
		t.Errorf("cursor moved to %q after resolution, want it to stay on web: build", got)
	}
}

// Frecency orders the visible list without reordering the loaded rows, so the
// base order is always there to fall back to.
func TestModel_FilterKeepsBaseOrder(t *testing.T) {
	m := createTestModel(createTestConfig())
	hist, err := history.NewHistory(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := hist.RecordExecution("backend", "go test ./..."); err != nil {
			t.Fatal(err)
		}
	}
	m.history = hist
	m.frecencyEnabled = true
	m.updateFilteredCommands()

	if got := m.filteredCommands[0].Command; got != "go test ./..." {
		t.Errorf("visible list starts with %q, want the most-used command", got)
	}
	if got := m.commands[0].Command; got != "npm start" {
		t.Errorf("base list starts with %q, want the loaded order untouched", got)
	}
}

// The cursor helpers shared by every list clamp and scroll consistently.
func TestListHelpers(t *testing.T) {
	if got := moveCursor(0, -1, 5); got != 0 {
		t.Errorf("moveCursor up at top = %d, want 0", got)
	}
	if got := moveCursor(4, 1, 5); got != 4 {
		t.Errorf("moveCursor down at bottom = %d, want 4", got)
	}
	if got := moveCursor(3, 1, 0); got != 0 {
		t.Errorf("moveCursor on an empty list = %d, want 0", got)
	}
	if got := scrollToCursor(7, 0, 5); got != 3 {
		t.Errorf("scrollToCursor below window = %d, want 3", got)
	}
	if got := scrollToCursor(1, 3, 5); got != 1 {
		t.Errorf("scrollToCursor above window = %d, want 1", got)
	}
	if start, end := visibleWindow(3, 5, 6); start != 3 || end != 6 {
		t.Errorf("visibleWindow = [%d, %d), want [3, 6)", start, end)
	}
}
