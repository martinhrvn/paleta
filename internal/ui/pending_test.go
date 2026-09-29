package ui

import (
	"strings"
	"testing"

	"github.com/martinhrvn/paleta/internal/config"
)

// pendingTestConfig has one resolved location and one whose type still has to be
// resolved in the background.
func pendingTestConfig() *config.Config {
	return &config.Config{
		Locations: []config.Location{
			{
				Name:     "web",
				Location: "/repo/web",
				Types:    config.Types{"npm"},
				Commands: []config.Command{{Name: "build", Command: "npm run build"}},
			},
			{
				Name:         "infra",
				Location:     "/repo/infra",
				Types:        config.Types{"make"},
				PendingTypes: []string{"make"},
			},
		},
		Frecency: config.DefaultFrecencyConfig(),
	}
}

func rowDisplays(m Model) []string {
	out := make([]string, len(m.commands))
	for i, c := range m.commands {
		out[i] = c.Display
	}
	return out
}

func TestSelector_ShowsLoadingRowForPendingLocation(t *testing.T) {
	m := NewModel(pendingTestConfig(), Backend{})
	m.loadCommands()

	var loading *CommandInfo
	for i := range m.commands {
		if m.commands[i].Loading {
			loading = &m.commands[i]
		}
	}
	if loading == nil {
		t.Fatalf("no loading row, rows = %v", rowDisplays(m))
	}
	if !strings.Contains(loading.Display, "infra") {
		t.Errorf("loading row = %q, want it to name the location", loading.Display)
	}
	if loading.Command != "" {
		t.Errorf("loading row Command = %q, want empty so it can't be run", loading.Command)
	}
}

func TestSelector_MergesResolvedCommandsInPlace(t *testing.T) {
	m := NewModel(pendingTestConfig(), Backend{})
	m.loadCommands()

	updated, _ := m.Update(pendingResolvedMsg{
		index:    1,
		commands: []config.Command{{Name: "deploy", Command: "make deploy"}},
	})
	m = updated.(Model)

	got := rowDisplays(m)
	want := []string{"web: build", "infra: deploy"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("rows = %v, want %v (resolved in the location's own slot)", got, want)
	}
	for _, c := range m.commands {
		if c.Loading {
			t.Errorf("loading row still present after resolution: %v", got)
		}
	}
	if len(m.config.Locations[1].PendingTypes) != 0 {
		t.Errorf("PendingTypes = %v, want cleared", m.config.Locations[1].PendingTypes)
	}
}

// TestSelector_LoadingRowIsNotRunnable: a placeholder must never be queued or
// executed — it has no command to run.
func TestSelector_LoadingRowIsNotRunnable(t *testing.T) {
	m := NewModel(pendingTestConfig(), Backend{})
	m.loadCommands()
	m.updateFilteredCommands()

	loadingIdx := -1
	for i, c := range m.filteredCommands {
		if c.Loading {
			loadingIdx = i
		}
	}
	if loadingIdx < 0 {
		t.Fatalf("no loading row to test")
	}

	m.currentIndex = loadingIdx
	m.toggleSelection(loadingIdx)
	if len(m.queue) != 0 {
		t.Errorf("queue = %v, want a loading row not to be queueable", m.queue)
	}

	m.confirmSelection()
	if len(m.results) != 0 {
		t.Errorf("results = %+v, want a loading row not to be runnable", m.results)
	}
}

func TestSelector_ResolveWarningsReachTheBanner(t *testing.T) {
	m := NewModel(pendingTestConfig(), Backend{})
	m.loadCommands()

	updated, _ := m.Update(pendingResolvedMsg{
		index:    1,
		warnings: []config.Warning{{Kind: "parser", Scope: "location", Context: "infra", Reason: "make parser timed out"}},
	})
	m = updated.(Model)

	if len(m.config.Warnings) != 1 {
		t.Fatalf("warnings = %+v, want the resolve warning surfaced", m.config.Warnings)
	}
	if banner := m.renderWarningBanner(); banner == "" {
		t.Errorf("warning banner empty after a resolve warning")
	}
}

// refreshingTestConfig has a location whose deferred type is showing its cached
// commands while it re-runs in the background.
func refreshingTestConfig() *config.Config {
	return &config.Config{
		Locations: []config.Location{
			{
				Name:         "infra",
				Location:     "/repo/infra",
				Types:        config.Types{"gradle"},
				Commands:     []config.Command{{Name: "build", Command: "./gradlew build", Type: "gradle"}},
				PendingTypes: []string{"gradle"},
				Refreshing:   []string{"gradle"},
			},
		},
		Frecency: config.DefaultFrecencyConfig(),
	}
}

// Cached rows are real rows — runnable straight away — with no placeholder.
func TestSelector_CachedRowsAreRunnableWhileRefreshing(t *testing.T) {
	m := NewModel(refreshingTestConfig(), Backend{})
	m.loadCommands()
	m.updateFilteredCommands()

	if got := rowDisplays(m); len(got) != 1 || got[0] != "infra: build" {
		t.Fatalf("rows = %v, want only the cached [infra: build]", got)
	}
	m.currentIndex = 0
	m.confirmSelection()
	if len(m.results) != 1 || m.results[0].Command != "./gradlew build" {
		t.Errorf("results = %+v, want the cached command to run", m.results)
	}
}

// The bottom line says what is refreshing, and stops once the result arrives.
func TestSelector_BottomLineShowsRefresh(t *testing.T) {
	m := NewModel(refreshingTestConfig(), Backend{})
	m.loadCommands()

	if view := m.View(); !strings.Contains(view, "refreshing gradle") {
		t.Errorf("view lacks the refresh indicator:\n%s", view)
	}

	updated, _ := m.Update(pendingResolvedMsg{
		index:    0,
		commands: []config.Command{{Name: "build", Command: "./gradlew build", Type: "gradle"}, {Name: "test", Command: "./gradlew test", Type: "gradle"}},
	})
	m = updated.(Model)

	if view := m.View(); strings.Contains(view, "refreshing") {
		t.Errorf("refresh indicator still shown after the result arrived:\n%s", view)
	}
	if len(m.config.Locations[0].Refreshing) != 0 {
		t.Errorf("Refreshing = %v, want cleared", m.config.Locations[0].Refreshing)
	}
	if got := rowDisplays(m); len(got) != 2 {
		t.Errorf("rows = %v, want the refreshed list", got)
	}
}
