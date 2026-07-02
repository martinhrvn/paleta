package commands

import (
	"os"
	"path/filepath"
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

	if err := AddCommandToLocation(path, "web", "packages/web", "ci-and-dev", "pnpm i && pnpm dev"); err != nil {
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
}

func TestAddCommandToLocation_MatchesUnnamedByPath(t *testing.T) {
	path := writePltrc(t, `locations:
  - location: .
    type: go
`)

	if err := AddCommandToLocation(path, ".", ".", "build-test", "go build ./... && go test ./..."); err != nil {
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

	if err := AddCommandToLocation(path, "api", "packages/api", "chain", "a && b"); err != nil {
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

	if err := AddCommandToLocation(path, "web", "packages/web", "chain", "a && b"); err != nil {
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
	if err := AddCommandToLocation(path, "web", "packages/web", "x", "a && b"); err == nil {
		t.Error("expected an error when the config file does not exist")
	}
}
