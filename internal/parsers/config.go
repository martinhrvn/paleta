package parsers

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed default_parsers.yaml
var defaultParsersYAML []byte

// ParserConfig represents a single parser configuration
type ParserConfig struct {
	// DetectFiles are the files to look for to detect this project type
	DetectFiles []string `yaml:"detect_files"`
	
	// BaseCommands are commands that are always available, regardless of parsing
	BaseCommands map[string]string `yaml:"base_commands"`
	
	// BuiltinParser specifies a built-in parser to use (e.g., "package_json_scripts")
	BuiltinParser string `yaml:"builtin_parser,omitempty"`
	
	// ParserCommand is a shell command that outputs available commands (one per line)
	ParserCommand string `yaml:"parser_command,omitempty"`
	
	// CommandTemplate is how to construct the final command (e.g., "npm run {key}")
	CommandTemplate string `yaml:"command_template,omitempty"`
}

// ParsersFile represents the entire parsers.yaml configuration
type ParsersFile struct {
	Parsers map[string]ParserConfig `yaml:"parsers"`
}

// LoadParsersConfig loads parser configuration from ~/.paleta/parsers.yaml
func LoadParsersConfig() (*ParsersFile, error) {
	// Start with embedded defaults
	defaults, err := loadEmbeddedDefaults()
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded defaults: %w", err)
	}

	// Try to load user config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// If we can't get home dir, just use defaults
		return defaults, nil
	}

	configPath := filepath.Join(homeDir, ".paleta", "parsers.yaml")
	
	// If user config doesn't exist, return defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return defaults, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		// If we can't read user config, return defaults
		return defaults, nil
	}

	var userConfig ParsersFile
	if err := yaml.Unmarshal(data, &userConfig); err != nil {
		return nil, fmt.Errorf("failed to parse user parsers config: %w", err)
	}

	// Merge user config with defaults (user config takes precedence)
	merged := &ParsersFile{
		Parsers: make(map[string]ParserConfig),
	}
	
	// Start with defaults
	for name, parser := range defaults.Parsers {
		merged.Parsers[name] = parser
	}
	
	// Override with user config
	if userConfig.Parsers != nil {
		for name, parser := range userConfig.Parsers {
			merged.Parsers[name] = parser
		}
	}

	return merged, nil
}

// loadEmbeddedDefaults loads the embedded default configuration
func loadEmbeddedDefaults() (*ParsersFile, error) {
	var defaults ParsersFile
	if err := yaml.Unmarshal(defaultParsersYAML, &defaults); err != nil {
		return nil, fmt.Errorf("failed to parse embedded defaults: %w", err)
	}
	return &defaults, nil
}


// GetParser returns a parser configuration by name
func (p *ParsersFile) GetParser(name string) (ParserConfig, bool) {
	parser, exists := p.Parsers[name]
	return parser, exists
}

// FindParserForDirectory finds a parser that matches files in the given directory
func (p *ParsersFile) FindParserForDirectory(directory string) (string, ParserConfig, error) {
	for name, parser := range p.Parsers {
		for _, detectFile := range parser.DetectFiles {
			filePath := filepath.Join(directory, detectFile)
			if _, err := os.Stat(filePath); err == nil {
				return name, parser, nil
			}
		}
	}
	return "", ParserConfig{}, fmt.Errorf("no parser found for directory: %s", directory)
}