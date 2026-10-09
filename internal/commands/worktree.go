package commands

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/martinhrvn/paleta/internal/ui"
)

// gitOutput runs git in dir and returns its stdout. A package variable so tests
// can answer for git (the same seam as runWizard).
var gitOutput = func(dir string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
}

// Worktrees lists the checkouts of the repository containing dir — the main
// worktree and every linked one — marking the one dir belongs to. It returns
// nil when dir is not in a repository, git is not installed, or there is only
// one checkout: then there is nothing to switch to and the picker stays hidden.
func Worktrees(dir string) []ui.WorktreeEntry {
	out, err := gitOutput(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil
	}
	entries := parseWorktreeList(out)
	if len(entries) < 2 {
		return nil
	}
	markCurrentWorktree(entries, dir)
	return entries
}

// parseWorktreeList reads `git worktree list --porcelain`: blank-line separated
// blocks of `worktree <path>`, `HEAD <sha>` and either `branch refs/heads/<name>`
// or `detached`, plus `bare`/`locked`/`prunable` notes. A bare entry has no files
// to run commands in and is dropped. Git's order (main worktree first) is kept.
func parseWorktreeList(out []byte) []ui.WorktreeEntry {
	var entries []ui.WorktreeEntry
	var cur ui.WorktreeEntry
	var head string
	bare := false
	flush := func() {
		if cur.Path != "" && !bare {
			if cur.Label == "" {
				cur.Label = "(detached " + shortHash(head) + ")"
			}
			entries = append(entries, cur)
		}
		cur, head, bare = ui.WorktreeEntry{}, "", false
	}
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "worktree "):
			cur.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			cur.Label = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		case line == "bare":
			bare = true
		}
	}
	flush()
	return entries
}

func shortHash(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// markCurrentWorktree flags the entry whose path is dir or the nearest ancestor
// of it. Git prints resolved paths, so dir is resolved the same way first.
func markCurrentWorktree(entries []ui.WorktreeEntry, dir string) {
	dir = realPath(dir)
	best := -1
	for i, e := range entries {
		if !isWithin(e.Path, dir) {
			continue
		}
		if best < 0 || len(e.Path) > len(entries[best].Path) {
			best = i
		}
	}
	if best >= 0 {
		entries[best].Current = true
	}
}

// currentWorktree returns the entry marked current, if any.
func currentWorktree(entries []ui.WorktreeEntry) (ui.WorktreeEntry, bool) {
	for _, e := range entries {
		if e.Current {
			return e, true
		}
	}
	return ui.WorktreeEntry{}, false
}

// mirrorBaseDir is where a project rooted at baseDir inside the current checkout
// sits inside the target checkout: the same relative place, or the target's root
// when baseDir is not inside the current checkout at all.
func mirrorBaseDir(current, target, baseDir string) string {
	if current == "" || !isWithin(current, baseDir) {
		return target
	}
	rel, err := filepath.Rel(current, baseDir)
	if err != nil {
		return target
	}
	return filepath.Join(target, rel)
}

// isWithin reports whether path is root or lies under it.
func isWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// realPath makes path absolute and follows symlinks where it can, matching how
// git prints worktree paths. A path that does not exist yet is cleaned as is.
func realPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	// Resolve the longest existing prefix so a not-yet-created subdirectory of a
	// symlinked checkout still compares equal to git's resolved path.
	rest := ""
	for p := path; ; {
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Clean(filepath.Join(resolved, rest))
		}
		parent := filepath.Dir(p)
		if parent == p {
			return filepath.Clean(path)
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
}
