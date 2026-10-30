package parsers

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// devenvConfig returns the embedded devenv parser configuration.
func devenvConfig(t *testing.T) ParserConfig {
	t.Helper()
	defaults, err := loadEmbeddedDefaults()
	if err != nil {
		t.Fatalf("loadEmbeddedDefaults failed: %v", err)
	}
	cfg, ok := defaults.Parsers["devenv"]
	if !ok {
		t.Fatal("expected a 'devenv' parser in embedded defaults")
	}
	return cfg
}

// fakeDevenv puts a `devenv` executable with the given shell body first on PATH.
// The body sees the CLI arguments as "$*".
func fakeDevenv(t *testing.T, body string) {
	t.Helper()
	bin := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(bin, "devenv"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// devenvFixture answers the two listing calls the parser makes, trimmed to the
// fields it reads, in the shape devenv 2.x prints them.
const devenvFixture = `case "$*" in
  "eval processes scripts") cat <<'JSON'
{
  "processes": {"web": {"exec": "npm start"}, "worker": {"exec": "./worker"}},
  "scripts": {"build": {"exec": "go build"}, "test": {"exec": "go test ./..."}}
}
JSON
  ;;
  "tasks list --json") cat <<'JSON'
[
  {"name": "devenv:enterShell", "hasExec": true},
  {"name": "devenv:files", "hasExec": false},
  {"name": "myapp:migrate", "hasExec": true},
  {"name": "myapp:seed", "hasExec": true}
]
JSON
  ;;
  *) echo "unexpected args: $*" >&2; exit 2 ;;
esac`

func TestDevenvParser_ListsProcessesTasksAndScripts(t *testing.T) {
	fakeDevenv(t, devenvFixture)
	dir := t.TempDir()
	touch(t, dir, "devenv.nix")

	commands, err := ParseAndFormatCommands(dir, devenvConfig(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]string{
		// base commands
		"up":     "devenv up",
		"shell":  "devenv shell",
		"test":   "devenv test",
		"update": "devenv update",
		// one per process
		"up:web":    "devenv up web",
		"up:worker": "devenv up worker",
		// one per user task
		"task:myapp:migrate": "devenv tasks run myapp:migrate",
		"task:myapp:seed":    "devenv tasks run myapp:seed",
		// one per script; prefixed so a `test` script doesn't shadow `devenv test`
		"script:build": "devenv shell build",
		"script:test":  "devenv shell test",
	}
	for name, cmd := range want {
		if commands[name] != cmd {
			t.Errorf("%s = %q, want %q", name, commands[name], cmd)
		}
	}
	if len(commands) != len(want) {
		t.Errorf("got %d commands, want %d: %v", len(commands), len(want), commands)
	}
}

// devenv's own lifecycle tasks (devenv:enterShell, devenv:files, ...) are
// plumbing, not something a user picks from a palette.
func TestDevenvParser_SkipsBuiltinTasks(t *testing.T) {
	fakeDevenv(t, devenvFixture)
	dir := t.TempDir()

	commands, err := ParseAndFormatCommands(dir, devenvConfig(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for name := range commands {
		if len(name) > len("task:devenv:") && name[:len("task:devenv:")] == "task:devenv:" {
			t.Errorf("unexpected builtin task row %q", name)
		}
	}
}

// A failing devenv (not installed, broken devenv.nix) degrades to the base
// commands and reports the error, like a failing parser command does.
func TestDevenvParser_FailureKeepsBaseCommands(t *testing.T) {
	fakeDevenv(t, `echo "error: evaluation aborted" >&2; exit 1`)
	dir := t.TempDir()

	commands, err := ParseAndFormatCommands(dir, devenvConfig(t))
	if err == nil {
		t.Fatal("expected an error from a failing devenv")
	}
	if commands["up"] != "devenv up" || commands["shell"] != "devenv shell" {
		t.Errorf("base commands lost on failure: %v", commands)
	}
}

// A hung devenv is abandoned at the parser deadline, like a parser command.
func TestDevenvParser_Timeout(t *testing.T) {
	fakeDevenv(t, `sleep 5`)
	old := ParserCommandTimeout
	ParserCommandTimeout = 100 * time.Millisecond
	t.Cleanup(func() { ParserCommandTimeout = old })

	_, err := ParseAndFormatCommands(t.TempDir(), devenvConfig(t))
	if !errors.Is(err, ErrParserTimeout) {
		t.Fatalf("err = %v, want ErrParserTimeout", err)
	}
}

func TestDevenvType_DetectedByDevenvNix(t *testing.T) {
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	touch(t, dir, "devenv.nix")
	if got := reg.DetectTypes(dir); len(got) != 1 || got[0] != "devenv" {
		t.Errorf("DetectTypes(devenv.nix) = %v, want [devenv]", got)
	}
}
