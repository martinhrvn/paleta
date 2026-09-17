package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/ui"
)

// stubWizard replaces the interactive step for one test, recording whether it was
// reached. confirm chooses between "user pressed Enter" and "user cancelled".
func stubWizard(t *testing.T, confirm bool) *bool {
	t.Helper()
	called := false
	prev := runWizard
	runWizard = func(items []ui.WizardItem) ([]config.Location, bool, error) {
		called = true
		if !confirm {
			return nil, false, nil
		}
		locs := make([]config.Location, 0, len(items))
		for _, it := range items {
			locs = append(locs, it.Location)
		}
		return locs, true, nil
	}
	t.Cleanup(func() { runWizard = prev })
	return &called
}

// chdirTemp moves into a fresh directory for the duration of the test, with $HOME
// pointed somewhere else so the home guard doesn't fire.
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	// os.Getwd resolves symlinks (/tmp is one on some systems), so compare
	// against what the code under test will actually see.
	resolved, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func writePackage(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	pkg := `{"name":"` + name + `","scripts":{"build":"vite build"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestBootstrap_NoConfigRunsWizard: the first `plt` in a repo with no .pltrc is
// the whole point — scan, ask, write, carry on into the selector.
func TestBootstrap_NoConfigRunsWizard(t *testing.T) {
	dir := chdirTemp(t)
	writePackage(t, filepath.Join(dir, "packages", "web"), "web")
	called := stubWizard(t, true)

	outcome, err := Bootstrap()
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if outcome != BootstrapWritten {
		t.Errorf("outcome = %v, want BootstrapWritten", outcome)
	}
	if !*called {
		t.Error("wizard was never shown")
	}
	if _, err := os.Stat(filepath.Join(dir, config.ConfigFileName)); err != nil {
		t.Errorf("no .pltrc written: %v", err)
	}
}

// TestBootstrap_RefusesHomeAndRoot: `plt` in $HOME (or above it) must not start
// walking the user's whole home directory. `plt init` has always scanned the cwd,
// but only when asked for by name.
func TestBootstrap_RefusesHomeAndRoot(t *testing.T) {
	t.Run("home", func(t *testing.T) {
		dir := chdirTemp(t)
		t.Setenv("HOME", dir)
		writePackage(t, filepath.Join(dir, "packages", "web"), "web")
		called := stubWizard(t, true)

		outcome, err := Bootstrap()
		if err != nil {
			t.Fatalf("Bootstrap() error = %v", err)
		}
		if outcome != BootstrapSkipped {
			t.Errorf("outcome = %v, want BootstrapSkipped", outcome)
		}
		if *called {
			t.Error("wizard ran in $HOME")
		}
	})

	t.Run("above home", func(t *testing.T) {
		dir := chdirTemp(t)
		t.Setenv("HOME", filepath.Join(dir, "user"))
		called := stubWizard(t, true)

		outcome, _ := Bootstrap()
		if outcome != BootstrapSkipped {
			t.Errorf("outcome = %v, want BootstrapSkipped", outcome)
		}
		if *called {
			t.Error("wizard ran in a parent of $HOME")
		}
	})

	t.Run("root", func(t *testing.T) {
		chdirTemp(t)
		if err := os.Chdir("/"); err != nil {
			t.Skipf("cannot chdir to /: %v", err)
		}
		called := stubWizard(t, true)

		outcome, _ := Bootstrap()
		if outcome != BootstrapSkipped {
			t.Errorf("outcome = %v, want BootstrapSkipped", outcome)
		}
		if *called {
			t.Error("wizard ran at the filesystem root")
		}
	})
}

// TestBootstrap_ExistingConfigUntouched: bootstrapping is for an empty slate. A
// config that exists but failed to load must never be overwritten by the wizard.
func TestBootstrap_ExistingConfigUntouched(t *testing.T) {
	dir := chdirTemp(t)
	writePackage(t, filepath.Join(dir, "packages", "web"), "web")
	existing := []byte("locations:\n  - name: hand-written\n    location: .\n")
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), existing, 0644); err != nil {
		t.Fatal(err)
	}
	called := stubWizard(t, true)

	outcome, err := Bootstrap()
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if outcome != BootstrapSkipped {
		t.Errorf("outcome = %v, want BootstrapSkipped", outcome)
	}
	if *called {
		t.Error("wizard ran with a config already present")
	}
	got, err := os.ReadFile(filepath.Join(dir, config.ConfigFileName))
	if err != nil || string(got) != string(existing) {
		t.Errorf("config = %q, want it untouched", string(got))
	}
}

// TestBootstrap_CancelLeavesNoFile: Esc out of the wizard and nothing is written.
func TestBootstrap_CancelLeavesNoFile(t *testing.T) {
	dir := chdirTemp(t)
	writePackage(t, filepath.Join(dir, "packages", "web"), "web")
	stubWizard(t, false)

	outcome, err := Bootstrap()
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if outcome != BootstrapCanceled {
		t.Errorf("outcome = %v, want BootstrapCanceled", outcome)
	}
	if _, err := os.Stat(filepath.Join(dir, config.ConfigFileName)); !os.IsNotExist(err) {
		t.Errorf("a .pltrc was written after cancelling (stat err = %v)", err)
	}
}

// TestBootstrap_NoProjectsSkipsWizard: an empty directory has nothing to offer,
// so the caller falls back to the plain hint rather than an empty list.
func TestBootstrap_NoProjectsSkipsWizard(t *testing.T) {
	chdirTemp(t)
	called := stubWizard(t, true)

	outcome, err := Bootstrap()
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if outcome != BootstrapNoProjects {
		t.Errorf("outcome = %v, want BootstrapNoProjects", outcome)
	}
	if *called {
		t.Error("wizard ran with nothing detected")
	}
}
