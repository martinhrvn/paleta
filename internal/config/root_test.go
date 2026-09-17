package config

import (
	"os"
	"path/filepath"
	"testing"
)

// FindProjectRoot stops at the nearest .pltrc or .git — a .git file (worktree,
// submodule) counts — resolves symlinks, and falls back to the start directory.
func TestFindProjectRoot(t *testing.T) {
	base := t.TempDir()
	mk := func(parts ...string) string {
		t.Helper()
		p := filepath.Join(append([]string{base}, parts...)...)
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	touch := func(p string) {
		t.Helper()
		if err := os.WriteFile(p, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	resolved := func(p string) string {
		t.Helper()
		r, err := filepath.EvalSymlinks(p)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	repo := mk("repo")
	mk("repo", ".git")
	deep := mk("repo", "a", "b")
	worktree := mk("wt")
	touch(filepath.Join(worktree, ".git"))
	wtSub := mk("wt", "x")
	proj := mk("proj")
	touch(filepath.Join(proj, ".pltrc"))
	projSrc := mk("proj", "src")
	sub := mk("repo", "sub")
	touch(filepath.Join(sub, ".pltrc"))
	subDeep := mk("repo", "sub", "deep")
	lone := mk("lone", "here")
	link := filepath.Join(base, "link")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}

	cases := []struct{ name, start, want string }{
		{".git directory", deep, repo},
		{".git file", wtSub, worktree},
		{".pltrc without git", projSrc, proj},
		{"nearest marker wins", subDeep, sub},
		{"no marker falls back to start", lone, lone},
		{"symlinked start resolves", filepath.Join(link, "a"), repo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FindProjectRoot(tc.start); got != resolved(tc.want) {
				t.Errorf("FindProjectRoot(%s) = %q, want %q", tc.start, got, resolved(tc.want))
			}
		})
	}
}
