package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhrvn/paleta/internal/projecttypes"
)

// withMakeParserCommand points the `make` parser at a given shell command for the
// duration of a test, by writing a user parsers override and re-reading it.
func withMakeParserCommand(t *testing.T, parserCommand string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".paleta")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	override := "parsers:\n  make:\n    detect_files: [\"Makefile\"]\n    base_commands:\n      build: \"make build\"\n    parser_command: " + parserCommand + "\n"
	if err := os.WriteFile(filepath.Join(dir, "parsers.yaml"), []byte(override), 0644); err != nil {
		t.Fatalf("write parsers.yaml: %v", err)
	}
	projecttypes.ReloadRegistry()
	t.Cleanup(projecttypes.ReloadRegistry)
}

// TestProcessProjectTypes_ParserFailureDegradesToWarning: a broken parser command
// must not fail the whole config load — the location keeps its base commands and
// the problem is reported as a warning.
func TestProcessProjectTypes_ParserFailureDegradesToWarning(t *testing.T) {
	withMakeParserCommand(t, "\"exit 3\"")

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte("build:\n\techo hi\n"), 0644); err != nil {
		t.Fatalf("write Makefile: %v", err)
	}
	configYAML := "locations:\n  - name: infra\n    location: \".\"\n    type: make\n"
	configPath := filepath.Join(tmpDir, ".pltrc")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() failed, want a degraded load: %v", err)
	}
	// `make` shells out, so it resolves after the load — that's where the parser
	// failure surfaces.
	ResolveAllPending(cfg)

	var names []string
	for _, cmd := range cfg.Locations[0].Commands {
		names = append(names, cmd.Name)
	}
	if len(names) != 1 || names[0] != "build" {
		t.Errorf("commands = %v, want the base command [build] to survive", names)
	}

	var parserWarnings []Warning
	for _, w := range cfg.Warnings {
		if w.Kind == "parser" {
			parserWarnings = append(parserWarnings, w)
		}
	}
	if len(parserWarnings) != 1 {
		t.Fatalf("warnings = %+v, want one parser warning", cfg.Warnings)
	}
	if !strings.Contains(parserWarnings[0].Context, "infra") {
		t.Errorf("warning context = %q, want it to name the location", parserWarnings[0].Context)
	}
}
