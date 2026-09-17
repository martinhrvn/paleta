package commands

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhrvn/paleta/internal/config"
)

// brokenConfig writes a .pltrc that cannot be parsed, and returns its path.
func brokenConfig(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, config.ConfigFileName)
	if err := os.WriteFile(path, []byte("locations:\n  - name: a\n   location: b\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRunInitWizard_BrokenConfigStops: a config that exists but can't be parsed
// is the user's file. The wizard preserves what is already configured, and it
// can't do that from a file it can't read — so it says which file is broken
// rather than silently rewriting it from a scan.
func TestRunInitWizard_BrokenConfigStops(t *testing.T) {
	dir := chdirTemp(t)
	path := brokenConfig(t, dir)
	writePackage(t, filepath.Join(dir, "packages", "web"), "web")
	called := stubWizard(t, true)

	_, err := RunInitWizard(".", false)
	if err == nil {
		t.Fatal("RunInitWizard() succeeded on an unparseable config")
	}
	var invalid *config.InvalidConfigError
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v (%T), want an *InvalidConfigError naming the file", err, err)
	}
	if invalid.Path != path {
		t.Errorf("Path = %q, want %q", invalid.Path, path)
	}
	if *called {
		t.Error("wizard ran despite the unreadable config")
	}
}

// TestRunInitWizard_ForceStartsFresh: --force is the way out — it ignores the
// broken file and writes a config from the scan.
func TestRunInitWizard_ForceStartsFresh(t *testing.T) {
	dir := chdirTemp(t)
	brokenConfig(t, dir)
	writePackage(t, filepath.Join(dir, "packages", "web"), "web")
	called := stubWizard(t, true)

	outcome, err := RunInitWizard(".", true)
	if err != nil {
		t.Fatalf("RunInitWizard() error = %v", err)
	}
	if outcome != InitWritten {
		t.Errorf("outcome = %v, want InitWritten", outcome)
	}
	if !*called {
		t.Error("wizard was never shown")
	}
	if _, err := config.LoadConfig(filepath.Join(dir, config.ConfigFileName)); err != nil {
		t.Errorf("rewritten config still doesn't load: %v", err)
	}
}
