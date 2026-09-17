package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeLoadFixture creates a directory holding a .pltrc with the given content.
func writeLoadFixture(t *testing.T, pltrc string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(pltrc), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Load assembles a complete config in one call: the discovered path recorded,
// deferred types resolved, tools attached — without the caller sequencing
// anything or changing directory.
func TestLoad_AssemblesCompleteConfig(t *testing.T) {
	dir := writeLoadFixture(t, "locations:\n  - name: app\n    location: .\n    commands: [\"echo hi\"]\ntools:\n  - lazygit\n")

	cfg, err := Load(LoadOptions{Workdir: dir})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := filepath.Join(dir, ConfigFileName); cfg.Path != want {
		t.Errorf("Path = %q, want %q", cfg.Path, want)
	}
	if cfg.HasPendingTypes() {
		t.Error("expected no pending types after a non-deferred load")
	}
	if len(cfg.ResolvedTools) != 1 {
		t.Fatalf("ResolvedTools = %+v, want the enabled lazygit row", cfg.ResolvedTools)
	}
	if cfg.ResolvedTools[0].Directory != dir {
		t.Errorf("tool directory = %q, want the workdir %q", cfg.ResolvedTools[0].Directory, dir)
	}
	if cfg.Locations[0].Location != dir {
		t.Errorf("location = %q, want it resolved against the config dir %q", cfg.Locations[0].Location, dir)
	}
}

// Discovery walks up from Workdir; globs resolve against the config's directory
// while tools keep running where the user is.
func TestLoad_FromSubdirectory(t *testing.T) {
	dir := writeLoadFixture(t, "locations:\n  - location: packages/*\ntools:\n  - lazygit\n")
	deep := filepath.Join(dir, "packages", "b", "deep")
	for _, p := range []string{filepath.Join(dir, "packages", "a"), deep} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := Load(LoadOptions{Workdir: deep})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Locations) != 2 {
		t.Fatalf("locations = %+v, want the two glob children", cfg.Locations)
	}
	if got, want := cfg.Locations[0].Location, filepath.Join(dir, "packages", "a"); got != want {
		t.Errorf("first child = %q, want %q", got, want)
	}
	if cfg.ResolvedTools[0].Directory != deep {
		t.Errorf("tool directory = %q, want the workdir %q", cfg.ResolvedTools[0].Directory, deep)
	}
}

// Defer leaves shell-backed types for the caller to resolve in the background.
func TestLoad_DeferKeepsPendingTypes(t *testing.T) {
	dir := writeLoadFixture(t, "locations:\n  - location: .\n    type: make\n")
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte("build:\n\t@echo build\n"), 0644); err != nil {
		t.Fatal(err)
	}

	deferred, err := Load(LoadOptions{Workdir: dir, Defer: true})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !deferred.HasPendingTypes() {
		t.Fatal("expected the make type to stay pending under Defer")
	}

	full, err := Load(LoadOptions{Workdir: dir})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if full.HasPendingTypes() {
		t.Fatal("expected a non-deferred load to resolve every type")
	}
}

// Reload re-runs the load that produced the config, so a re-read after a save is
// just as complete as the original: same path, tools attached, new content.
func TestConfig_Reload_KeepsToolsAndPath(t *testing.T) {
	dir := writeLoadFixture(t, "locations:\n  - name: app\n    location: .\n    commands: [\"echo hi\"]\ntools:\n  - lazygit\n")
	cfg, err := Load(LoadOptions{Workdir: dir, Defer: true})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	rewritten := "locations:\n  - name: app\n    location: .\n    commands: [\"echo hi\", \"echo again\"]\ntools:\n  - lazygit\n"
	if err := os.WriteFile(cfg.Path, []byte(rewritten), 0644); err != nil {
		t.Fatal(err)
	}

	again, err := cfg.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if again.Path != cfg.Path {
		t.Errorf("Path = %q, want %q", again.Path, cfg.Path)
	}
	if len(again.ResolvedTools) != 1 {
		t.Errorf("ResolvedTools = %+v, want the lazygit row to survive the reload", again.ResolvedTools)
	}
	if len(again.Locations[0].Commands) != 2 {
		t.Errorf("commands = %+v, want the rewritten file's two commands", again.Locations[0].Commands)
	}
}

// An unknown-tool warning must survive a later warning rebuild, and attaching
// twice must not repeat it.
func TestAttachTools_WarningSurvivesResolveAllPending(t *testing.T) {
	cfg := &Config{
		Tools:     ToolsField{Enabled: []string{"no-such-tool"}},
		Locations: []Location{{Location: "/tmp/x", PendingTypes: []string{"bogus"}}},
	}
	AttachTools(cfg, "/tmp")
	AttachTools(cfg, "/tmp")
	ResolveAllPending(cfg)

	toolWarnings := 0
	for _, w := range cfg.Warnings {
		if w.Kind == "tool" {
			toolWarnings++
		}
	}
	if toolWarnings != 1 {
		t.Errorf("tool warnings = %d, want exactly 1; warnings: %+v", toolWarnings, cfg.Warnings)
	}
}

// A global project's relative paths and globs resolve against its declared root,
// not the directory plt happened to be run from.
func TestLoad_GlobalProjectResolvesAgainstRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	projectsDir := filepath.Join(home, ".config", "paleta", "projects")
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	sub := filepath.Join(root, "packages", "web", "src")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	project := "root: " + root + "\nlocations:\n  - location: packages/*\n"
	if err := os.WriteFile(filepath.Join(projectsDir, "proj.yaml"), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(LoadOptions{Workdir: sub})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Path != "" {
		t.Errorf("Path = %q, want empty for a global project", cfg.Path)
	}
	if len(cfg.Locations) != 1 || cfg.Locations[0].Location != filepath.Join(root, "packages", "web") {
		t.Errorf("locations = %+v, want the glob expanded under the project root", cfg.Locations)
	}
}

// A missing configuration is reported with ErrConfigNotFound; a broken one names
// its file.
func TestLoad_Errors(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, err := Load(LoadOptions{Workdir: t.TempDir()}); !errors.Is(err, ErrConfigNotFound) {
		t.Errorf("no config: err = %v, want ErrConfigNotFound", err)
	}

	broken := writeLoadFixture(t, "locations: [\n")
	_, err := Load(LoadOptions{Workdir: broken})
	var invalid *InvalidConfigError
	if !errors.As(err, &invalid) {
		t.Fatalf("broken config: err = %v, want *InvalidConfigError", err)
	}
	if want := filepath.Join(broken, ConfigFileName); invalid.Path != want {
		t.Errorf("Path = %q, want %q", invalid.Path, want)
	}
}
