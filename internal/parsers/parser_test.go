package parsers

import (
	"testing"
)

func TestGetParser(t *testing.T) {
	tests := []struct {
		name         string
		config       ParserConfig
		expectedType string
		shouldError  bool
	}{
		{
			name: "builtin parser",
			config: ParserConfig{
				BuiltinParser: "package_json_scripts",
			},
			expectedType: "*parsers.PackageJsonParser",
			shouldError:  false,
		},
		{
			name: "command parser",
			config: ParserConfig{
				ParserCommand: "echo 'test'",
			},
			expectedType: "*parsers.CommandParser",
			shouldError:  false,
		},
		{
			name:   "null parser",
			config: ParserConfig{
				// Neither builtin nor command specified
			},
			expectedType: "*parsers.NullParser",
			shouldError:  false,
		},
		{
			name: "invalid builtin parser",
			config: ParserConfig{
				BuiltinParser: "invalid_parser",
			},
			expectedType: "",
			shouldError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, err := GetParser(tt.config)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if parser == nil {
				t.Error("Expected parser but got nil")
				return
			}

			// Check parser type (this is a simplified check)
			switch tt.expectedType {
			case "*parsers.PackageJsonParser":
				if _, ok := parser.(*PackageJsonParser); !ok {
					t.Errorf("Expected PackageJsonParser, got %T", parser)
				}
			case "*parsers.CommandParser":
				if _, ok := parser.(*CommandParser); !ok {
					t.Errorf("Expected CommandParser, got %T", parser)
				}
			case "*parsers.NullParser":
				if _, ok := parser.(*NullParser); !ok {
					t.Errorf("Expected NullParser, got %T", parser)
				}
			}
		})
	}
}

func TestParseAndFormatCommands(t *testing.T) {
	// Test with base commands only
	config := ParserConfig{
		BaseCommands: map[string]string{
			"install": "npm install",
			"test":    "npm test",
		},
	}

	commands, err := ParseAndFormatCommands(".", config)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if len(commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(commands))
	}

	if commands["install"] != "npm install" {
		t.Errorf("Expected install command to be 'npm install', got '%s'", commands["install"])
	}
}

func TestNullParser(t *testing.T) {
	parser := &NullParser{}

	commands, err := parser.ParseCommands(".", ParserConfig{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(commands) != 0 {
		t.Errorf("Expected 0 commands from null parser, got %d", len(commands))
	}
}
