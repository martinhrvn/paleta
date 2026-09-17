package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/scan"
)

// TestInitWizardChain_CollapsedGlobRoundTrips runs the collapse the wizard applies
// through the same chain as TestInitWizardChain: scan a temp monorepo, write the
// glob form, and load it back. The written config has to expand into exactly the
// folders that were ticked — with the unticked one excluded.
func TestInitWizardChain_CollapsedGlobRoundTrips(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b", "c", "legacy"} {
		dir := filepath.Join(root, "packages", name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		pkg := `{"name":"` + name + `","scripts":{"build":"vite build"}}`
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
			t.Fatal(err)
		}
	}

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

	var selected, deselected []config.Location
	for _, it := range BuildWizardItems(cands, nil) {
		if filepath.Base(it.Location.Location) == "legacy" {
			deselected = append(deselected, it.Location)
			continue
		}
		selected = append(selected, it.Location)
	}

	content := writeGenerated(t, ".pltrc", config.CollapseSiblingsToGlobs(selected, deselected))
	if !strings.Contains(content, "packages/*") {
		t.Fatalf("generated config has no glob:\n%s", content)
	}

	loaded, err := config.LoadConfig(".pltrc")
	if err != nil {
		t.Fatalf("LoadConfig failed on generated file:\n%s\nerror: %v", content, err)
	}

	var names []string
	for _, loc := range loaded.Locations {
		names = append(names, loc.Name)
		if len(loc.Commands) == 0 {
			t.Errorf("location %q has no commands; the glob dropped its type", loc.Name)
		}
	}
	if strings.Join(names, ",") != "a,b,c" {
		t.Errorf("expanded locations = %v, want the three ticked folders", names)
	}
	if len(loaded.Warnings) > 0 {
		t.Errorf("generated config produced warnings: %+v", loaded.Warnings)
	}
}
