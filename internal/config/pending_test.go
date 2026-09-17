package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// makeProject writes a .pltrc and a Makefile into a temp dir, chdirs there, and
// returns the config path. The `make` parser is pointed at parserCommand.
func makeProject(t *testing.T, parserCommand, configYAML string) string {
	t.Helper()
	withMakeParserCommand(t, parserCommand)

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte("build:\n\techo hi\n"), 0644); err != nil {
		t.Fatalf("write Makefile: %v", err)
	}
	configPath := filepath.Join(tmpDir, ".pltrc")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldWd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(oldWd) })
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return configPath
}

func commandNamesOf(cmds []Command) []string {
	names := make([]string, len(cmds))
	for i, c := range cmds {
		names[i] = c.Name
	}
	return names
}

// TestProcessProjectTypes_DefersShellOutTypes: a type whose commands come from a
// shell command is left pending at load, so opening the palette never waits on it.
func TestProcessProjectTypes_DefersShellOutTypes(t *testing.T) {
	configPath := makeProject(t, "\"printf 'test\\\\nlint\\\\n'\"", "locations:\n  - name: infra\n    location: \".\"\n    type: make\n    commands:\n      - name: deploy\n        command: \"./deploy.sh\"\n")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	loc := cfg.Locations[0]
	if !reflect.DeepEqual(loc.PendingTypes, []string{"make"}) {
		t.Errorf("PendingTypes = %v, want [make]", loc.PendingTypes)
	}
	if got := commandNamesOf(loc.Commands); !reflect.DeepEqual(got, []string{"deploy"}) {
		t.Errorf("commands = %v, want only the authored [deploy] until the type resolves", got)
	}
}

// TestResolveAllPending_FillsCommands: resolving produces exactly what a blocking
// load would have produced — same commands, same order — and clears the pending set.
func TestResolveAllPending_FillsCommands(t *testing.T) {
	configPath := makeProject(t, "\"printf 'test\\\\nlint\\\\n'\"", "locations:\n  - name: infra\n    location: \".\"\n    type: make\n    commands:\n      - name: deploy\n        command: \"./deploy.sh\"\n")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	ResolveAllPending(cfg)

	loc := cfg.Locations[0]
	if len(loc.PendingTypes) != 0 {
		t.Errorf("PendingTypes = %v, want empty after resolution", loc.PendingTypes)
	}
	// Authored first, then the type's commands sorted by name (base + parsed).
	want := []string{"deploy", "build", "lint", "test"}
	if got := commandNamesOf(loc.Commands); !reflect.DeepEqual(got, want) {
		t.Errorf("commands = %v, want %v", got, want)
	}
}

// TestResolvePending_AppliesCommandFilters: filters must apply to late-arriving
// commands too, not just the ones present at load.
func TestResolvePending_AppliesCommandFilters(t *testing.T) {
	configPath := makeProject(t, "\"printf 'test\\\\nlint\\\\n'\"", "locations:\n  - name: infra\n    location: \".\"\n    type: make\n    exclude_commands: [\"lint\"]\n")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	ResolveAllPending(cfg)

	got := commandNamesOf(cfg.Locations[0].Commands)
	for _, name := range got {
		if name == "lint" {
			t.Errorf("commands = %v, want lint excluded", got)
		}
	}
	if len(got) != 2 {
		t.Errorf("commands = %v, want build and test", got)
	}
}

// TestResolvePending_ReexpandsAliases: an @project:command reference pointing at a
// deferred command resolves once that command arrives.
func TestResolvePending_ReexpandsAliases(t *testing.T) {
	configYAML := "locations:\n  - name: infra\n    location: \".\"\n    type: make\n  - name: root\n    location: \".\"\n    commands:\n      - name: ci\n        command: \"@infra[make]:test\"\n"
	configPath := makeProject(t, "\"printf 'test\\\\n'\"", configYAML)

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	ResolveAllPending(cfg)

	var ci *Command
	for i, cmd := range cfg.Locations[1].Commands {
		if cmd.Name == "ci" {
			ci = &cfg.Locations[1].Commands[i]
		}
	}
	if ci == nil {
		t.Fatalf("ci command missing: %+v", cfg.Locations[1].Commands)
	}
	if ci.Error != "" {
		t.Errorf("ci.Error = %q, want the reference resolved after pending types loaded", ci.Error)
	}
	if ci.Command == "@infra[make]:test" {
		t.Errorf("ci.Command = %q, want the reference expanded", ci.Command)
	}
}

// TestLoadConfig_CheapTypesUnaffected: types that read a file (or need no I/O at
// all) still resolve during the load, so nothing about them changes.
func TestLoadConfig_CheapTypesUnaffected(t *testing.T) {
	tmpDir := t.TempDir()
	pkg := `{"name":"web","scripts":{"build":"tsc"}}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	configPath := filepath.Join(tmpDir, ".pltrc")
	if err := os.WriteFile(configPath, []byte("locations:\n  - name: web\n    location: \".\"\n    type: npm\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Locations[0].PendingTypes) != 0 {
		t.Errorf("PendingTypes = %v, want none for npm", cfg.Locations[0].PendingTypes)
	}
	// npm contributes its base commands plus the package.json script, all at load.
	got := commandNamesOf(cfg.Locations[0].Commands)
	var hasBuild bool
	for _, name := range got {
		if name == "build" {
			hasBuild = true
		}
	}
	if !hasBuild {
		t.Errorf("commands = %v, want the package.json script resolved at load", got)
	}
}
