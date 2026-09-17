package parsers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry_Lookup(t *testing.T) {
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry: %v", err)
	}
	for _, name := range []string{"npm", "yarn", "pnpm", "go", "compose"} {
		typ, ok := reg.Lookup(name)
		if !ok || typ.Name() != name {
			t.Errorf("Lookup(%q) = %v, %v", name, typ, ok)
		}
	}
	if _, ok := reg.Lookup("unknown"); ok {
		t.Error("Lookup(unknown) should report no such type")
	}
}

// Types come back in priority order, so the caller that ranks a folder's types
// (the wizard scan) and the one that resolves declared types (the loader) agree.
func TestRegistry_TypesAreOrderedByPriority(t *testing.T) {
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, typ := range reg.Types() {
		names = append(names, typ.Name())
	}
	index := func(name string) int {
		for i, n := range names {
			if n == name {
				return i
			}
		}
		t.Fatalf("type %q missing from %v", name, names)
		return -1
	}
	if !(index("go") < index("npm") && index("npm") < index("docker") && index("compose") < index("docker")) {
		t.Errorf("types not in priority order: %v", names)
	}
}

// A user parser without a priority sorts after the built-ins; one with a
// priority slots in where it says.
func TestNewRegistry_PriorityFromConfig(t *testing.T) {
	reg := NewRegistry(&ParsersFile{Parsers: map[string]ParserConfig{
		"zig":  {DetectFiles: []string{"build.zig"}},
		"go":   {DetectFiles: []string{"go.mod"}, Priority: 10},
		"nix":  {DetectFiles: []string{"flake.nix"}, Priority: 5},
		"make": {DetectFiles: []string{"Makefile"}, Priority: 90},
	}})
	var names []string
	for _, typ := range reg.Types() {
		names = append(names, typ.Name())
	}
	want := []string{"nix", "go", "make", "zig"}
	if len(names) != len(want) {
		t.Fatalf("types = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("types = %v, want %v", names, want)
		}
	}
}

func TestType_CanHandleDirectory_Glob(t *testing.T) {
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	compose, _ := reg.Lookup("compose")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.prod.yml"), []byte("services: {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if !compose.CanHandleDirectory(dir) {
		t.Error("compose should handle a dir with docker-compose.prod.yml")
	}
	if compose.CanHandleDirectory(t.TempDir()) {
		t.Error("compose should not handle an empty dir")
	}
}

// DetectTypes lists every type whose detect files are present, in priority
// order, with package.json refined to the package manager the lockfile names.
func TestRegistry_DetectTypes(t *testing.T) {
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, f := range []string{"package.json", "pnpm-lock.yaml", "Dockerfile"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got := reg.DetectTypes(dir)
	want := []string{"pnpm", "docker"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("DetectTypes = %v, want %v", got, want)
	}
	if got := reg.DetectTypes(t.TempDir()); len(got) != 0 {
		t.Errorf("DetectTypes(empty) = %v, want none", got)
	}
}

func TestType_DefersLoading(t *testing.T) {
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if m, _ := reg.Lookup("make"); !m.DefersLoading() {
		t.Error("make runs a parser command, so it should defer")
	}
	if n, _ := reg.Lookup("npm"); n.DefersLoading() {
		t.Error("npm reads package.json, so it should not defer")
	}
}
