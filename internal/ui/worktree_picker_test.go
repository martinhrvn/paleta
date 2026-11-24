package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func worktreeTestModel() Model {
	m := createTestModel(createTestConfig())
	m.width, m.height = 100, 30
	m.backend.Worktrees = []WorktreeEntry{
		{Path: "/repo", Label: "main"},
		{Path: "/repo-feature", Label: "feature", Current: true},
	}
	m.backend.WorktreeLabel = "feature"
	return m
}

func press(t *testing.T, m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	return updated.(Model), cmd
}

// Without worktrees to switch to, Ctrl+W does nothing: the search keeps its
// text (the key is not forwarded as delete-word either) and the help line does
// not advertise it.
func TestWorktreePicker_HiddenWithoutWorktrees(t *testing.T) {
	m := createTestModel(createTestConfig())
	m.width, m.height = 100, 30
	m, _ = press(t, m, runes("np"))
	m, _ = press(t, m, key(tea.KeyCtrlW))
	if m.mode != modeNormal {
		t.Fatalf("mode = %v, want normal", m.mode)
	}
	if got := m.searchInput.Value(); got != "np" {
		t.Errorf("search = %q, want it untouched by Ctrl+W", got)
	}
	if view := ansi.Strip(m.View()); strings.Contains(view, "^W") {
		t.Errorf("help line advertises ^W without worktrees:\n%s", view)
	}
}

// With worktrees, Ctrl+W opens the picker on the current checkout, the help line
// says so, and the status line names the checkout the rows belong to.
func TestWorktreePicker_OpensOnCurrent(t *testing.T) {
	m := worktreeTestModel()
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "^W") {
		t.Errorf("help line should advertise ^W:\n%s", view)
	}
	if !strings.Contains(view, "feature") {
		t.Errorf("status line should name the current worktree:\n%s", view)
	}

	m, _ = press(t, m, key(tea.KeyCtrlW))
	if m.mode != modeWorktreePick {
		t.Fatalf("mode = %v, want worktree-pick", m.mode)
	}
	if m.searchInput.Focused() {
		t.Error("search box should be blurred while picking a worktree")
	}
	if m.worktreeCursor != 1 {
		t.Errorf("cursor = %d, want it on the current worktree (1)", m.worktreeCursor)
	}
	picker := ansi.Strip(m.View())
	for _, want := range []string{"Switch worktree", "main", "/repo", "feature", "/repo-feature"} {
		if !strings.Contains(picker, want) {
			t.Errorf("picker view missing %q:\n%s", want, picker)
		}
	}

	m, _ = press(t, m, key(tea.KeyEscape))
	if m.mode != modeNormal || !m.searchInput.Focused() {
		t.Errorf("after Esc: mode = %v focused = %v, want normal and focused", m.mode, m.searchInput.Focused())
	}
	if m.worktree != "" {
		t.Errorf("worktree = %q after cancel, want none", m.worktree)
	}
}

// Choosing another checkout ends the program with that path, for the caller to
// re-enter the selector re-rooted there.
func TestWorktreePicker_EnterSwitches(t *testing.T) {
	m := worktreeTestModel()
	m, _ = press(t, m, key(tea.KeyCtrlW))
	m, _ = press(t, m, key(tea.KeyUp))
	if m.worktreeCursor != 0 {
		t.Fatalf("cursor = %d, want 0", m.worktreeCursor)
	}
	m, cmd := press(t, m, key(tea.KeyEnter))
	if m.worktree != "/repo" {
		t.Errorf("worktree = %q, want /repo", m.worktree)
	}
	if !m.quitting || cmd == nil {
		t.Error("choosing a worktree should quit the program")
	}
}

// Choosing the checkout already shown is a no-op: back to the palette.
func TestWorktreePicker_EnterOnCurrentJustLeaves(t *testing.T) {
	m := worktreeTestModel()
	m, _ = press(t, m, key(tea.KeyCtrlW))
	m, _ = press(t, m, key(tea.KeyEnter))
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want normal", m.mode)
	}
	if m.worktree != "" || m.quitting {
		t.Errorf("worktree = %q quitting = %v, want no switch", m.worktree, m.quitting)
	}
}

// `c` changes the shell's directory to the checkout under the cursor instead of
// re-rooting the palette: the program ends with a single "cd" result for the
// wrapper to act on.
func TestWorktreePicker_CdSwitchesShell(t *testing.T) {
	m := worktreeTestModel()
	m, _ = press(t, m, key(tea.KeyCtrlW))
	if view := ansi.Strip(m.View()); !strings.Contains(view, "cd there") {
		t.Errorf("picker help should advertise c:\n%s", view)
	}
	m, _ = press(t, m, key(tea.KeyUp))
	m, cmd := press(t, m, runes("c"))
	if !m.quitting || cmd == nil {
		t.Fatal("c should quit the program")
	}
	if m.worktree != "" {
		t.Errorf("worktree = %q, want no palette re-root", m.worktree)
	}
	if len(m.results) != 1 || m.results[0].Action != "cd" || m.results[0].Directory != "/repo" || m.results[0].Command != "" {
		t.Errorf("results = %+v, want one cd to /repo", m.results)
	}
}

// `c` on the checkout you are already in has nowhere to go: back to the palette.
func TestWorktreePicker_CdOnCurrentJustLeaves(t *testing.T) {
	m := worktreeTestModel()
	m, _ = press(t, m, key(tea.KeyCtrlW))
	m, _ = press(t, m, runes("c"))
	if m.mode != modeNormal || m.quitting || len(m.results) != 0 {
		t.Errorf("mode = %v quitting = %v results = %+v, want back in the palette", m.mode, m.quitting, m.results)
	}
}
