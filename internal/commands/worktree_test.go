package commands

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// stubGit replaces the git call for one test with a fixed answer.
func stubGit(t *testing.T, out string, err error) {
	t.Helper()
	prev := gitOutput
	gitOutput = func(string, ...string) ([]byte, error) { return []byte(out), err }
	t.Cleanup(func() { gitOutput = prev })
}

const porcelainFixture = `worktree /repo
HEAD 1111111111111111111111111111111111111111
branch refs/heads/main

worktree /repo-feature
HEAD 2222222222222222222222222222222222222222
branch refs/heads/feature/login

worktree /repo.git
HEAD 3333333333333333333333333333333333333333
bare

worktree /repo-old
HEAD 4444444444444444444444444444444444444444
detached
locked
`

// The porcelain listing is read block by block: a branch names the checkout, a
// detached HEAD is named by its short hash, and a bare entry (nothing to run in)
// is dropped. Git's order — main worktree first — is kept.
func TestParseWorktreeList(t *testing.T) {
	got := parseWorktreeList([]byte(porcelainFixture))
	want := []struct{ path, label string }{
		{"/repo", "main"},
		{"/repo-feature", "feature/login"},
		{"/repo-old", "(detached 4444444)"},
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %+v, want %d", got, len(want))
	}
	for i, w := range want {
		if got[i].Path != w.path || got[i].Label != w.label {
			t.Errorf("entry %d = %+v, want path %q label %q", i, got[i], w.path, w.label)
		}
		if got[i].Current {
			t.Errorf("entry %d marked current before any directory was given", i)
		}
	}
}

// Outside a git repository (or without git) there is nothing to pick; a single
// checkout is equally nothing to switch to. Both leave the picker disabled.
func TestWorktrees_DisabledWithoutAlternatives(t *testing.T) {
	stubGit(t, "", errors.New("fatal: not a git repository"))
	if got := Worktrees("/somewhere"); got != nil {
		t.Errorf("Worktrees outside git = %+v, want nil", got)
	}

	stubGit(t, "worktree /repo\nHEAD 1111111\nbranch refs/heads/main\n\n", nil)
	if got := Worktrees("/repo"); got != nil {
		t.Errorf("Worktrees with one checkout = %+v, want nil", got)
	}
}

// The checkout containing the project directory is marked current, even from a
// subdirectory of it.
func TestWorktrees_MarksCurrent(t *testing.T) {
	stubGit(t, porcelainFixture, nil)
	got := Worktrees("/repo-feature/packages/web")
	if len(got) != 3 {
		t.Fatalf("entries = %+v, want 3", got)
	}
	for _, e := range got {
		if e.Current != (e.Path == "/repo-feature") {
			t.Errorf("%s current = %v", e.Path, e.Current)
		}
	}
}

// Against a real repository: the main checkout and a linked worktree are both
// listed, by branch, with the one asked about marked current.
func TestWorktrees_RealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	main := filepath.Join(root, "main")
	if err := os.MkdirAll(filepath.Join(main, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", main}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	git("commit", "-q", "--allow-empty", "-m", "init")
	git("worktree", "add", "-q", filepath.Join(root, "feature"), "-b", "feature")

	got := Worktrees(filepath.Join(main, "sub"))
	if len(got) != 2 {
		t.Fatalf("entries = %+v, want main + feature", got)
	}
	if got[0].Label != "main" || !got[0].Current {
		t.Errorf("first = %+v, want the current main checkout", got[0])
	}
	if got[1].Label != "feature" || got[1].Current {
		t.Errorf("second = %+v, want the feature worktree, not current", got[1])
	}
	if real, _ := filepath.EvalSymlinks(filepath.Join(root, "feature")); got[1].Path != real {
		t.Errorf("second path = %q, want %q", got[1].Path, real)
	}
}

// The project directory keeps its place inside the repository when moving to
// another checkout; a project outside the current checkout lands at its root.
func TestMirrorBaseDir(t *testing.T) {
	tests := []struct{ current, target, baseDir, want string }{
		{"/repo", "/repo-wt", "/repo", "/repo-wt"},
		{"/repo", "/repo-wt", "/repo/apps/web", "/repo-wt/apps/web"},
		{"/repo", "/repo-wt", "/elsewhere/project", "/repo-wt"},
		{"", "/repo-wt", "/repo/apps/web", "/repo-wt"},
	}
	for _, tc := range tests {
		if got := mirrorBaseDir(tc.current, tc.target, tc.baseDir); got != tc.want {
			t.Errorf("mirrorBaseDir(%q, %q, %q) = %q, want %q", tc.current, tc.target, tc.baseDir, got, tc.want)
		}
	}
}
