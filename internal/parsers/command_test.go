package parsers

import (
	"reflect"
	"testing"
)

func TestCommandParserEmptyCommand(t *testing.T) {
	parser := &CommandParser{}
	commands, err := parser.ParseCommands(t.TempDir(), ParserConfig{ParserCommand: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commands) != 0 {
		t.Errorf("expected no commands for empty parser command, got %v", commands)
	}
}

func TestCommandParserMultilineOutput(t *testing.T) {
	parser := &CommandParser{}
	// printf emits three lines; the parser should split them into commands.
	cfg := ParserConfig{ParserCommand: "printf 'build\\ntest\\nlint\\n'"}
	commands, err := parser.ParseCommands(t.TempDir(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"build", "test", "lint"}
	if !reflect.DeepEqual(commands, want) {
		t.Errorf("commands = %v, want %v", commands, want)
	}
}

func TestCommandParserTrimsWhitespaceAndBlankLines(t *testing.T) {
	parser := &CommandParser{}
	// Leading/trailing spaces and blank lines must be cleaned up.
	cfg := ParserConfig{ParserCommand: "printf '  build  \\n\\n  test\\n'"}
	commands, err := parser.ParseCommands(t.TempDir(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"build", "test"}
	if !reflect.DeepEqual(commands, want) {
		t.Errorf("commands = %v, want %v", commands, want)
	}
}

func TestCommandParserEmptyOutput(t *testing.T) {
	parser := &CommandParser{}
	cfg := ParserConfig{ParserCommand: "true"} // succeeds, prints nothing
	commands, err := parser.ParseCommands(t.TempDir(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commands) != 0 {
		t.Errorf("expected no commands for empty output, got %v", commands)
	}
}

func TestCommandParserCommandFailure(t *testing.T) {
	parser := &CommandParser{}
	cfg := ParserConfig{ParserCommand: "exit 1"}
	_, err := parser.ParseCommands(t.TempDir(), cfg)
	if err == nil {
		t.Fatal("expected error when parser command fails, got nil")
	}
}
