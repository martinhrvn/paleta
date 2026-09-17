package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/martinhrvn/paleta/internal/config"
)

// BootstrapOutcome says what happened when plt found no configuration and offered
// to make one.
type BootstrapOutcome int

const (
	// BootstrapSkipped means the wizard was not offered: the working directory is
	// no place to scan ($HOME, a parent of it, or the filesystem root), or a
	// config file is already sitting there. The caller falls back to its usual
	// "run plt init" hint.
	BootstrapSkipped         BootstrapOutcome = iota
	BootstrapWritten                          // a .pltrc was written; the caller can load it and carry on
	BootstrapNoProjects                       // nothing detected in the tree, so nothing to offer
	BootstrapCanceled                         // user aborted the wizard
	BootstrapNothingSelected                  // wizard confirmed with no locations ticked
)

func (o BootstrapOutcome) String() string {
	switch o {
	case BootstrapSkipped:
		return "BootstrapSkipped"
	case BootstrapWritten:
		return "BootstrapWritten"
	case BootstrapNoProjects:
		return "BootstrapNoProjects"
	case BootstrapCanceled:
		return "BootstrapCanceled"
	case BootstrapNothingSelected:
		return "BootstrapNothingSelected"
	}
	return fmt.Sprintf("BootstrapOutcome(%d)", int(o))
}

// Bootstrap turns "no paleta configuration found here" into the init wizard, so
// the first `plt` in a repo goes straight from nothing to a working palette
// instead of an error telling the user to run something else. It shows the same
// wizard `plt init` and Ctrl+N use, scanning and writing at the repository root
// when there is one (see bootstrapRoot) and in the working directory otherwise.
// The working directory is left alone, so the caller still resolves tools and
// reloads config relative to where the user actually is.
//
// It performs no stdout output — `plt select` writes its selection JSON there —
// so the caller reports the outcome (on stderr) and decides whether to continue.
func Bootstrap() (BootstrapOutcome, error) {
	wd, err := os.Getwd()
	if err != nil {
		return BootstrapSkipped, fmt.Errorf("getting working directory: %w", err)
	}
	root := bootstrapRoot(wd)

	home, _ := os.UserHomeDir() // an unset $HOME just means no home guard to apply
	if !bootstrapAllowed(root, home) {
		return BootstrapSkipped, nil
	}
	// Scanning is only ever a substitute for a config that isn't there. One that
	// exists but failed to load is the user's file to fix, not ours to replace.
	if _, err := os.Stat(filepath.Join(root, config.ConfigFileName)); err == nil {
		return BootstrapSkipped, nil
	}

	outcome, err := RunInitWizard(root, false)
	switch outcome {
	case InitWritten:
		return BootstrapWritten, err
	case InitNoProjects:
		return BootstrapNoProjects, err
	case InitCanceled:
		return BootstrapCanceled, err
	default:
		return BootstrapNothingSelected, err
	}
}

// bootstrapRoot picks the directory a bootstrap should scan and write to: the
// nearest ancestor holding a .git (a directory for a normal clone, a file for a
// submodule or linked worktree), falling back to dir itself.
//
// A .pltrc describes a repository, not whichever folder someone happened to be
// standing in when they first ran plt — writing it at the root is what makes the
// file worth committing, and it means the config is found from anywhere in the
// tree afterwards.
func bootstrapRoot(dir string) string {
	current := resolve(dir)
	for {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return resolve(dir)
		}
		current = parent
	}
}

// bootstrapAllowed reports whether dir is somewhere plt may scan unprompted.
// `plt init` has always scanned the working directory, but it was asked for by
// name; this runs off a bare `plt`, so it refuses the places where a scan would
// be a surprise: the home directory itself, anything above it, and the root.
func bootstrapAllowed(dir, home string) bool {
	dir = resolve(dir)
	if dir == string(filepath.Separator) {
		return false
	}
	if home == "" {
		return true
	}
	home = resolve(home)
	if dir == home {
		return false
	}
	return !strings.HasPrefix(home, dir+string(filepath.Separator))
}

// resolve cleans a path and follows symlinks where it can, so a directory and
// $HOME are compared in the same terms (os.Getwd already returns a resolved path).
func resolve(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}
