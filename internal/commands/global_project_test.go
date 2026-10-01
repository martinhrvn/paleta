package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhrvn/paleta/internal/config"
)

// globalProjectsDir is where a test's global project files live under the $HOME
// chdirTemp set up.
func globalProjectsDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(os.Getenv("HOME"), ".config", "paleta", "projects")
}

// writeGlobalProject stores a global project file for root and returns its path.
func writeGlobalProject(t *testing.T, name, root, body string) string {
	t.Helper()
	dir := globalProjectsDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("root: "+root+"\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// plt edit opens the global project file when the repo has no .pltrc.
func TestFindConfigForEdit_GlobalProject(t *testing.T) {
	root := chdirTemp(t)
	path := writeGlobalProject(t, "repo.yaml", root, "locations: []\n")

	got, err := FindConfigForEdit()
	if err != nil {
		t.Fatalf("FindConfigForEdit() error = %v", err)
	}
	if got != path {
		t.Errorf("FindConfigForEdit() = %q, want %q", got, path)
	}
}

// A global project config gets the same selector persistence as a .pltrc, with
// cross-folder queue saves anchored at the project root, not the projects dir.
func TestSelectorBackend_GlobalProject(t *testing.T) {
	root := chdirTemp(t)
	writeGlobalProject(t, "repo.yaml", root, "locations:\n  - location: \".\"\n    commands: [\"build\"]\n")

	cfg, err := config.Load(config.LoadOptions{Workdir: root, Defer: true})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	be := selectorBackend(cfg)
	if be.SaveFocus == nil || be.SaveQueue == nil || be.ListFocus == nil {
		t.Fatal("focus and queue persistence should be enabled for a global project")
	}
	if be.RootDir != root {
		t.Errorf("RootDir = %q, want %q", be.RootDir, root)
	}
}

// Re-running the wizard on a global project (Ctrl+N) scans the project root and
// edits the global file in place, keeping its root.
func TestRunInitWizard_GlobalProjectFile(t *testing.T) {
	root := chdirTemp(t)
	writePackage(t, filepath.Join(root, "web"), "web")
	path := writeGlobalProject(t, "repo.yaml", root, "# mine\nlocations: []\n")
	stubWizard(t, true)

	outcome, err := RunInitWizard(root, path, false)
	if err != nil || outcome != InitWritten {
		t.Fatalf("RunInitWizard() = %v, %v; want InitWritten", outcome, err)
	}
	written, _ := os.ReadFile(path)
	for _, want := range []string{"root: " + root, "# mine", "web"} {
		if !strings.Contains(string(written), want) {
			t.Errorf("missing %q in:\n%s", want, written)
		}
	}
	if _, err := os.Stat(filepath.Join(root, config.ConfigFileName)); !os.IsNotExist(err) {
		t.Error("wrote a .pltrc into the repo")
	}
}

func TestRunInitGlobal(t *testing.T) {
	t.Run("writes the global project file", func(t *testing.T) {
		root := chdirTemp(t)
		writePackage(t, filepath.Join(root, "web"), "web")
		stubWizard(t, true)

		var out, errOut bytes.Buffer
		if code := Run("dev", []string{"init", "--global"}, &out, &errOut); code != 0 {
			t.Fatalf("exit %d, stderr: %s", code, errOut.String())
		}
		path := filepath.Join(globalProjectsDir(t), filepath.Base(root)+".yaml")
		written, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("no global project file: %v", err)
		}
		if !strings.Contains(string(written), "root: "+root) || !strings.Contains(string(written), "web") {
			t.Errorf("unexpected content:\n%s", written)
		}
		if !strings.Contains(out.String(), path) {
			t.Errorf("output does not name %s:\n%s", path, out.String())
		}
		if _, err := os.Stat(filepath.Join(root, config.ConfigFileName)); !os.IsNotExist(err) {
			t.Error("wrote a .pltrc into the repo")
		}
	})

	t.Run("refuses when a .pltrc would shadow it", func(t *testing.T) {
		root := chdirTemp(t)
		if err := os.WriteFile(filepath.Join(root, config.ConfigFileName), []byte("locations: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		called := stubWizard(t, true)

		var out, errOut bytes.Buffer
		if code := Run("dev", []string{"init", "--global"}, &out, &errOut); code == 0 {
			t.Fatal("expected a non-zero exit")
		}
		if *called {
			t.Error("wizard ran")
		}
		if !strings.Contains(errOut.String(), config.ConfigFileName) {
			t.Errorf("stderr should explain the .pltrc: %s", errOut.String())
		}
	})

	t.Run("refuses with --template", func(t *testing.T) {
		chdirTemp(t)
		var out, errOut bytes.Buffer
		if code := Run("dev", []string{"init", "--global", "--template"}, &out, &errOut); code == 0 {
			t.Fatal("expected a non-zero exit")
		}
	})
}
