package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetFocusedPersistsAndUnsets(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".pltrc")

	initial := `locations:
  - name: "frontend"
    location: "packages/frontend"
    commands:
      - "npm run dev"
  - name: "backend"
    location: "packages/backend"
    focused: true
    commands:
      - "npm start"
`
	if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	// Focus frontend, unfocus backend.
	if err := SetFocused(configPath, map[string]bool{"frontend": true, "backend": false}); err != nil {
		t.Fatalf("SetFocused failed: %v", err)
	}

	authored, err := LoadAuthoredConfig(configPath)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	byName := map[string]bool{}
	for _, loc := range authored.Locations {
		byName[loc.Name] = loc.Focused
	}
	if !byName["frontend"] {
		t.Error("expected frontend to be focused after SetFocused")
	}
	if byName["backend"] {
		t.Error("expected backend to be unfocused after SetFocused")
	}

	// Hand-authored commands must survive the round-trip.
	if len(authored.Locations) != 2 || len(authored.Locations[0].Commands) != 1 {
		t.Errorf("commands were not preserved: %+v", authored.Locations)
	}
}

func TestFocusEntriesReflectsConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".pltrc")

	content := `locations:
  - name: "a"
    location: "a"
    focused: true
  - name: "b"
    location: "b"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := FocusEntries(configPath)
	if err != nil {
		t.Fatalf("FocusEntries failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	got := map[string]bool{}
	for _, e := range entries {
		got[e.Key] = e.Focused
	}
	if !got["a"] {
		t.Error("expected entry a to be focused")
	}
	if got["b"] {
		t.Error("expected entry b to be unfocused")
	}
}
