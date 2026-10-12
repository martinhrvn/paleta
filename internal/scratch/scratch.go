// Package scratch is a project's list of pinned temporary commands: ad-hoc
// shell commands worth keeping at hand for a while (an infra migration, a
// debugging session) without adding them to .pltrc. It lives outside the
// project, next to the history store, and also remembers the Ctrl+R picker's
// chosen time period.
package scratch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/martinhrvn/paleta/internal/history"
)

// Entry is one pinned command and the directory it runs in.
type Entry struct {
	Dir     string    `json:"dir"`
	Command string    `json:"cmd"`
	Added   time.Time `json:"added"`
}

// List is the persisted scratch file for one project.
type List struct {
	Version       string  `json:"version"`
	ProjectRoot   string  `json:"project_root"`
	ShellPeriod   string  `json:"shell_period,omitempty"`
	ShellByFolder bool    `json:"shell_by_folder,omitempty"`
	Entries       []Entry `json:"entries"`
}

const version = "1.0"

// Path is the project's scratch file: <DataDir>/scratch/<ProjectKey>.json.
func Path(projectRoot string) string {
	return filepath.Join(history.DataDir(), "scratch", history.ProjectKey(projectRoot)+".json")
}

// Load reads the list; a missing file is an empty list.
func Load(path string) (*List, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return &List{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH); err != nil {
		return nil, err
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	var l List
	if err := json.NewDecoder(f).Decode(&l); err != nil {
		return nil, fmt.Errorf("decode scratch file: %w", err)
	}
	return &l, nil
}

// Save writes the list as indented JSON under an exclusive lock.
func (l *List) Save(path string) error {
	l.Version = version
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create scratch directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	_, err = f.Write(data)
	return err
}

func (l *List) index(dir, cmd string) int {
	cmd = strings.TrimSpace(cmd)
	for i, e := range l.Entries {
		if e.Dir == dir && strings.TrimSpace(e.Command) == cmd {
			return i
		}
	}
	return -1
}

// Has reports whether cmd is pinned in dir.
func (l *List) Has(dir, cmd string) bool { return l.index(dir, cmd) >= 0 }

// Add pins cmd in dir; false when it already is.
func (l *List) Add(dir, cmd string, now time.Time) bool {
	if l.Has(dir, cmd) {
		return false
	}
	l.Entries = append(l.Entries, Entry{Dir: dir, Command: strings.TrimSpace(cmd), Added: now})
	return true
}

// Remove unpins cmd in dir; false when it wasn't pinned.
func (l *List) Remove(dir, cmd string) bool {
	i := l.index(dir, cmd)
	if i < 0 {
		return false
	}
	l.Entries = append(l.Entries[:i], l.Entries[i+1:]...)
	return true
}
