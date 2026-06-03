package projecttypes

import (
	"fmt"
	"path/filepath"

	"github.com/martinhrvn/go-pm/internal/parsers"
)

// ConfigurableProjectType implements ProjectType using the new parser system
type ConfigurableProjectType struct {
	name         string
	parserConfig parsers.ParserConfig
}

// NewConfigurableProjectType creates a new configurable project type
func NewConfigurableProjectType(name string, config parsers.ParserConfig) *ConfigurableProjectType {
	return &ConfigurableProjectType{
		name:         name,
		parserConfig: config,
	}
}

func (c *ConfigurableProjectType) Name() string {
	return c.name
}

func (c *ConfigurableProjectType) DetectConfigFile() string {
	if len(c.parserConfig.DetectFiles) > 0 {
		return c.parserConfig.DetectFiles[0]
	}
	return ""
}

func (c *ConfigurableProjectType) ParseCommands(configPath string) ([]string, error) {
	directory := filepath.Dir(configPath)
	
	// Parse and format commands using the parser system
	commands, err := parsers.ParseAndFormatCommands(directory, c.parserConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse commands: %w", err)
	}

	// Convert map to slice for compatibility with existing interface
	var result []string
	for key, _ := range commands {
		result = append(result, key)
	}

	return result, nil
}

func (c *ConfigurableProjectType) GetCommandPrefix() string {
	// For configurable project types, we don't use a prefix because
	// the command template already includes the full command format
	return ""
}

// GetFullCommand returns the full command for a given key
func (c *ConfigurableProjectType) GetFullCommand(directory string, key string) (string, error) {
	// Parse and format commands using the parser system
	commands, err := parsers.ParseAndFormatCommands(directory, c.parserConfig)
	if err != nil {
		return "", fmt.Errorf("failed to parse commands: %w", err)
	}

	cmd, exists := commands[key]
	if !exists {
		return "", fmt.Errorf("command '%s' not found", key)
	}

	return cmd, nil
}

// CanHandleDirectory checks if this project type can handle the given directory
func (c *ConfigurableProjectType) CanHandleDirectory(directory string) bool {
	for _, detectFile := range c.parserConfig.DetectFiles {
		configFile := filepath.Join(directory, detectFile)
		if fileExists(configFile) {
			return true
		}
	}
	return false
}

// GetAllCommands returns all commands for a directory as a map
func (c *ConfigurableProjectType) GetAllCommands(directory string) (map[string]string, error) {
	return parsers.ParseAndFormatCommands(directory, c.parserConfig)
}