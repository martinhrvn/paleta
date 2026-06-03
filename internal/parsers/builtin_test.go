package parsers

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeFile writes content to a file inside dir and returns dir.
func writePackageJSON(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}
}

func TestPackageJsonParserValid(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{
		"name": "example",
		"scripts": {
			"test": "jest",
			"build": "webpack",
			"start": "node index.js"
		}
	}`)

	parser := &PackageJsonParser{}
	commands, err := parser.ParseCommands(dir, ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output must be sorted for deterministic display.
	want := []string{"build", "start", "test"}
	if !reflect.DeepEqual(commands, want) {
		t.Errorf("commands = %v, want %v", commands, want)
	}
}

func TestPackageJsonParserIgnoresNonStringScripts(t *testing.T) {
	dir := t.TempDir()
	// A malformed/nested script value must be skipped, not crash.
	writePackageJSON(t, dir, `{
		"scripts": {
			"ok": "echo ok",
			"weird": { "nested": "object" },
			"alsoweird": ["array"]
		}
	}`)

	parser := &PackageJsonParser{}
	commands, err := parser.ParseCommands(dir, ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"ok"}
	if !reflect.DeepEqual(commands, want) {
		t.Errorf("commands = %v, want %v", commands, want)
	}
}

func TestPackageJsonParserNoScripts(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"name": "no-scripts"}`)

	parser := &PackageJsonParser{}
	commands, err := parser.ParseCommands(dir, ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commands) != 0 {
		t.Errorf("expected no commands, got %v", commands)
	}
}

func TestPackageJsonParserMissingFile(t *testing.T) {
	dir := t.TempDir() // no package.json written

	parser := &PackageJsonParser{}
	_, err := parser.ParseCommands(dir, ParserConfig{})
	if err == nil {
		t.Fatal("expected error for missing package.json, got nil")
	}
}

func TestPackageJsonParserMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{ this is not valid json `)

	parser := &PackageJsonParser{}
	_, err := parser.ParseCommands(dir, ParserConfig{})
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestGoStandardParser(t *testing.T) {
	parser := &GoStandardParser{}
	commands, err := parser.ParseCommands(t.TempDir(), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return the well-known static set of Go subcommands.
	if len(commands) == 0 {
		t.Fatal("expected standard go commands, got none")
	}
	for _, want := range []string{"build", "test", "run", "vet", "fmt"} {
		found := false
		for _, c := range commands {
			if c == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in go commands, got %v", want, commands)
		}
	}
}
