package commands

import (
	"strings"
	"testing"
	"time"

	"github.com/martinhrvn/paleta/internal/history"
)

func statsFixture(now time.Time) *history.History {
	return &history.History{
		Commands: map[string]history.CommandEntry{
			"svc:npm run build": {Count: 12, LastAccess: now.Add(-3 * time.Hour), FirstAccess: now.Add(-6 * 24 * time.Hour)},
			"api:go test ./...": {Count: 7, LastAccess: now.Add(-2 * 24 * time.Hour), FirstAccess: now.Add(-10 * 24 * time.Hour)},
			"web:npm run dev":   {Count: 3, LastAccess: now.Add(-21 * 24 * time.Hour), FirstAccess: now.Add(-30 * 24 * time.Hour)},
		},
	}
}

// orderOf returns the indices at which each label first appears in out.
func orderOf(out string, labels ...string) []int {
	idx := make([]int, len(labels))
	for i, l := range labels {
		idx[i] = strings.Index(out, l)
	}
	return idx
}

func TestFormatStats_ByCount(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	out := FormatStats(statsFixture(now), StatsOptions{By: SortCount}, now)

	// Header and key/command labels present.
	for _, want := range []string{"RUNS", "LAST", "SCORE", "COMMAND", "svc: npm run build", "3h ago", "2d ago", "3w ago"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}

	// Ordered by count desc: svc(12) > api(7) > web(3).
	o := orderOf(out, "svc: npm run build", "api: go test ./...", "web: npm run dev")
	if !(o[0] < o[1] && o[1] < o[2]) {
		t.Errorf("rows not ordered by count: %v\n%s", o, out)
	}

	if !strings.Contains(out, "22 runs across 3 commands") {
		t.Errorf("missing summary line:\n%s", out)
	}
}

func TestFormatStats_LabelSplitsOnFirstColon(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	hist := &history.History{Commands: map[string]history.CommandEntry{
		"web:npm run test:watch": {Count: 1, LastAccess: now, FirstAccess: now},
	}}
	out := FormatStats(hist, StatsOptions{}, now)
	if !strings.Contains(out, "web: npm run test:watch") {
		t.Errorf("expected only the first colon to split location/command:\n%s", out)
	}
}

func TestFormatStats_Limit(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	out := FormatStats(statsFixture(now), StatsOptions{By: SortCount, Limit: 2}, now)

	if strings.Contains(out, "web: npm run dev") {
		t.Errorf("limit=2 should drop the third row:\n%s", out)
	}
	if !strings.Contains(out, "(showing top 2)") {
		t.Errorf("expected truncation note:\n%s", out)
	}
	// Summary still counts all commands.
	if !strings.Contains(out, "across 3 commands") {
		t.Errorf("summary should reflect total command count:\n%s", out)
	}
}

func TestFormatStats_Empty(t *testing.T) {
	hist := &history.History{Commands: map[string]history.CommandEntry{}}
	out := FormatStats(hist, StatsOptions{}, time.Now())
	if !strings.Contains(out, "No command history yet") {
		t.Errorf("expected empty-history message, got:\n%s", out)
	}
}
