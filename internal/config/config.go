package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/martin/go-pm/internal/projecttypes"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Locations []Location `yaml:"locations"`
}

type Location struct {
	Name     string   `yaml:"name,omitempty"`
	Location string   `yaml:"location,omitempty"`
	Type     string   `yaml:"type,omitempty"`
	Commands []string `yaml:"commands,omitempty"`
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Normalize empty paths to current directory
	normalizeEmptyPaths(&config)

	// Expand glob patterns in locations
	expandedLocations, err := ExpandGlobPatterns(config.Locations)
	if err != nil {
		return nil, fmt.Errorf("failed to expand glob patterns: %w", err)
	}
	config.Locations = expandedLocations

	// Process project types and add their commands
	if err := processProjectTypes(&config); err != nil {
		return nil, fmt.Errorf("failed to process project types: %w", err)
	}

	return &config, nil
}

// normalizeEmptyPaths normalizes empty location paths to "." (current directory)
func normalizeEmptyPaths(config *Config) {
	for i := range config.Locations {
		if config.Locations[i].Location == "" {
			config.Locations[i].Location = "."
		}
	}
}

// processProjectTypes processes project types and adds their commands to locations
func processProjectTypes(config *Config) error {
	for i, location := range config.Locations {
		if location.Type == "" {
			continue
		}

		// Get the project type
		projectType, err := projecttypes.GetProjectType(location.Type)
		if err != nil {
			return fmt.Errorf("location %s has invalid type: %w", location.Location, err)
		}

		// Check if the project type config file exists in the location
		configFile := filepath.Join(location.Location, projectType.DetectConfigFile())
		if !fileExists(configFile) {
			// If no config file found, just keep the existing commands
			continue
		}

		// For configurable project types, get the full commands directly
		if configurableType, ok := projectType.(*projecttypes.ConfigurableProjectType); ok {
			// Get all commands as a map
			commands, err := configurableType.GetAllCommands(location.Location)
			if err != nil {
				return fmt.Errorf("failed to parse commands for location %s: %w", location.Location, err)
			}

			// Convert to command list format
			var commandList []string
			for _, cmd := range commands {
				commandList = append(commandList, cmd)
			}

			// Merge with existing commands
			allCommands := append(location.Commands, commandList...)
			config.Locations[i].Commands = allCommands
		} else {
			// Fallback to old behavior for backward compatibility
			// Parse commands from the project type
			projectCommands, err := projectType.ParseCommands(configFile)
			if err != nil {
				return fmt.Errorf("failed to parse commands for location %s: %w", location.Location, err)
			}

			// Prefix the commands with the project type command prefix
			var prefixedCommands []string
			for _, cmd := range projectCommands {
				if projectType.GetCommandPrefix() != "" {
					prefixedCommands = append(prefixedCommands, fmt.Sprintf("%s %s", projectType.GetCommandPrefix(), cmd))
				} else {
					prefixedCommands = append(prefixedCommands, cmd)
				}
			}

			// Merge with existing commands
			allCommands := append(location.Commands, prefixedCommands...)
			config.Locations[i].Commands = allCommands
		}
	}

	return nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}