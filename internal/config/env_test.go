package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEffectiveEnvMergePrecedence(t *testing.T) {
	loc := Location{Env: map[string]string{
		"PORT":  "3000",
		"DEBUG": "0",
	}}
	cmd := Command{Env: map[string]string{
		"PORT": "3001", // overrides location
	}}

	env := EffectiveEnv(loc, cmd)

	if env["PORT"] != "3001" {
		t.Errorf("PORT = %q, want 3001 (command overrides location)", env["PORT"])
	}
	if env["DEBUG"] != "0" {
		t.Errorf("DEBUG = %q, want 0 (inherited from location)", env["DEBUG"])
	}
}

func TestEffectiveEnvEmptyReturnsNil(t *testing.T) {
	if env := EffectiveEnv(Location{}, Command{}); env != nil {
		t.Errorf("expected nil env when nothing defined, got %v", env)
	}
}

func TestEffectiveEnvAmbientExpansion(t *testing.T) {
	t.Setenv("GOPM_TEST_HOME", "/home/tester")

	loc := Location{Env: map[string]string{
		"BINDIR": "${GOPM_TEST_HOME}/bin",
	}}
	env := EffectiveEnv(loc, Command{})

	if env["BINDIR"] != "/home/tester/bin" {
		t.Errorf("BINDIR = %q, want /home/tester/bin", env["BINDIR"])
	}
}

func TestEffectiveEnvSiblingExpansion(t *testing.T) {
	t.Setenv("GOPM_TEST_PATH", "/usr/bin")

	loc := Location{Env: map[string]string{
		"BIN":  "/opt/bin",
		"PATH": "${BIN}:$GOPM_TEST_PATH", // references sibling + ambient
	}}
	env := EffectiveEnv(loc, Command{})

	if env["PATH"] != "/opt/bin:/usr/bin" {
		t.Errorf("PATH = %q, want /opt/bin:/usr/bin", env["PATH"])
	}
}

func TestEffectiveEnvMultiHopChain(t *testing.T) {
	loc := Location{Env: map[string]string{
		"A": "a",
		"B": "${A}b",
		"C": "${B}c",
	}}
	env := EffectiveEnv(loc, Command{})

	if env["C"] != "abc" {
		t.Errorf("C = %q, want abc (multi-hop chain)", env["C"])
	}
}

func TestEffectiveEnvUndefinedExpandsToEmpty(t *testing.T) {
	loc := Location{Env: map[string]string{
		"OUT": "${GOPM_DEFINITELY_UNSET_VAR}/x",
	}}
	env := EffectiveEnv(loc, Command{})

	if env["OUT"] != "/x" {
		t.Errorf("OUT = %q, want /x (undefined expands to empty)", env["OUT"])
	}
}

func TestEffectiveEnvCycleTerminates(t *testing.T) {
	// A references B and B references A: must terminate (not hang) and
	// resolve cyclic references to empty rather than looping forever. If the
	// cycle guard is missing this call recurses infinitely and `go test`'s
	// timeout fails the run.
	loc := Location{Env: map[string]string{
		"A": "${B}",
		"B": "${A}",
	}}

	env := EffectiveEnv(loc, Command{})
	if env["A"] != "" || env["B"] != "" {
		t.Errorf("cyclic vars = (%q,%q), want empty", env["A"], env["B"])
	}
}

func TestCommandObjectFormWithEnvUnmarshals(t *testing.T) {
	data := `
locations:
  - location: api
    env:
      PORT: "3000"
    commands:
      - name: dev
        command: npm run dev
        env:
          DEBUG: "1"
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(cfg.Locations) != 1 {
		t.Fatalf("expected 1 location, got %d", len(cfg.Locations))
	}
	loc := cfg.Locations[0]
	if loc.Env["PORT"] != "3000" {
		t.Errorf("location env PORT = %q, want 3000", loc.Env["PORT"])
	}
	if len(loc.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(loc.Commands))
	}
	if loc.Commands[0].Env["DEBUG"] != "1" {
		t.Errorf("command env DEBUG = %q, want 1", loc.Commands[0].Env["DEBUG"])
	}
}

func TestCommandStringShorthandHasNilEnv(t *testing.T) {
	data := `
locations:
  - location: api
    commands:
      - npm run build
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	cmd := cfg.Locations[0].Commands[0]
	if cmd.Command != "npm run build" {
		t.Errorf("command = %q, want 'npm run build'", cmd.Command)
	}
	if cmd.Env != nil {
		t.Errorf("string-shorthand command should have nil env, got %v", cmd.Env)
	}
}
