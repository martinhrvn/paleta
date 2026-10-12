package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/scratch"
	"github.com/martinhrvn/paleta/internal/shelllog"
)

var shellNow = time.Date(2026, 10, 12, 15, 0, 0, 0, time.Local)

// shellTestModel is a palette over a project at /p with a shell log of three
// commands: one 5 minutes ago in /p/infra, one 2 hours ago at the root, and
// one 10 days ago.
func shellTestModel() Model {
	cfg := &config.Config{Locations: []config.Location{
		{Name: "root", Location: "/p", Commands: stringsToCommands([]string{"make all"})},
		{Name: "infra", Location: "/p/infra", Commands: stringsToCommands([]string{"pnpm i"})},
	}}
	m := NewModel(cfg, Backend{
		RootDir: "/p",
		ShellHistory: []shelllog.Record{
			{Time: shellNow.AddDate(0, 0, -10), Dir: "/p", Command: "git log --oneline"},
			{Time: shellNow.Add(-2 * time.Hour), Dir: "/p", Command: "go test ./..."},
			{Time: shellNow.Add(-5 * time.Minute), Dir: "/p/infra", Command: "pnpm cdk diff FooStack"},
		},
	})
	m.now = func() time.Time { return shellNow }
	m.width, m.height = 120, 30
	m.loadCommands()
	m.updateFilteredCommands()
	return m
}

func shellView(m Model) string { return ansi.Strip(m.View()) }

func TestShellPicker_HelpLineAdvertisesCtrlR(t *testing.T) {
	m := shellTestModel()
	if !strings.Contains(shellView(m), "^R") {
		t.Errorf("help line should advertise ^R:\n%s", shellView(m))
	}
}

func TestShellPicker_ListsNewestFirstWithRelDirAndAge(t *testing.T) {
	m := shellTestModel()
	m, _ = press(t, m, key(tea.KeyCtrlR))
	if m.mode != modeShellPick {
		t.Fatalf("mode = %v, want shell-pick", m.mode)
	}
	if m.searchInput.Focused() {
		t.Error("search box should be blurred in the picker")
	}
	view := shellView(m)
	for _, want := range []string{"Recent shell commands", "last 7 days", "infra", "5m ago", "2h ago"} {
		if !strings.Contains(view, want) {
			t.Errorf("picker missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "git log") {
		t.Errorf("a 10-day-old command should be outside the default 7-day period:\n%s", view)
	}
	if strings.Index(view, "pnpm cdk diff") > strings.Index(view, "go test") {
		t.Errorf("newest command should be listed first:\n%s", view)
	}
}

func TestShellPicker_PeriodCycles(t *testing.T) {
	saved := []string{}
	m := shellTestModel()
	m.backend.SaveShellView = func(p string, byFolder bool) error {
		saved = append(saved, fmt.Sprintf("%s:%v", p, byFolder))
		return nil
	}
	m, _ = press(t, m, key(tea.KeyCtrlR))
	m, _ = press(t, m, key(tea.KeyDown)) // cursor on the second row

	// Left narrows: 7d -> today -> last hour.
	m, _ = press(t, m, key(tea.KeyLeft))
	m, _ = press(t, m, key(tea.KeyLeft))
	view := shellView(m)
	if !strings.Contains(view, "last hour") || strings.Contains(view, "go test") || !strings.Contains(view, "pnpm cdk diff") {
		t.Errorf("last hour should list only the 5-minute-old command:\n%s", view)
	}
	if m.shellCursor != 0 {
		t.Errorf("cursor = %d, want it clamped to the shorter list", m.shellCursor)
	}

	// Right widens, wrapping from last hour... and Tab cycles the same way.
	m, _ = press(t, m, key(tea.KeyLeft)) // wraps to all time
	if view := shellView(m); !strings.Contains(view, "all time") || !strings.Contains(view, "git log") {
		t.Errorf("all time should include the old command:\n%s", view)
	}
	m, _ = press(t, m, key(tea.KeyRight))
	if !strings.Contains(shellView(m), "last hour") {
		t.Errorf("Right from all time should wrap to last hour:\n%s", shellView(m))
	}
	m, _ = press(t, m, key(tea.KeyTab))
	if !strings.Contains(shellView(m), "today") {
		t.Errorf("Tab should widen like Right:\n%s", shellView(m))
	}
	want := []string{"today:false", "1h:false", "all:false", "1h:false", "today:false"}
	if strings.Join(saved, ",") != strings.Join(want, ",") {
		t.Errorf("saved periods = %v, want %v", saved, want)
	}
}

func TestShellPicker_StartsOnSavedPeriod(t *testing.T) {
	m := shellTestModel()
	m.backend.ShellPeriod = "all"
	m, _ = press(t, m, key(tea.KeyCtrlR))
	if view := shellView(m); !strings.Contains(view, "all time") || !strings.Contains(view, "git log") {
		t.Errorf("picker should open on the saved period:\n%s", view)
	}
}

func TestShellPicker_EmptyShowsHint(t *testing.T) {
	m := shellTestModel()
	m.backend.ShellHistory = nil
	m, _ = press(t, m, key(tea.KeyCtrlR))
	if view := shellView(m); !strings.Contains(view, "plt-integration.zsh") {
		t.Errorf("empty picker should explain how commands get here:\n%s", view)
	}
	m, _ = press(t, m, key(tea.KeyEnter))
	if m.quitting || len(m.results) != 0 {
		t.Error("Enter on an empty picker should do nothing")
	}
	m, _ = press(t, m, key(tea.KeyEscape))
	if m.mode != modeNormal {
		t.Errorf("Esc should return to the palette, mode = %v", m.mode)
	}
}

func TestShellPicker_EnterRunsInRecordedDir(t *testing.T) {
	m := shellTestModel()
	m, _ = press(t, m, key(tea.KeyCtrlR))
	m, cmd := press(t, m, key(tea.KeyEnter))
	if cmd == nil || !m.quitting {
		t.Fatal("Enter should quit with a selection")
	}
	want := SelectionResult{Directory: "/p/infra", Command: "pnpm cdk diff FooStack", Action: "shell"}
	if len(m.results) != 1 || m.results[0].Directory != want.Directory || m.results[0].Command != want.Command ||
		m.results[0].Action != want.Action || m.results[0].DisplayName != "" {
		t.Fatalf("results = %+v, want [%+v]", m.results, want)
	}
}

func TestShellPicker_PinAndUnpin(t *testing.T) {
	var pinned, unpinned []string
	m := shellTestModel()
	m.backend.PinShell = func(dir, cmd string) error { pinned = append(pinned, dir+"|"+cmd); return nil }
	m.backend.UnpinShell = func(dir, cmd string) error { unpinned = append(unpinned, dir+"|"+cmd); return nil }
	m, _ = press(t, m, key(tea.KeyCtrlR))
	m, _ = press(t, m, key(tea.KeyDown)) // "go test ./..." at /p
	m, _ = press(t, m, runes("p"))

	if len(pinned) != 1 || pinned[0] != "/p|go test ./..." {
		t.Fatalf("PinShell calls = %v", pinned)
	}
	view := shellView(m)
	if !strings.Contains(view, "Pinned") || strings.Index(view, "go test") > strings.Index(view, "pnpm cdk diff") {
		t.Errorf("pinned command should move to the Pinned section above Recent:\n%s", view)
	}
	if strings.Count(view, "go test") != 1 {
		t.Errorf("a pinned command should not also be listed under Recent:\n%s", view)
	}

	// The palette gains a tmp row for it.
	found := false
	for _, c := range m.commands {
		if c.IsScratch && c.Command == "go test ./..." {
			found = c.Directory == "/p" && c.DisplayName == "root" && c.Type == "tmp"
		}
	}
	if !found {
		t.Errorf("palette should have a tmp row for the pinned command: %+v", m.commands)
	}

	// Cursor is on the pinned row (first); p again unpins it.
	m.shellCursor = 0
	m, _ = press(t, m, runes("p"))
	if len(unpinned) != 1 || unpinned[0] != "/p|go test ./..." {
		t.Fatalf("UnpinShell calls = %v", unpinned)
	}
	if strings.Contains(shellView(m), "Pinned") {
		t.Errorf("Pinned section should disappear when empty:\n%s", shellView(m))
	}
	for _, c := range m.commands {
		if c.IsScratch {
			t.Errorf("unpinned command still in the palette: %+v", c)
		}
	}
}

func TestShellPicker_PinUnavailableShowsHint(t *testing.T) {
	m := shellTestModel()
	m, _ = press(t, m, key(tea.KeyCtrlR))
	m, _ = press(t, m, runes("p"))
	if !strings.Contains(shellView(m), "unavailable") {
		t.Errorf("pinning without a backend should say so:\n%s", shellView(m))
	}
}

func TestShellPicker_PromoteSavesAndReloads(t *testing.T) {
	var promoted []string
	reloads := 0
	m := shellTestModel()
	m.backend.Scratch = []scratch.Entry{{Dir: "/p/infra", Command: "pnpm cdk diff FooStack"}}
	m.backend.UnpinShell = func(dir, cmd string) error { return nil }
	m.backend.PromoteShell = func(dir, cmd, name string) error {
		promoted = append(promoted, dir+"|"+cmd+"|"+name)
		return nil
	}
	m.backend.Reload = func() (*config.Config, error) { reloads++; return m.config, nil }
	m.loadCommands()

	m, _ = press(t, m, key(tea.KeyCtrlR))
	m, _ = press(t, m, runes("a"))
	if m.mode != modeShellPromote {
		t.Fatalf("mode = %v, want shell-promote", m.mode)
	}
	if got := m.saveInput.Value(); got != "cdk-diff-FooStack" {
		t.Errorf("suggested name = %q, want cdk-diff-FooStack", got)
	}
	m, _ = press(t, m, key(tea.KeyEnter))

	if len(promoted) != 1 || promoted[0] != "/p/infra|pnpm cdk diff FooStack|cdk-diff-FooStack" {
		t.Fatalf("PromoteShell calls = %v", promoted)
	}
	if reloads != 1 {
		t.Errorf("config should be reloaded once after promoting, got %d", reloads)
	}
	if m.mode != modeShellPick {
		t.Errorf("mode = %v, want back in the picker", m.mode)
	}
	if len(m.backend.Scratch) != 0 {
		t.Errorf("a promoted pinned command should be unpinned, scratch = %+v", m.backend.Scratch)
	}
	if !strings.Contains(shellView(m), "saved") {
		t.Errorf("picker should confirm the save:\n%s", shellView(m))
	}
}

func TestShellPicker_PromoteEscReturnsToPicker(t *testing.T) {
	m := shellTestModel()
	m.backend.PromoteShell = func(string, string, string) error { return nil }
	m, _ = press(t, m, key(tea.KeyCtrlR))
	m, _ = press(t, m, key(tea.KeyCtrlS))
	if m.mode != modeShellPromote {
		t.Fatalf("Ctrl+S should also start promoting, mode = %v", m.mode)
	}
	m, _ = press(t, m, key(tea.KeyEscape))
	if m.mode != modeShellPick {
		t.Errorf("Esc in the prompt should return to the picker, mode = %v", m.mode)
	}
}

func TestShellPicker_PromoteUnavailableAndErrors(t *testing.T) {
	m := shellTestModel()
	m, _ = press(t, m, key(tea.KeyCtrlR))
	m, _ = press(t, m, runes("a"))
	if m.mode != modeShellPick || !strings.Contains(shellView(m), "unavailable") {
		t.Errorf("promote without a config file should stay in the picker with a hint:\n%s", shellView(m))
	}

	m.backend.PromoteShell = func(string, string, string) error { return errors.New("boom") }
	m, _ = press(t, m, runes("a"))
	m, _ = press(t, m, key(tea.KeyEnter))
	if m.mode != modeShellPick || !strings.Contains(shellView(m), "save failed: boom") {
		t.Errorf("a failed save should return to the picker with the error:\n%s", shellView(m))
	}
}

func TestSuggestShellName(t *testing.T) {
	cases := map[string]string{
		"go test ./internal/ui/...": "go-test",
		"FOO=1 make build":          "make-build",
		"npm run dev":               "dev",
		"pnpm cdk diff FooStack":    "cdk-diff-FooStack",
		"./scripts/deploy.sh prod":  "prod",
		"terraform plan -out x":     "terraform-plan",
		"./run.sh":                  "command",
	}
	for cmd, want := range cases {
		if got := suggestShellName(cmd); got != want {
			t.Errorf("suggestShellName(%q) = %q, want %q", cmd, got, want)
		}
	}
}

func TestScratchRowsInPalette(t *testing.T) {
	m := shellTestModel()
	m.backend.Scratch = []scratch.Entry{
		{Dir: "/p/infra/stacks", Command: "pnpm cdk diff FooStack"},
		{Dir: "/elsewhere", Command: "make x"},
	}
	m.loadCommands()
	m.updateFilteredCommands()

	var rows []CommandInfo
	for _, c := range m.commands {
		if c.IsScratch {
			rows = append(rows, c)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 scratch rows, got %+v", rows)
	}
	if rows[0].DisplayName != "infra" || rows[0].Directory != "/p/infra/stacks" || rows[0].Type != "tmp" {
		t.Errorf("row in a location should take its display name: %+v", rows[0])
	}
	if rows[1].DisplayName != "/elsewhere" {
		t.Errorf("row outside every location should show its dir: %+v", rows[1])
	}
	if view := shellView(m); !strings.Contains(view, "[tmp]") {
		t.Errorf("scratch rows should carry a [tmp] badge:\n%s", view)
	}

	// Runnable like any row, with its own directory.
	for i, c := range m.filteredCommands {
		if c.IsScratch && c.Command == "pnpm cdk diff FooStack" {
			m.currentIndex = i
		}
	}
	m, _ = press(t, m, key(tea.KeyEnter))
	if len(m.results) != 1 || m.results[0].Directory != "/p/infra/stacks" || m.results[0].DisplayName != "infra" {
		t.Errorf("results = %+v", m.results)
	}
}

// folderTestModel has one command run three times: twice in /p/infra/stacks
// (a subfolder of the infra location) and once in /p/infra itself.
func folderTestModel() Model {
	m := shellTestModel()
	m.backend.ShellHistory = []shelllog.Record{
		{Time: shellNow.Add(-30 * time.Minute), Dir: "/p/infra/stacks", Command: "make plan"},
		{Time: shellNow.Add(-20 * time.Minute), Dir: "/p/infra", Command: "make plan"},
		{Time: shellNow.Add(-10 * time.Minute), Dir: "/p/infra/stacks", Command: "make plan"},
	}
	return m
}

func TestShellPicker_ByLocationGroupsSubfolders(t *testing.T) {
	m := folderTestModel()
	m, _ = press(t, m, key(tea.KeyCtrlR))
	view := shellView(m)
	if !strings.Contains(view, "by location") || strings.Count(view, "make plan") != 1 || !strings.Contains(view, "3×") {
		t.Errorf("by location should show one row counting all three runs:\n%s", view)
	}
	if strings.Contains(view, "infra/stacks") {
		t.Errorf("by location should show the location, not the subfolder:\n%s", view)
	}
	m, _ = press(t, m, key(tea.KeyEnter))
	if len(m.results) != 1 || m.results[0].Directory != "/p/infra" {
		t.Errorf("by location should run in the location's directory, results = %+v", m.results)
	}
}

func TestShellPicker_ByFolderCountsPerFolder(t *testing.T) {
	var saved []string
	var promoted []string
	m := folderTestModel()
	m.backend.SaveShellView = func(p string, byFolder bool) error {
		saved = append(saved, fmt.Sprintf("%s:%v", p, byFolder))
		return nil
	}
	m.backend.PromoteShell = func(dir, cmd, name string) error { promoted = append(promoted, dir); return nil }
	m, _ = press(t, m, key(tea.KeyCtrlR))
	m, _ = press(t, m, runes("f"))

	view := shellView(m)
	if !strings.Contains(view, "by folder") || strings.Count(view, "make plan") != 2 {
		t.Errorf("by folder should list the command once per folder:\n%s", view)
	}
	if !strings.Contains(view, "infra/stacks") || !strings.Contains(view, "2×") || !strings.Contains(view, "1×") {
		t.Errorf("by folder should show each folder with its own count:\n%s", view)
	}
	if len(saved) != 1 || saved[0] != "7d:true" {
		t.Errorf("toggle should be remembered, saved = %v", saved)
	}

	// The newest group is infra/stacks; promoting it passes the subfolder.
	m, _ = press(t, m, runes("a"))
	m, _ = press(t, m, key(tea.KeyEnter))
	if len(promoted) != 1 || promoted[0] != "/p/infra/stacks" {
		t.Errorf("by folder should promote with the exact folder, got %v", promoted)
	}

	m, _ = press(t, m, runes("f"))
	if !strings.Contains(shellView(m), "by location") {
		t.Errorf("f should toggle back:\n%s", shellView(m))
	}
}

func TestShellPicker_StartsOnSavedView(t *testing.T) {
	m := folderTestModel()
	m.backend.ShellByFolder = true
	m, _ = press(t, m, key(tea.KeyCtrlR))
	if !strings.Contains(shellView(m), "by folder") {
		t.Errorf("picker should open on the saved grouping:\n%s", shellView(m))
	}
}

func TestShellPicker_ByLocationOutsideEveryLocationKeepsDir(t *testing.T) {
	m := shellTestModel()
	m.config.Locations = m.config.Locations[1:] // only infra; /p is no longer a location
	m.backend.ShellHistory = []shelllog.Record{{Time: shellNow.Add(-time.Minute), Dir: "/p/tools", Command: "./gen.sh"}}
	m, _ = press(t, m, key(tea.KeyCtrlR))
	m, _ = press(t, m, key(tea.KeyEnter))
	if len(m.results) != 1 || m.results[0].Directory != "/p/tools" {
		t.Errorf("a command outside every location should keep its own dir, results = %+v", m.results)
	}
}
