package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// worktreeFixture builds two checkouts of the same project: main/ holds the
// .pltrc, wt/ mirrors its folders (a linked git worktree without the file).
func worktreeFixture(t *testing.T, pltrc string) (main, wt string) {
	t.Helper()
	root := t.TempDir()
	main = filepath.Join(root, "main")
	wt = filepath.Join(root, "wt")
	for _, d := range []string{"web", "packages/a", "packages/b"} {
		for _, base := range []string{main, wt} {
			if err := os.MkdirAll(filepath.Join(base, d), 0755); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(main, ConfigFileName), []byte(pltrc), 0644); err != nil {
		t.Fatal(err)
	}
	return main, wt
}

const worktreePltrc = "locations:\n  - name: web\n    location: web\n    commands: [\"npm start\"]\n  - location: packages/*\n    commands: [\"go test\"]\n"

// A worktree without its own .pltrc reuses the main checkout's file, with every
// relative path and glob resolved under the worktree instead.
func TestReloadAt_ReusesFileUnderWorktree(t *testing.T) {
	main, wt := worktreeFixture(t, worktreePltrc)
	cfg, err := Load(LoadOptions{Workdir: filepath.Join(main, "web"), Defer: true})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	moved, err := cfg.ReloadAt(wt)
	if err != nil {
		t.Fatalf("ReloadAt: %v", err)
	}
	if moved.BaseDir() != wt {
		t.Errorf("BaseDir = %q, want the worktree %q", moved.BaseDir(), wt)
	}
	if want := filepath.Join(main, ConfigFileName); moved.Path != want {
		t.Errorf("Path = %q, want the main checkout's file %q", moved.Path, want)
	}
	if len(moved.Locations) != 3 {
		t.Fatalf("locations = %+v, want web + two glob children", moved.Locations)
	}
	for _, loc := range moved.Locations {
		if rel, err := filepath.Rel(wt, loc.Location); err != nil || strings.HasPrefix(rel, "..") {
			t.Errorf("location %q not under the worktree %q", loc.Location, wt)
		}
	}
	if got, want := moved.Locations[0].Location, filepath.Join(wt, "web"); got != want {
		t.Errorf("web = %q, want %q", got, want)
	}
	if !moved.loadOpts.Defer {
		t.Error("ReloadAt should keep the original load's deferral")
	}
}

// When the worktree has its own .pltrc (committed on that branch), that file is
// what the palette shows there.
func TestReloadAt_PrefersWorktreeOwnFile(t *testing.T) {
	main, wt := worktreeFixture(t, worktreePltrc)
	own := "locations:\n  - name: web\n    location: web\n    commands: [\"pnpm dev\"]\n"
	if err := os.WriteFile(filepath.Join(wt, ConfigFileName), []byte(own), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(LoadOptions{Workdir: main})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	moved, err := cfg.ReloadAt(wt)
	if err != nil {
		t.Fatalf("ReloadAt: %v", err)
	}
	if want := filepath.Join(wt, ConfigFileName); moved.Path != want {
		t.Errorf("Path = %q, want the worktree's own file %q", moved.Path, want)
	}
	if moved.BaseDir() != wt {
		t.Errorf("BaseDir = %q, want %q", moved.BaseDir(), wt)
	}
	if len(moved.Locations) != 1 || moved.Locations[0].Commands[0].Command != "pnpm dev" {
		t.Errorf("locations = %+v, want the worktree file's single pnpm location", moved.Locations)
	}
}

// A global project file (root: main) follows the same rule: its declared root
// gives way to the worktree.
func TestReloadAt_GlobalProjectResolvesUnderWorktree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	main, wt := worktreeFixture(t, "")
	if err := os.Remove(filepath.Join(main, ConfigFileName)); err != nil {
		t.Fatal(err)
	}
	projects := filepath.Join(home, ".config", "paleta", "projects")
	if err := os.MkdirAll(projects, 0755); err != nil {
		t.Fatal(err)
	}
	global := "root: " + main + "\n" + worktreePltrc
	if err := os.WriteFile(filepath.Join(projects, "main.yaml"), []byte(global), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(LoadOptions{Workdir: main})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseDir() != main {
		t.Fatalf("precondition: BaseDir = %q, want %q", cfg.BaseDir(), main)
	}

	moved, err := cfg.ReloadAt(wt)
	if err != nil {
		t.Fatalf("ReloadAt: %v", err)
	}
	if moved.BaseDir() != wt {
		t.Errorf("BaseDir = %q, want %q", moved.BaseDir(), wt)
	}
	if got, want := moved.Locations[0].Location, filepath.Join(wt, "web"); got != want {
		t.Errorf("web = %q, want %q", got, want)
	}
	if want := filepath.Join(projects, "main.yaml"); moved.Path != want {
		t.Errorf("Path = %q, want the global file %q", moved.Path, want)
	}
}

// A reload after a save from inside the re-rooted selector must stay in the
// worktree rather than drifting back to the main checkout.
func TestReloadAt_ReloadStaysInWorktree(t *testing.T) {
	main, wt := worktreeFixture(t, worktreePltrc)
	cfg, err := Load(LoadOptions{Workdir: main})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	moved, err := cfg.ReloadAt(wt)
	if err != nil {
		t.Fatalf("ReloadAt: %v", err)
	}

	again, err := moved.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if again.BaseDir() != wt {
		t.Errorf("BaseDir after Reload = %q, want %q", again.BaseDir(), wt)
	}
	if got, want := again.Locations[0].Location, filepath.Join(wt, "web"); got != want {
		t.Errorf("web = %q, want %q", got, want)
	}
}
