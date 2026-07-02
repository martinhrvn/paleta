package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLocationFocusedUnmarshal(t *testing.T) {
	yamlStr := `locations:
  - name: "a"
    location: "a"
    focused: true
  - name: "b"
    location: "b"`

	var cfg Config
	if err := yaml.Unmarshal([]byte(yamlStr), &cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(cfg.Locations) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(cfg.Locations))
	}
	if !cfg.Locations[0].Focused {
		t.Errorf("expected location a to be focused")
	}
	if cfg.Locations[1].Focused {
		t.Errorf("expected location b to be unfocused")
	}
}

func TestAnyFocused(t *testing.T) {
	none := &Config{Locations: []Location{{Name: "a"}, {Name: "b"}}}
	if none.AnyFocused() {
		t.Errorf("expected AnyFocused() false when no location is focused")
	}

	some := &Config{Locations: []Location{{Name: "a"}, {Name: "b", Focused: true}}}
	if !some.AnyFocused() {
		t.Errorf("expected AnyFocused() true when a location is focused")
	}
}

// Focused must survive glob expansion so a focused `packages/*` marks every
// expanded child focused.
func TestExpandGlobPatternsPropagatesFocused(t *testing.T) {
	tmp := t.TempDir()
	for _, d := range []string{"packages/frontend", "packages/backend"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	expanded, err := ExpandGlobPatterns([]Location{
		{Name: "svc", Location: "packages/*", Types: Types{"npm"}, Focused: true},
	})
	if err != nil {
		t.Fatalf("expand failed: %v", err)
	}
	if len(expanded) != 2 {
		t.Fatalf("expected 2 expanded locations, got %d", len(expanded))
	}
	for _, loc := range expanded {
		if !loc.Focused {
			t.Errorf("expected expanded location %q to be focused", loc.Location)
		}
	}
}
