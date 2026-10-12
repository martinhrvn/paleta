package commands

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/scratch"
	"github.com/martinhrvn/paleta/internal/shelllog"
	"github.com/martinhrvn/paleta/internal/ui"
)

// runShellRecord notes a command typed in the shell (not run through plt) in
// the shell log of the project containing cwd. The zsh integration's preexec
// hook calls it in the background for every command; like history recording,
// failures are swallowed because nothing can act on them.
func runShellRecord(args []string, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "Usage: plt shell-record <cwd> <command>")
		return 1
	}
	cwd, command := args[0], args[1]
	if shelllog.Ignored(command) {
		return 0
	}
	if abs, err := filepath.Abs(cwd); err == nil {
		cwd = filepath.Clean(abs)
	}
	root, ok := projectRootOf(cwd)
	if !ok {
		return 0
	}
	path := shelllog.LogPath(root)
	if shelllog.Append(path, shelllog.Record{Time: time.Now(), Dir: cwd, Command: command}) == nil {
		_ = shelllog.Prune(path, shelllog.DefaultKeep)
	}
	return 0
}

// runAdd pins a command in the working directory as a temporary command of the
// enclosing project. Arguments are joined with spaces, so both
// `plt add "pnpm cdk diff Foo"` and `plt add pnpm cdk diff Foo` work.
func runAdd(args []string, stdout, stderr io.Writer) int {
	command := strings.TrimSpace(strings.Join(args, " "))
	if command == "" {
		fmt.Fprintln(stderr, "Usage: plt add <command>")
		return 1
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	root, ok := projectRootOf(cwd)
	if !ok {
		fmt.Fprintln(stderr, "Error: not inside a project (no .pltrc or .git found)")
		return 1
	}
	added, err := PinCommand(root, cwd, command)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if added {
		fmt.Fprintf(stdout, "Pinned (tmp): %s\n", command)
	} else {
		fmt.Fprintf(stdout, "Not added, already pinned: %s\n", command)
	}
	return 0
}

// PinCommand adds command (run in dir) to the project's scratch list.
func PinCommand(root, dir, command string) (bool, error) {
	return updateScratch(root, func(l *scratch.List) bool { return l.Add(dir, command, time.Now()) })
}

// UnpinCommand removes command (run in dir) from the project's scratch list.
func UnpinCommand(root, dir, command string) (bool, error) {
	return updateScratch(root, func(l *scratch.List) bool { return l.Remove(dir, command) })
}

// SetShellView remembers how the Ctrl+R picker was last viewed in the project:
// its time period and whether it groups by folder rather than by location.
func SetShellView(root, period string, byFolder bool) error {
	_, err := updateScratch(root, func(l *scratch.List) bool {
		if l.ShellPeriod == period && l.ShellByFolder == byFolder {
			return false
		}
		l.ShellPeriod, l.ShellByFolder = period, byFolder
		return true
	})
	return err
}

// updateScratch loads the project's scratch list, applies change, and saves
// it when change reports that something changed.
func updateScratch(root string, change func(*scratch.List) bool) (bool, error) {
	path := scratch.Path(root)
	l, err := scratch.Load(path)
	if err != nil {
		return false, err
	}
	if !change(l) {
		return false, nil
	}
	l.ProjectRoot = root
	return true, l.Save(path)
}

// projectRootOf is FindProjectRoot that also says whether dir is inside a
// project at all (FindProjectRoot falls back to dir itself).
func projectRootOf(dir string) (string, bool) {
	root := config.FindProjectRoot(dir)
	for _, marker := range []string{config.ConfigFileName, ".git"} {
		if _, err := os.Lstat(filepath.Join(root, marker)); err == nil {
			return root, true
		}
	}
	return root, false
}

var errOutsideProject = errors.New("directory is outside the project")

// PromoteShellCommand saves a shell command (as run in dir) to .pltrc under
// name, editing the file in place with comments kept. It goes to the location
// containing dir: as is when dir is that location's own directory, prefixed
// with `cd <subdir> &&` when dir lies below it, so it still runs where it was
// typed. A dir no location covers gets a new, unnamed location entry.
func PromoteShellCommand(cfg *config.Config, configPath, dir, command, name string) error {
	base := cfg.BaseDir()
	if abs, err := filepath.Abs(base); err == nil {
		base = abs
	}
	target, ok := relWithin(base, dir)
	if !ok {
		return errOutsideProject
	}
	locName := ""
	if loc, found := cfg.LocationFor(dir); found {
		if locRel, inside := relWithin(base, loc.Location); inside {
			if sub, _ := relWithin(loc.Location, dir); sub != "." {
				command = "cd " + shellQuote(sub) + " && " + command
			}
			target, locName = locRel, loc.Name
		}
	}

	file, err := config.OpenFile(configPath)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("no config file at %s", configPath)
	}
	if err != nil {
		return err
	}
	authored, err := file.Authored()
	if err != nil {
		return err
	}
	newCmd := config.NewCommand(name, []string{command})
	idx := -1
	if locName != "" {
		idx = findAuthoredLocation(authored, locName, "")
	}
	if idx < 0 {
		idx = findAuthoredByDir(authored, target)
	}
	if idx >= 0 {
		err = file.AppendCommand(idx, newCmd)
	} else {
		err = file.AddLocation(config.Location{Name: locName, Location: target, Commands: []config.Command{newCmd}})
	}
	if err != nil {
		return err
	}
	return file.Save()
}

// relWithin returns dir relative to base when dir is base or below it,
// comparing again with symlinks resolved when the paths as given disagree.
func relWithin(base, dir string) (string, bool) {
	try := func(b, d string) (string, bool) {
		rel, err := filepath.Rel(b, d)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			return "", false
		}
		return rel, true
	}
	if rel, ok := try(base, dir); ok {
		return rel, true
	}
	rb, err1 := filepath.EvalSymlinks(base)
	rd, err2 := filepath.EvalSymlinks(dir)
	if err1 != nil || err2 != nil {
		return "", false
	}
	return try(rb, rd)
}

// shellQuote leaves a plain path as is and single-quotes anything else.
func shellQuote(s string) string {
	for _, r := range s {
		if !(r == '/' || r == '.' || r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
		}
	}
	return s
}

// wireShell gives the selector the project's shell log, its pinned temporary
// commands and remembered picker view, and the functions that change them.
// root is the project identity the shell hook recorded under. Nothing here
// touches .pltrc, so it works without a config file; promoting is wired
// separately, only when there is one. Unreadable files just leave the picker
// empty.
func wireShell(be *ui.Backend, root string) {
	if recs, err := shelllog.Load(shelllog.LogPath(root)); err == nil {
		be.ShellHistory = recs
	}
	if l, err := scratch.Load(scratch.Path(root)); err == nil {
		be.Scratch = l.Entries
		be.ShellPeriod = l.ShellPeriod
		be.ShellByFolder = l.ShellByFolder
	}
	be.PinShell = func(dir, command string) error { _, err := PinCommand(root, dir, command); return err }
	be.UnpinShell = func(dir, command string) error { _, err := UnpinCommand(root, dir, command); return err }
	be.SaveShellView = func(period string, byFolder bool) error { return SetShellView(root, period, byFolder) }
}
