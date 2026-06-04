package commands

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/martinhrvn/paleta/internal/history"
)

// StatsSort selects how `plt stats` orders its rows.
type StatsSort string

const (
	SortFrecency StatsSort = "frecency" // default: combined frequency + recency score
	SortCount    StatsSort = "count"    // most-run first
	SortRecent   StatsSort = "recent"   // most-recently-used first
)

// StatsOptions configures FormatStats.
type StatsOptions struct {
	By    StatsSort // ordering; empty means SortFrecency
	Limit int       // max rows to print; 0 means all
}

// statRow is one command's history, prepared for display.
type statRow struct {
	label string // "location: command"
	entry history.CommandEntry
	score float64
}

// FormatStats renders recorded command history as an aligned table. now is the
// reference time for recency/score (injected for testability).
func FormatStats(hist *history.History, opts StatsOptions, now time.Time) string {
	all := hist.All()
	if len(all) == 0 {
		return "No command history yet.\nRun commands with 'plt select' to start tracking usage."
	}

	rows := make([]statRow, 0, len(all))
	totalRuns := 0
	for key, entry := range all {
		totalRuns += entry.Count
		rows = append(rows, statRow{
			label: labelFromKey(key),
			entry: entry,
			score: hist.ScoreEntry(entry, now),
		})
	}

	sortRows(rows, opts.By)

	shown := rows
	if opts.Limit > 0 && opts.Limit < len(shown) {
		shown = shown[:opts.Limit]
	}

	return renderTable(shown, now) + "\n\n" + summaryLine(totalRuns, len(rows), len(shown))
}

// labelFromKey turns a "location:command" history key into a "location: command"
// display label. Only the first colon is treated as the separator, since command
// strings may themselves contain colons (e.g. "npm run test:watch").
func labelFromKey(key string) string {
	if i := strings.IndexByte(key, ':'); i >= 0 {
		return key[:i] + ": " + key[i+1:]
	}
	return key
}

// sortRows orders rows by the requested key, with a stable label tiebreak so
// output is deterministic.
func sortRows(rows []statRow, by StatsSort) {
	less := map[StatsSort]func(a, b statRow) bool{
		SortCount:  func(a, b statRow) bool { return a.entry.Count > b.entry.Count },
		SortRecent: func(a, b statRow) bool { return a.entry.LastAccess.After(b.entry.LastAccess) },
		SortFrecency: func(a, b statRow) bool {
			return a.score > b.score
		},
	}[by]
	if less == nil {
		less = func(a, b statRow) bool { return a.score > b.score }
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if less(rows[i], rows[j]) {
			return true
		}
		if less(rows[j], rows[i]) {
			return false
		}
		return rows[i].label < rows[j].label
	})
}

// renderTable builds the aligned RUNS / LAST / SCORE / COMMAND table.
func renderTable(rows []statRow, now time.Time) string {
	const (
		hRuns  = "RUNS"
		hLast  = "LAST"
		hScore = "SCORE"
		hCmd   = "COMMAND"
	)

	runsW, lastW, scoreW := len(hRuns), len(hLast), len(hScore)
	runs := make([]string, len(rows))
	last := make([]string, len(rows))
	score := make([]string, len(rows))
	for i, r := range rows {
		runs[i] = strconv.Itoa(r.entry.Count)
		last[i] = history.FormatSince(now.Sub(r.entry.LastAccess))
		score[i] = strconv.FormatFloat(r.score, 'f', 1, 64)
		runsW = max(runsW, len(runs[i]))
		lastW = max(lastW, len(last[i]))
		scoreW = max(scoreW, len(score[i]))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%*s  %-*s  %*s  %s\n", runsW, hRuns, lastW, hLast, scoreW, hScore, hCmd)
	for i, r := range rows {
		fmt.Fprintf(&b, "%*s  %-*s  %*s  %s\n", runsW, runs[i], lastW, last[i], scoreW, score[i], r.label)
	}
	return strings.TrimRight(b.String(), "\n")
}

// summaryLine reports totals; it notes truncation when fewer rows are shown than
// exist.
func summaryLine(totalRuns, total, shown int) string {
	s := fmt.Sprintf("%s across %s",
		pluralize(totalRuns, "run"), pluralize(total, "command"))
	if shown < total {
		s += fmt.Sprintf(" (showing top %d)", shown)
	}
	return s
}

func pluralize(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
