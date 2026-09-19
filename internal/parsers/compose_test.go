package parsers

import (
	"testing"
)

// composeConfig returns the embedded compose parser configuration.
func composeConfig(t *testing.T) ParserConfig {
	t.Helper()
	defaults, err := loadEmbeddedDefaults()
	if err != nil {
		t.Fatalf("loadEmbeddedDefaults failed: %v", err)
	}
	cfg, ok := defaults.Parsers["compose"]
	if !ok {
		t.Fatal("expected a 'compose' parser in embedded defaults")
	}
	return cfg
}

func TestComposeFilesParser_VariantFilesGetOwnCommands(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "docker-compose.yml")
	touch(t, dir, "docker-compose.dev.yaml")
	touch(t, dir, "compose.prod.yml")

	commands, err := ParseAndFormatCommands(dir, composeConfig(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Plain base commands are untouched.
	if commands["up"] != "docker compose up" {
		t.Errorf("up = %q, want 'docker compose up'", commands["up"])
	}

	want := map[string]string{
		"dev:up":    "docker compose -f docker-compose.dev.yaml up",
		"dev:down":  "docker compose -f docker-compose.dev.yaml down",
		"dev:build": "docker compose -f docker-compose.dev.yaml build",
		"dev:logs":  "docker compose -f docker-compose.dev.yaml logs -f",
		"dev:ps":    "docker compose -f docker-compose.dev.yaml ps",
		"prod:up":   "docker compose -f compose.prod.yml up",
		"prod:ps":   "docker compose -f compose.prod.yml ps",
	}
	for name, cmd := range want {
		if commands[name] != cmd {
			t.Errorf("%s = %q, want %q", name, commands[name], cmd)
		}
	}

	// The plain files must not produce a variant of their own.
	for name := range commands {
		if name == "yml:up" || name == "yaml:up" || name == "docker-compose:up" || name == "compose:up" {
			t.Errorf("unexpected variant command %q derived from a plain compose file", name)
		}
	}
}

func TestComposeFilesParser_PlainFileOnlyYieldsBaseCommands(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "docker-compose.yml")

	cfg := composeConfig(t)
	commands, err := ParseAndFormatCommands(dir, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commands) != len(cfg.BaseCommands) {
		t.Errorf("got %d commands %v, want only the %d base commands", len(commands), commands, len(cfg.BaseCommands))
	}
}

func TestComposeFilesParser_LeavesNonComposeBaseCommandsAlone(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "docker-compose.dev.yml")

	cfg := composeConfig(t)
	cfg.BaseCommands = map[string]string{
		"up":    "docker compose up",
		"shell": "sh ./scripts/shell.sh",
	}
	commands, err := ParseAndFormatCommands(dir, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commands["dev:up"] != "docker compose -f docker-compose.dev.yml up" {
		t.Errorf("dev:up = %q", commands["dev:up"])
	}
	if _, has := commands["dev:shell"]; has {
		t.Error("a base command that is not a docker compose invocation must not get a -f variant")
	}
	if commands["shell"] != "sh ./scripts/shell.sh" {
		t.Errorf("shell = %q, want it preserved", commands["shell"])
	}
}

func TestComposeVariantName(t *testing.T) {
	tests := map[string]string{
		"docker-compose.dev.yaml":       "dev",
		"docker-compose.prod.yml":       "prod",
		"compose.prod.yml":              "prod",
		"compose.staging.yaml":          "staging",
		"docker-compose.dev.local.yaml": "dev.local",
	}
	for file, want := range tests {
		if got := composeVariantName(file); got != want {
			t.Errorf("composeVariantName(%q) = %q, want %q", file, got, want)
		}
	}
}
