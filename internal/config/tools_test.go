package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// A YAML sequence under `tools:` (the .pltrc form) decodes into the enabled list.
func TestToolsFieldUnmarshalSequence(t *testing.T) {
	var cfg Config
	src := "tools:\n  - lazygit\n  - docker\n"
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got, want := len(cfg.Tools.Enabled), 2; got != want {
		t.Fatalf("Enabled len = %d, want %d", got, want)
	}
	if cfg.Tools.Enabled[0] != "lazygit" || cfg.Tools.Enabled[1] != "docker" {
		t.Fatalf("Enabled = %v, want [lazygit docker]", cfg.Tools.Enabled)
	}
	if len(cfg.Tools.Defs) != 0 {
		t.Fatalf("Defs should be empty for a sequence, got %v", cfg.Tools.Defs)
	}
}

// A YAML mapping under `tools:` (the global-config form) decodes into definitions,
// supporting both the single-command and multi-command shapes.
func TestToolsFieldUnmarshalMapping(t *testing.T) {
	var cfg Config
	src := `tools:
  lazygit:
    command: lazygit
  docker:
    commands:
      - name: up
        command: docker compose up
      - name: logs
        command: docker compose logs -f
`
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(cfg.Tools.Enabled) != 0 {
		t.Fatalf("Enabled should be empty for a mapping, got %v", cfg.Tools.Enabled)
	}
	if got := len(cfg.Tools.Defs); got != 2 {
		t.Fatalf("Defs len = %d, want 2", got)
	}
	if cfg.Tools.Defs["lazygit"].Command.joined != "lazygit" {
		t.Fatalf("lazygit command = %q, want lazygit", cfg.Tools.Defs["lazygit"].Command.joined)
	}
	docker := cfg.Tools.Defs["docker"]
	if len(docker.Commands) != 2 || docker.Commands[0].Name != "up" {
		t.Fatalf("docker commands = %+v, want up/logs", docker.Commands)
	}
}

// A builtin single-command tool resolves to one row running in the given workdir.
func TestAttachToolsBuiltinSingle(t *testing.T) {
	cfg := &Config{Tools: ToolsField{Enabled: []string{"lazygit"}}}
	AttachTools(cfg, "/home/user/proj")

	if len(cfg.ResolvedTools) != 1 {
		t.Fatalf("ResolvedTools len = %d, want 1", len(cfg.ResolvedTools))
	}
	rt := cfg.ResolvedTools[0]
	if rt.Display != "lazygit" || rt.Command != "lazygit" || rt.Directory != "/home/user/proj" {
		t.Fatalf("resolved = %+v", rt)
	}
	if len(cfg.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", cfg.Warnings)
	}
}

// A builtin multi-command tool resolves to one row per command, labeled "tool: cmd".
func TestAttachToolsBuiltinMulti(t *testing.T) {
	cfg := &Config{Tools: ToolsField{Enabled: []string{"docker"}}}
	AttachTools(cfg, "/w")

	// The builtin docker tool ships up/down/logs/ps.
	if len(cfg.ResolvedTools) != 4 {
		t.Fatalf("ResolvedTools len = %d, want 4 (%+v)", len(cfg.ResolvedTools), cfg.ResolvedTools)
	}
	if cfg.ResolvedTools[0].Display != "docker: up" {
		t.Fatalf("first row display = %q, want 'docker: up'", cfg.ResolvedTools[0].Display)
	}
	if cfg.ResolvedTools[0].Command != "docker compose up" {
		t.Fatalf("first row command = %q", cfg.ResolvedTools[0].Command)
	}
}

// A global definition overrides a builtin of the same name.
func TestAttachToolsGlobalOverride(t *testing.T) {
	cfg := &Config{
		Tools: ToolsField{Enabled: []string{"lazygit"}},
		ToolDefs: map[string]ToolDefinition{
			"lazygit": {Command: commandField{joined: "lazygit --custom"}},
		},
	}
	AttachTools(cfg, "/w")

	if len(cfg.ResolvedTools) != 1 || cfg.ResolvedTools[0].Command != "lazygit --custom" {
		t.Fatalf("override not applied: %+v", cfg.ResolvedTools)
	}
}

// An enabled tool that isn't defined anywhere records a warning and produces no row.
func TestAttachToolsUnknown(t *testing.T) {
	cfg := &Config{Tools: ToolsField{Enabled: []string{"nope"}}}
	AttachTools(cfg, "/w")

	if len(cfg.ResolvedTools) != 0 {
		t.Fatalf("unknown tool should not resolve: %+v", cfg.ResolvedTools)
	}
	if len(cfg.Warnings) != 1 || cfg.Warnings[0].Kind != "tool" {
		t.Fatalf("expected one tool warning, got %+v", cfg.Warnings)
	}
}

// Tool-level and command-level env are merged (command overrides) and resolved.
func TestAttachToolsEnv(t *testing.T) {
	cfg := &Config{
		Tools: ToolsField{Enabled: []string{"mytool"}},
		ToolDefs: map[string]ToolDefinition{
			"mytool": {
				Env:      map[string]string{"BASE": "base", "SHARED": "tool"},
				Commands: []Command{{Name: "run", Command: "do", Env: map[string]string{"SHARED": "cmd"}}},
			},
		},
	}
	AttachTools(cfg, "/w")

	if len(cfg.ResolvedTools) != 1 {
		t.Fatalf("ResolvedTools len = %d, want 1", len(cfg.ResolvedTools))
	}
	env := cfg.ResolvedTools[0].Env
	if env["BASE"] != "base" || env["SHARED"] != "cmd" {
		t.Fatalf("env merge wrong: %v", env)
	}
}
