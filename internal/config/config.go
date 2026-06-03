package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/martinhrvn/paleta/internal/projecttypes"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Root      string          `yaml:"root,omitempty"`
	Locations []Location      `yaml:"locations"`
	Frecency  FrecencyConfig  `yaml:"frecency,omitempty"`
}

// FrecencyConfig configures frecency sorting behavior
type FrecencyConfig struct {
	Enabled         bool    `yaml:"enabled"`
	RecencyWeight   float64 `yaml:"recency_weight"`
	FrequencyWeight float64 `yaml:"frequency_weight"`
}

// DefaultFrecencyConfig returns the default frecency configuration
func DefaultFrecencyConfig() FrecencyConfig {
	return FrecencyConfig{
		Enabled:         true,
		RecencyWeight:   0.5,
		FrequencyWeight: 0.5,
	}
}

type Command struct {
	Name    string            `yaml:"name,omitempty"`
	Command string            `yaml:"command"`
	Env     map[string]string `yaml:"env,omitempty"`
}

// UnmarshalYAML implements custom YAML unmarshaling for Command
// Supports both string format (old) and object format (new)
func (c *Command) UnmarshalYAML(value *yaml.Node) error {
	// Try to unmarshal as a string (old format)
	var cmdString string
	if err := value.Decode(&cmdString); err == nil {
		c.Command = cmdString
		c.Name = "" // No name for old format
		return nil
	}

	// Try to unmarshal as an object (new format)
	type commandAlias Command
	var cmd commandAlias
	if err := value.Decode(&cmd); err != nil {
		return err
	}
	*c = Command(cmd)
	return nil
}

type Location struct {
	Name     string            `yaml:"name,omitempty"`
	Location string            `yaml:"location,omitempty"`
	Type     string            `yaml:"type,omitempty"`
	Commands []Command         `yaml:"commands,omitempty"`
	Include  []string          `yaml:"include,omitempty"`
	Exclude  []string          `yaml:"exclude,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
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

	// Apply default frecency config if not specified
	applyDefaultFrecencyConfig(&config)

	// Normalize empty paths to current directory
	normalizeEmptyPaths(&config)

	// Expand glob patterns in locations
	expandedLocations, err := ExpandGlobPatterns(config.Locations)
	if err != nil {
		return nil, fmt.Errorf("failed to expand glob patterns: %w", err)
	}
	config.Locations = expandedLocations

	// Convert all location paths to absolute paths
	// This must happen while we're still in the config directory
	if err := makeLocationPathsAbsolute(&config); err != nil {
		return nil, fmt.Errorf("failed to make location paths absolute: %w", err)
	}

	// Process project types and add their commands
	if err := processProjectTypes(&config); err != nil {
		return nil, fmt.Errorf("failed to process project types: %w", err)
	}

	return &config, nil
}

// applyDefaultFrecencyConfig applies default frecency settings if not configured
func applyDefaultFrecencyConfig(config *Config) {
	// If frecency config is missing, use defaults
	if config.Frecency.RecencyWeight == 0 && config.Frecency.FrequencyWeight == 0 {
		config.Frecency = DefaultFrecencyConfig()
	}
}

// normalizeEmptyPaths normalizes empty location paths to "." (current directory)
func normalizeEmptyPaths(config *Config) {
	for i := range config.Locations {
		if config.Locations[i].Location == "" {
			config.Locations[i].Location = "."
		}
	}
}

// makeLocationPathsAbsolute converts all relative location paths to absolute paths
// For local configs: uses current working directory (should be config directory)
// For global configs: uses config.Root as base directory
func makeLocationPathsAbsolute(config *Config) error {
	for i := range config.Locations {
		if config.Locations[i].Location == "" {
			continue
		}

		var absPath string

		// If config has a Root field (global projects), use that as base
		if config.Root != "" {
			// Join location with root directory
			absPath = filepath.Join(config.Root, config.Locations[i].Location)
		} else {
			// Use current working directory (local .pltrc case)
			var err error
			absPath, err = filepath.Abs(config.Locations[i].Location)
			if err != nil {
				return fmt.Errorf("failed to get absolute path for location %q: %w", config.Locations[i].Location, err)
			}
		}

		config.Locations[i].Location = absPath
	}
	return nil
}

// filterCommands filters commands based on include and exclude patterns
// Include patterns act as a whitelist (if specified)
// Exclude patterns act as a blacklist (applied after include)
// Both support glob patterns using filepath.Match syntax
// Patterns match against command name (if present) or command string
func filterCommands(commands []Command, include []string, exclude []string) []Command {
	if len(commands) == 0 {
		return []Command{}
	}

	// If no filters specified, return all commands
	if len(include) == 0 && len(exclude) == 0 {
		return commands
	}

	var result []Command

	// Apply include filter (whitelist)
	if len(include) > 0 {
		for _, cmd := range commands {
			// Match against command name if present, otherwise against command string
			matchString := cmd.Name
			if matchString == "" {
				matchString = cmd.Command
			}
			for _, pattern := range include {
				matched, err := filepath.Match(pattern, matchString)
				if err == nil && matched {
					result = append(result, cmd)
					break // Command matched, no need to check other patterns
				}
			}
		}
	} else {
		// No include filter, start with all commands
		result = append(result, commands...)
	}

	// Apply exclude filter (blacklist)
	if len(exclude) > 0 {
		filtered := make([]Command, 0, len(result))
		for _, cmd := range result {
			// Match against command name if present, otherwise against command string
			matchString := cmd.Name
			if matchString == "" {
				matchString = cmd.Command
			}
			shouldExclude := false
			for _, pattern := range exclude {
				matched, err := filepath.Match(pattern, matchString)
				if err == nil && matched {
					shouldExclude = true
					break
				}
			}
			if !shouldExclude {
				filtered = append(filtered, cmd)
			}
		}
		result = filtered
	}

	return result
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
			// Get all commands as a map (key = name, value = full command)
			commands, err := configurableType.GetAllCommands(location.Location)
			if err != nil {
				return fmt.Errorf("failed to parse commands for location %s: %w", location.Location, err)
			}

			// Convert to command list format with names
			var commandList []Command
			for name, cmd := range commands {
				commandList = append(commandList, Command{
					Name:    name,
					Command: cmd,
				})
			}

			// Filter discovered commands based on include/exclude patterns
			filteredCommands := filterCommands(commandList, location.Include, location.Exclude)

			// Merge filtered auto-discovered commands with manual commands (manual first)
			allCommands := append(location.Commands, filteredCommands...)
			config.Locations[i].Commands = allCommands
		} else {
			// Fallback to old behavior for backward compatibility
			// Parse commands from the project type
			projectCommands, err := projectType.ParseCommands(configFile)
			if err != nil {
				return fmt.Errorf("failed to parse commands for location %s: %w", location.Location, err)
			}

			// Prefix the commands with the project type command prefix
			var commandList []Command
			for _, cmd := range projectCommands {
				var fullCmd string
				if projectType.GetCommandPrefix() != "" {
					fullCmd = fmt.Sprintf("%s %s", projectType.GetCommandPrefix(), cmd)
				} else {
					fullCmd = cmd
				}
				// No name available for fallback path
				commandList = append(commandList, Command{
					Name:    "",
					Command: fullCmd,
				})
			}

			// Filter discovered commands based on include/exclude patterns
			filteredCommands := filterCommands(commandList, location.Include, location.Exclude)

			// Merge filtered auto-discovered commands with manual commands
			allCommands := append(location.Commands, filteredCommands...)
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

// LoadGlobalConfig loads the global configuration from ~/.config/paleta/config.yaml
func LoadGlobalConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// If home directory can't be determined, return empty config
		return &Config{
			Frecency: DefaultFrecencyConfig(),
		}, nil
	}

	globalConfigPath := filepath.Join(homeDir, ".config", "paleta", "config.yaml")

	// If global config doesn't exist, return defaults
	if !fileExists(globalConfigPath) {
		return &Config{
			Frecency: DefaultFrecencyConfig(),
		}, nil
	}

	// Load global config
	config, err := LoadConfig(globalConfigPath)
	if err != nil {
		// If there's an error loading, return defaults
		return &Config{
			Frecency: DefaultFrecencyConfig(),
		}, nil
	}

	return config, nil
}

// MergeFrecencyConfig merges frecency settings, with local config taking precedence
func MergeFrecencyConfig(global, local FrecencyConfig) FrecencyConfig {
	result := global

	// If local config has non-zero values, use them (they override global)
	// Note: We need to distinguish between "not set" and "set to false/0"
	// For simplicity, we'll check if any local config is different from zero values
	if local.RecencyWeight != 0 || local.FrequencyWeight != 0 {
		result.RecencyWeight = local.RecencyWeight
		result.FrequencyWeight = local.FrequencyWeight
	}

	// Enabled can be explicitly set to false, so we need special handling
	// If both weights are zero in local, we assume frecency config wasn't specified locally
	localConfigSpecified := local.RecencyWeight != 0 || local.FrequencyWeight != 0
	if localConfigSpecified {
		result.Enabled = local.Enabled
	}

	return result
}

// LoadConfigWithGlobal loads both global and local configs and merges them
func LoadConfigWithGlobal(localConfigPath string) (*Config, error) {
	// Load global config
	globalConfig, _ := LoadGlobalConfig()

	// Load local config
	localConfig, err := LoadConfig(localConfigPath)
	if err != nil {
		return nil, err
	}

	// Merge frecency settings (local overrides global)
	localConfig.Frecency = MergeFrecencyConfig(globalConfig.Frecency, localConfig.Frecency)

	return localConfig, nil
}