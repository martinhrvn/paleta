package parsers

import (
	"fmt"
	"strings"
)

// Parser is the interface for parsing commands from a project
type Parser interface {
	// ParseCommands returns a list of commands from the project
	ParseCommands(directory string, config ParserConfig) ([]string, error)
}

// GetParser returns the appropriate parser based on the configuration
func GetParser(config ParserConfig) (Parser, error) {
	// If a built-in parser is specified, use it
	if config.BuiltinParser != "" {
		switch config.BuiltinParser {
		case "package_json_scripts":
			return &PackageJsonParser{}, nil
		case "go_standard":
			return &GoStandardParser{}, nil
		default:
			return nil, fmt.Errorf("unknown built-in parser: %s", config.BuiltinParser)
		}
	}

	// If a parser command is specified, use the command parser
	if config.ParserCommand != "" {
		return &CommandParser{}, nil
	}

	// If neither is specified, return a null parser (only base commands)
	return &NullParser{}, nil
}

// NullParser returns no commands (used when only base commands are needed)
type NullParser struct{}

func (n *NullParser) ParseCommands(directory string, config ParserConfig) ([]string, error) {
	return []string{}, nil
}

// ParseAndFormatCommands parses commands and applies templates
func ParseAndFormatCommands(directory string, config ParserConfig) (map[string]string, error) {
	parser, err := GetParser(config)
	if err != nil {
		return nil, err
	}

	// Start with base commands
	commands := make(map[string]string)
	for key, cmd := range config.BaseCommands {
		commands[key] = cmd
	}

	// Parse additional commands
	parsedKeys, err := parser.ParseCommands(directory, config)
	if err != nil {
		return nil, err
	}

	// Apply command template to parsed commands
	for _, key := range parsedKeys {
		if config.CommandTemplate != "" {
			cmd := strings.ReplaceAll(config.CommandTemplate, "{key}", key)
			commands[key] = cmd
		} else {
			// No template, use key as-is
			commands[key] = key
		}
	}

	return commands, nil
}

// DetectAndParseCommands detects the project type and parses commands
func DetectAndParseCommands(directory string, parsersConfig *ParsersFile) (map[string]string, error) {
	// Find the appropriate parser for this directory
	parserName, parserConfig, err := parsersConfig.FindParserForDirectory(directory)
	if err != nil {
		// No parser found, return empty
		return map[string]string{}, nil
	}

	// Parse and format commands
	commands, err := ParseAndFormatCommands(directory, parserConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse commands for %s: %w", parserName, err)
	}

	return commands, nil
}
