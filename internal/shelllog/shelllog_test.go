package shelllog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/martinhrvn/paleta/internal/history"
)

func at(h int) time.Time {
	return time.Date(2026, 10, 12, h, 0, 0, 0, time.UTC)
}

func TestAppendLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell", "x.jsonl")
	recs := []Record{
		{Time: at(9), Dir: "/p", Command: `echo "quoted" \ backslash`},
		{Time: at(10), Dir: "/p/sub", Command: "printf 'a\nb'\tc"},
	}
	for _, r := range recs {
		if err := Append(path, r); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2", len(got))
	}
	for i := range recs {
		if !got[i].Time.Equal(recs[i].Time) || got[i].Dir != recs[i].Dir || got[i].Command != recs[i].Command {
			t.Errorf("record %d = %+v, want %+v", i, got[i], recs[i])
		}
	}
	data, _ := os.ReadFile(path)
	if n := strings.Count(string(data), "\n"); n != 2 {
		t.Fatalf("file should hold one line per record, has %d newlines", n)
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "none.jsonl"))
	if err != nil || got != nil {
		t.Fatalf("Load(missing) = %v, %v; want nil, nil", got, err)
	}
}

func TestLoadSkipsMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.jsonl")
	_ = Append(path, Record{Time: at(1), Dir: "/p", Command: "one"})
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	_, _ = f.WriteString(`{"ts":"2026-10-12T02:00:00Z","cwd":"/p","cmd":"tru` + "\n")
	_ = f.Close()
	_ = Append(path, Record{Time: at(3), Dir: "/p", Command: "three"})

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 2 || got[0].Command != "one" || got[1].Command != "three" {
		t.Fatalf("got %+v, want the two well-formed records", got)
	}
}

func TestRecentNewestFirstGroupedAndLimited(t *testing.T) {
	recs := []Record{
		{Time: at(1), Dir: "/p", Command: "go test ./..."},
		{Time: at(2), Dir: "/p", Command: "make build"},
		{Time: at(3), Dir: "/p/sub", Command: "go test ./... "}, // same command, trailing space, newer dir
		{Time: at(4), Dir: "/p", Command: "npm run dev"},
	}
	got := Recent(recs, time.Time{}, 0, ByCommand)
	want := []string{"npm run dev", "go test ./...", "make build"}
	wantCount := []int{1, 2, 1}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if strings.TrimSpace(got[i].Command) != w || got[i].Count != wantCount[i] {
			t.Errorf("row %d = %q x%d, want %q x%d", i, got[i].Command, got[i].Count, w, wantCount[i])
		}
	}
	if got[1].Dir != "/p/sub" || !got[1].Time.Equal(at(3)) {
		t.Errorf("a group should carry its latest occurrence, got %+v", got[1])
	}
	if got := Recent(recs, time.Time{}, 2, ByCommand); len(got) != 2 {
		t.Errorf("limit 2 gave %d rows", len(got))
	}
}

func TestRecentByDirAndCommandCountsPerFolder(t *testing.T) {
	recs := []Record{
		{Time: at(1), Dir: "/p", Command: "go test ./..."},
		{Time: at(2), Dir: "/p/sub", Command: "go test ./..."},
		{Time: at(3), Dir: "/p", Command: "go test ./..."},
	}
	got := Recent(recs, time.Time{}, 0, ByDirAndCommand)
	if len(got) != 2 || got[0].Dir != "/p" || got[0].Count != 2 || got[1].Dir != "/p/sub" || got[1].Count != 1 {
		t.Fatalf("got %+v, want /p x2 then /p/sub x1", got)
	}
}

func TestRecentHonoursSince(t *testing.T) {
	recs := []Record{
		{Time: at(1), Dir: "/p", Command: "new"},
		{Time: at(5), Dir: "/p", Command: "new"},
		{Time: at(1), Dir: "/p", Command: "old"},
	}
	got := Recent(recs, at(3), 0, ByCommand)
	if len(got) != 1 || got[0].Command != "new" || got[0].Count != 1 {
		t.Fatalf("Recent since 03:00 = %+v, want only the new record, counted within the period", got)
	}
}

func TestIgnored(t *testing.T) {
	cases := map[string]bool{
		"":                     true,
		"   ":                  true,
		"ls":                   true,
		"ls -la":               true,
		"cd ..":                true,
		"clear":                true,
		"exit":                 true,
		"pwd":                  true,
		"fg":                   true,
		"history":              true,
		"plt":                  true,
		"plt select":           true,
		"  plt list":           true,
		"go test ./...":        false,
		"cdk deploy":           false,
		"lsof -i :3000":        false,
		"pnpm cdk diff FooStk": false,
		"cdx":                  false,
	}
	for cmd, want := range cases {
		if got := Ignored(cmd); got != want {
			t.Errorf("Ignored(%q) = %v, want %v", cmd, got, want)
		}
	}
}

func TestPruneKeepsTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.jsonl")
	for i := 0; i < 11; i++ {
		_ = Append(path, Record{Time: at(i), Dir: "/p", Command: strings.Repeat("x", i+1)})
	}
	if err := Prune(path, 5); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	got, _ := Load(path)
	if len(got) != 5 || got[0].Command != strings.Repeat("x", 7) || got[4].Command != strings.Repeat("x", 11) {
		t.Fatalf("after prune got %d records (%+v), want the newest 5", len(got), got)
	}
	// Below the 2*keep threshold nothing happens.
	if err := Prune(path, 5); err != nil {
		t.Fatalf("Prune (no-op): %v", err)
	}
	if got, _ := Load(path); len(got) != 5 {
		t.Fatalf("no-op prune changed the file to %d records", len(got))
	}
}

func TestLogPathUsesProjectKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := "/some/project"
	want := filepath.Join(home, ".paleta", "shell", history.ProjectKey(root)+".jsonl")
	if got := LogPath(root); got != want {
		t.Fatalf("LogPath = %q, want %q", got, want)
	}
}

func TestPeriod(t *testing.T) {
	now := time.Date(2026, 10, 12, 15, 30, 0, 0, time.Local)
	cases := []struct {
		p     Period
		label string
		since time.Time
	}{
		{PeriodHour, "last hour", now.Add(-time.Hour)},
		{PeriodToday, "today", time.Date(2026, 10, 12, 0, 0, 0, 0, time.Local)},
		{PeriodWeek, "last 7 days", now.AddDate(0, 0, -7)},
		{PeriodMonth, "last 30 days", now.AddDate(0, 0, -30)},
		{PeriodAll, "all time", time.Time{}},
	}
	for _, c := range cases {
		if got := c.p.Label(); got != c.label {
			t.Errorf("%v.Label() = %q, want %q", c.p, got, c.label)
		}
		if got := c.p.Since(now); !got.Equal(c.since) {
			t.Errorf("%v.Since() = %v, want %v", c.p, got, c.since)
		}
		if ParsePeriod(c.p.String()) != c.p {
			t.Errorf("ParsePeriod(%q) != %v", c.p.String(), c.p)
		}
	}
	if ParsePeriod("bogus") != DefaultPeriod || ParsePeriod("") != DefaultPeriod {
		t.Error("unknown period strings should fall back to the default")
	}
	if PeriodAll.Next() != PeriodHour || PeriodHour.Prev() != PeriodAll {
		t.Error("Next/Prev should wrap around")
	}
	if PeriodHour.Next() != PeriodToday || PeriodWeek.Prev() != PeriodToday {
		t.Error("Next/Prev should step through the presets in order")
	}
}
