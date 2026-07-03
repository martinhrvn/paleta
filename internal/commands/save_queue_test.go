package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePltrc(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".pltrc")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAddCommandToLocation_AppendsToNamedLocation(t *testing.T) {
	path := writePltrc(t, `locations:
  - name: web
    location: packages/web
    type: npm
`)

	if err := AddCommandToLocation(path, "web", "packages/web", "ci-and-dev", []string{"pnpm i", "pnpm dev"}); err != nil {
		t.Fatalf("AddCommandToLocation failed: %v", err)
	}

	cfg, err := LoadAuthoredConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Locations) != 1 {
		t.Fatalf("expected 1 location, got %d", len(cfg.Locations))
	}
	cmds := cfg.Locations[0].Commands
	if len(cmds) != 1 {
		t.Fatalf("expected 1 authored command, got %d", len(cmds))
	}
	if cmds[0].Name != "ci-and-dev" || cmds[0].Command != "pnpm i && pnpm dev" {
		t.Errorf("unexpected saved command: %+v", cmds[0])
	}

	// A multi-part chain is stored as a YAML list for readability.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "command:\n") || !strings.Contains(string(raw), "- pnpm i") {
		t.Errorf("expected the chain to be written as a YAML list, got:\n%s", raw)
	}
}

func TestAddCommandToLocation_SinglePartStaysScalar(t *testing.T) {
	path := writePltrc(t, `locations:
  - name: web
    location: packages/web
`)

	if err := AddCommandToLocation(path, "web", "packages/web", "dev", []string{"pnpm dev"}); err != nil {
		t.Fatalf("AddCommandToLocation failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "- pnpm dev") {
		t.Errorf("expected a single command to stay scalar, got:\n%s", raw)
	}

	cfg, _ := LoadAuthoredConfig(path)
	if cmds := cfg.Locations[0].Commands; len(cmds) != 1 || cmds[0].Command != "pnpm dev" {
		t.Errorf("unexpected saved command: %+v", cfg.Locations[0].Commands)
	}
}

func TestAddCommandToLocation_MatchesUnnamedByPath(t *testing.T) {
	path := writePltrc(t, `locations:
  - location: .
    type: go
`)

	if err := AddCommandToLocation(path, ".", ".", "build-test", []string{"go build ./...", "go test ./..."}); err != nil {
		t.Fatalf("AddCommandToLocation failed: %v", err)
	}

	cfg, _ := LoadAuthoredConfig(path)
	if len(cfg.Locations) != 1 {
		t.Fatalf("expected the command to append to the existing location, got %d locations", len(cfg.Locations))
	}
	if len(cfg.Locations[0].Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cfg.Locations[0].Commands))
	}
}

func TestAddCommandToLocation_PreservesExistingCommands(t *testing.T) {
	path := writePltrc(t, `locations:
  - name: api
    location: packages/api
    commands:
      - go run .
`)

	if err := AddCommandToLocation(path, "api", "packages/api", "chain", []string{"a", "b"}); err != nil {
		t.Fatalf("AddCommandToLocation failed: %v", err)
	}

	cfg, _ := LoadAuthoredConfig(path)
	cmds := cfg.Locations[0].Commands
	if len(cmds) != 2 {
		t.Fatalf("expected existing + new command (2), got %d", len(cmds))
	}
	if cmds[0].Command != "go run ." {
		t.Errorf("expected existing command preserved, got %q", cmds[0].Command)
	}
	if cmds[1].Name != "chain" || cmds[1].Command != "a && b" {
		t.Errorf("unexpected appended command: %+v", cmds[1])
	}
}

func TestAddCommandToLocation_GlobFallbackCreatesLocation(t *testing.T) {
	// The queued command resolved to packages/web, but the authored config only
	// has a glob entry that doesn't match by path — a new location is created.
	path := writePltrc(t, `locations:
  - location: "packages/*"
    type: npm
`)

	if err := AddCommandToLocation(path, "web", "packages/web", "chain", []string{"a", "b"}); err != nil {
		t.Fatalf("AddCommandToLocation failed: %v", err)
	}

	cfg, _ := LoadAuthoredConfig(path)
	if len(cfg.Locations) != 2 {
		t.Fatalf("expected a new location appended (2 total), got %d", len(cfg.Locations))
	}
	newLoc := cfg.Locations[1]
	if newLoc.Location != "packages/web" || len(newLoc.Commands) != 1 || newLoc.Commands[0].Name != "chain" {
		t.Errorf("unexpected new location: %+v", newLoc)
	}
}

func TestAddCommandToLocation_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".pltrc")
	if err := AddCommandToLocation(path, "web", "packages/web", "x", []string{"a", "b"}); err == nil {
		t.Error("expected an error when the config file does not exist")
	}
}
