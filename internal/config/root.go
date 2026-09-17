package config

import (
	"os"
	"path/filepath"
)

// FindProjectRoot returns the directory that identifies the project containing
// start: the nearest ancestor (start included) holding a .pltrc or a .git — a
// directory for a normal clone, a file for a submodule or linked worktree — with
// symlinks resolved so the same project always yields the same path. With
// neither in sight it returns start itself. This is the identity command
// history is keyed by and the place a bootstrap writes its .pltrc, so every
// caller has to agree on it.
func FindProjectRoot(start string) string {
	dir := resolvePath(start)
	root, ok := ascend(dir, func(d string) bool {
		return exists(filepath.Join(d, ConfigFileName)) || exists(filepath.Join(d, ".git"))
	})
	if !ok {
		return dir
	}
	return root
}

// ascend walks from dir up to the filesystem root and returns the first
// directory found accepts.
func ascend(dir string, found func(string) bool) (string, bool) {
	for {
		if found(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// resolvePath makes a path absolute and follows symlinks where it can, so two
// spellings of one directory compare equal.
func resolvePath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}

// exists reports whether path names anything at all (a file, a directory, a
// symlink — even a dangling one).
func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
