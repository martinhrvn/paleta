package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/scan"
)

// TestInitWizardChain exercises the full non-UI init pipeline end to end:
// scan a temp monorepo -> build items -> generate .pltrc -> load it back and
// confirm the detected project types expand into real commands.
func TestInitWizardChain(t *testing.T) {
	root := t.TempDir()

	// A Go project at the root and an npm package in a subdir.
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\n\ngo 1.24\n"), 0644); err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Join(root, "packages", "web")
	if err := os.MkdirAll(webDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "package.json"),
		[]byte(`{"name":"web","scripts":{"dev":"vite","build":"vite build"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	// config.LoadConfig resolves relative paths against the cwd, so run from root.
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	cands, err := scan.Scan(".")
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	items := BuildWizardItems(cands, nil)
	locations := make([]config.Location, len(items))
	for i, it := range items {
		locations[i] = it.Location
	}

	content := GenerateConfig(locations, nil)
	if err := WriteConfig(".pltrc", content); err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	loaded, err := config.LoadConfig(".pltrc")
	if err != nil {
		t.Fatalf("LoadConfig failed on generated file:\n%s\nerror: %v", content, err)
	}

	byName := map[string]config.Location{}
	for _, loc := range loaded.Locations {
		byName[loc.Name] = loc
	}

	rootLoc, ok := byName["root"]
	if !ok {
		t.Fatal("expected a 'root' location in generated config")
	}
	if len(rootLoc.Types) != 1 || rootLoc.Types[0] != "go" {
		t.Errorf("root types = %v, want [go]", rootLoc.Types)
	}
	if !hasCommandContaining(rootLoc.Commands, "build") || !hasCommandContaining(rootLoc.Commands, "test") {
		t.Errorf("expected go commands for root, got %+v", rootLoc.Commands)
	}

	webLoc, ok := byName["web"]
	if !ok {
		t.Fatal("expected a 'web' location in generated config")
	}
	if len(webLoc.Types) != 1 || webLoc.Types[0] != "npm" {
		t.Errorf("web types = %v, want [npm]", webLoc.Types)
	}
	if !hasCommandContaining(webLoc.Commands, "npm run dev") {
		t.Errorf("expected npm scripts discovered for web, got %+v", webLoc.Commands)
	}
}

func hasCommandContaining(cmds []config.Command, substr string) bool {
	for _, c := range cmds {
		if containsSubstring(c.Command, substr) || containsSubstring(c.Name, substr) {
			return true
		}
	}
	return false
}
