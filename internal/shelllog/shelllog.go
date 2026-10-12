// Package shelllog is the append-only log of commands typed in the shell (not
// run through the palette) while inside a project. The zsh integration's
// preexec hook feeds it through `plt shell-record`; the selector's Ctrl+R
// picker reads it back so ad-hoc commands can be re-run, pinned or promoted.
package shelllog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/martinhrvn/paleta/internal/history"
)

// Record is one command as it was typed, where, and when.
type Record struct {
	Time    time.Time `json:"ts"`
	Dir     string    `json:"cwd"`
	Command string    `json:"cmd"`
}

const (
	// DefaultKeep is how many records Prune keeps.
	DefaultKeep = 2000
	// DefaultShown caps how many distinct commands the picker lists.
	DefaultShown = 200
	maxLine      = 1 << 20
)

// LogPath is the project's log file: <DataDir>/shell/<ProjectKey>.jsonl.
func LogPath(projectRoot string) string {
	return filepath.Join(history.DataDir(), "shell", history.ProjectKey(projectRoot)+".jsonl")
}

// Append writes r as one JSON line. The line goes out in a single write under
// an exclusive lock so concurrent shells never interleave.
func Append(path string, r Record) error {
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create shell log directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	_, err = f.Write(append(line, '\n'))
	return err
}

// Load reads every well-formed record, oldest first. A missing file is an
// empty log; a torn or malformed line is skipped.
func Load(path string) ([]Record, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH); err != nil {
		return nil, err
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	var recs []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), maxLine)
	for sc.Scan() {
		var r Record
		if json.Unmarshal(sc.Bytes(), &r) != nil || strings.TrimSpace(r.Command) == "" {
			continue
		}
		recs = append(recs, r)
	}
	return recs, sc.Err()
}

// Group is one row of the picker: the commands that share a key, represented
// by the latest of them (so its directory is where it last ran), and how many
// times they ran in the period.
type Group struct {
	Record
	Count int
}

// ByCommand groups records by command text alone.
func ByCommand(r Record) string { return strings.TrimSpace(r.Command) }

// ByDirAndCommand groups records by directory and command text, so the same
// command run in two folders is two groups.
func ByDirAndCommand(r Record) string { return r.Dir + "\x00" + strings.TrimSpace(r.Command) }

// Recent groups the records at or after since (zero means no cutoff) by key,
// newest group first. limit <= 0 means no limit.
func Recent(recs []Record, since time.Time, limit int, key func(Record) string) []Group {
	index := map[string]int{}
	var out []Group
	for _, r := range recs {
		if !since.IsZero() && r.Time.Before(since) {
			continue
		}
		k := key(r)
		i, seen := index[k]
		if !seen {
			index[k] = len(out)
			out = append(out, Group{Record: r, Count: 1})
			continue
		}
		out[i].Count++
		if !r.Time.Before(out[i].Time) {
			out[i].Record = r
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ignoredWords are commands not worth remembering: navigation, housekeeping,
// and plt itself.
var ignoredWords = map[string]bool{
	"ls": true, "cd": true, "clear": true, "exit": true, "pwd": true,
	"fg": true, "bg": true, "history": true, "plt": true, "l": true, "ll": true,
}

// Ignored reports whether cmd should not be recorded: blank, or its first word
// is one of the ignored commands.
func Ignored(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return true
	}
	return ignoredWords[fields[0]]
}

// Prune trims the log to its newest keep records once it has grown past twice
// that, so the rewrite happens rarely.
func Prune(path string, keep int) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := bytes.SplitAfter(data, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= 2*keep {
		return nil
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, bytes.Join(lines[len(lines)-keep:], nil), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
