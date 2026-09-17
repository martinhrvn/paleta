package parsers

import (
	"errors"
	"reflect"
	"testing"
	"time"
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

// TestCommandParser_TimesOut pins that a hanging parser command can't stall a
// `plt` launch: it returns promptly with the timeout sentinel.
func TestCommandParser_TimesOut(t *testing.T) {
	parser := &CommandParser{}
	old := ParserCommandTimeout
	ParserCommandTimeout = 150 * time.Millisecond
	defer func() { ParserCommandTimeout = old }()

	start := time.Now()
	_, err := parser.ParseCommands(t.TempDir(), ParserConfig{ParserCommand: "sleep 30"})
	elapsed := time.Since(start)

	if !errors.Is(err, ErrParserTimeout) {
		t.Fatalf("err = %v, want ErrParserTimeout", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("took %s, want the parser to be abandoned near the timeout", elapsed)
	}
}

// TestCommandParser_TimeoutDoesNotWaitForOrphan covers a child that keeps the
// stdout pipe open past the deadline: WaitDelay must cut it loose.
func TestCommandParser_TimeoutDoesNotWaitForOrphan(t *testing.T) {
	parser := &CommandParser{}
	old := ParserCommandTimeout
	ParserCommandTimeout = 150 * time.Millisecond
	defer func() { ParserCommandTimeout = old }()

	start := time.Now()
	_, err := parser.ParseCommands(t.TempDir(), ParserConfig{ParserCommand: "sleep 30 & echo build; wait"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected an error, got none")
	}
	if elapsed > 5*time.Second {
		t.Errorf("took %s, want the orphaned child not to block the return", elapsed)
	}
}
