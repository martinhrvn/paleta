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

func TestFindParserForDirectoryMatch(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "package.json")

	pf := &ParsersFile{Parsers: map[string]ParserConfig{
		"npm": {DetectFiles: []string{"package.json"}},
		"go":  {DetectFiles: []string{"go.mod"}},
	}}

	name, cfg, err := pf.FindParserForDirectory(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "npm" {
		t.Errorf("matched parser = %q, want npm", name)
	}
	if len(cfg.DetectFiles) == 0 || cfg.DetectFiles[0] != "package.json" {
		t.Errorf("returned config does not match npm: %+v", cfg)
	}
}

func TestFindParserForDirectoryGlobMatch(t *testing.T) {
	dir := t.TempDir()
	// Only an env-specific override file is present, no plain docker-compose.yml.
	touch(t, dir, "docker-compose.prod.yml")

	pf := &ParsersFile{Parsers: map[string]ParserConfig{
		"compose": {DetectFiles: []string{"docker-compose.yml", "docker-compose.*.yml"}},
		"go":      {DetectFiles: []string{"go.mod"}},
	}}

	name, _, err := pf.FindParserForDirectory(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "compose" {
		t.Errorf("matched parser = %q, want compose", name)
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

func TestFindParserForDirectoryNoMatch(t *testing.T) {
	dir := t.TempDir() // empty directory

	pf := &ParsersFile{Parsers: map[string]ParserConfig{
		"npm": {DetectFiles: []string{"package.json"}},
	}}

	_, _, err := pf.FindParserForDirectory(dir)
	if err == nil {
		t.Fatal("expected error when no parser matches, got nil")
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

func TestDetectAndParseCommandsEndToEnd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"),
		[]byte(`{"scripts":{"dev":"vite"}}`), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	pf := &ParsersFile{Parsers: map[string]ParserConfig{
		"npm": {
			DetectFiles:     []string{"package.json"},
			BaseCommands:    map[string]string{"install": "npm install"},
			BuiltinParser:   "package_json_scripts",
			CommandTemplate: "npm run {key}",
		},
	}}

	commands, err := DetectAndParseCommands(dir, pf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if commands["dev"] != "npm run vite" && commands["dev"] != "npm run dev" {
		t.Errorf("dev = %q, want 'npm run dev'", commands["dev"])
	}
	if commands["install"] != "npm install" {
		t.Errorf("install = %q, want 'npm install'", commands["install"])
	}
}

func TestDetectAndParseCommandsNoParserReturnsEmpty(t *testing.T) {
	dir := t.TempDir() // nothing to detect

	pf := &ParsersFile{Parsers: map[string]ParserConfig{
		"npm": {DetectFiles: []string{"package.json"}},
	}}

	commands, err := DetectAndParseCommands(dir, pf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commands) != 0 {
		t.Errorf("expected empty command map, got %v", commands)
	}
}
