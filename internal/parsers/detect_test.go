package parsers

import (
	"os"
	"path/filepath"
	"testing"
)

func touch(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create %s: %v", name, err)
	}
}

func TestEmbeddedDefaultsDockerAndCompose(t *testing.T) {
	defaults, err := loadEmbeddedDefaults()
	if err != nil {
		t.Fatalf("loadEmbeddedDefaults failed: %v", err)
	}

	docker, ok := defaults.Parsers["docker"]
	if !ok {
		t.Fatal("expected a 'docker' parser in embedded defaults")
	}
	if docker.BaseCommands["build"] != "docker build ." {
		t.Errorf("docker build = %q, want 'docker build .'", docker.BaseCommands["build"])
	}
	if docker.BaseCommands["run"] == "" {
		t.Error("expected docker parser to define a 'run' command")
	}
	// docker (Dockerfile) must not carry compose commands.
	if _, has := docker.BaseCommands["up"]; has {
		t.Error("docker parser should not define compose 'up'")
	}

	compose, ok := defaults.Parsers["compose"]
	if !ok {
		t.Fatal("expected a 'compose' parser in embedded defaults")
	}
	if compose.BaseCommands["up"] != "docker compose up" {
		t.Errorf("compose up = %q, want 'docker compose up'", compose.BaseCommands["up"])
	}
	if compose.BaseCommands["down"] != "docker compose down" {
		t.Errorf("compose down = %q, want 'docker compose down'", compose.BaseCommands["down"])
	}
	if compose.BaseCommands["build"] != "docker compose build" {
		t.Errorf("compose build = %q, want 'docker compose build'", compose.BaseCommands["build"])
	}
	// compose must detect glob override files.
	foundGlob := false
	for _, f := range compose.DetectFiles {
		if f == "docker-compose.*.yml" {
			foundGlob = true
		}
	}
	if !foundGlob {
		t.Errorf("compose detect_files = %v, want it to include 'docker-compose.*.yml'", compose.DetectFiles)
	}
}

func TestParseAndFormatCommandsAppliesTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"),
		[]byte(`{"scripts":{"build":"webpack","test":"jest"}}`), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	cfg := ParserConfig{
		BaseCommands:    map[string]string{"install": "npm install"},
		BuiltinParser:   "package_json_scripts",
		CommandTemplate: "npm run {key}",
	}

	commands, err := ParseAndFormatCommands(dir, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Base command preserved verbatim.
	if commands["install"] != "npm install" {
		t.Errorf("install = %q, want 'npm install'", commands["install"])
	}
	// Parsed scripts get the template applied.
	if commands["build"] != "npm run build" {
		t.Errorf("build = %q, want 'npm run build'", commands["build"])
	}
	if commands["test"] != "npm run test" {
		t.Errorf("test = %q, want 'npm run test'", commands["test"])
	}
}

func TestParseAndFormatCommandsNoTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"),
		[]byte(`{"scripts":{"build":"webpack"}}`), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	cfg := ParserConfig{BuiltinParser: "package_json_scripts"}

	commands, err := ParseAndFormatCommands(dir, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Without a template, the key is used as the command as-is.
	if commands["build"] != "build" {
		t.Errorf("build = %q, want 'build'", commands["build"])
	}
}

func TestDetectFilePresent(t *testing.T) {
	dir := t.TempDir()
	// Only an env-specific override file is present, no plain docker-compose.yml.
	touch(t, dir, "docker-compose.prod.yml")

	if !DetectFilePresent(dir, "docker-compose.*.yml") {
		t.Error("glob pattern should match docker-compose.prod.yml")
	}
	if DetectFilePresent(dir, "docker-compose.yml") {
		t.Error("literal name must not match a differently named file")
	}
	if DetectFilePresent(t.TempDir(), "docker-compose.*.yml") {
		t.Error("empty directory must not match")
	}
}
