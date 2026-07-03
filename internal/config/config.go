package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/martinhrvn/paleta/internal/projecttypes"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Root      string         `yaml:"root,omitempty"`
	Locations []Location     `yaml:"locations"`
	Frecency  FrecencyConfig `yaml:"frecency,omitempty"`
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
	// Type is the project type that produced this command (e.g. "npm",
	// "compose"). It is set internally by processProjectTypes and is never read
	// from or written to YAML. Empty for manually authored commands.
	Type string `yaml:"-"`
	// Error is set by expandCommandAliases when this command's references can't
	// be resolved (e.g. a referenced command was renamed or an ambiguous saved
	// chain). The command keeps its authored text so one bad reference never
	// blocks loading the rest; callers surface it rather than run it. Never
	// serialized.
	Error string `yaml:"-"`
}

// Types is a location's set of project types. In .pltrc the `type` key accepts
// either a single value (`type: npm`) or a list (`type: [npm, docker]`); both
// decode into this slice. A single-element slice re-marshals as a scalar so
// existing configs round-trip unchanged.
type Types []string

// UnmarshalYAML accepts either a scalar string or a sequence of strings.
// Whitespace is trimmed and empty entries are dropped.
func (t *Types) UnmarshalYAML(value *yaml.Node) error {
	var single string
	if err := value.Decode(&single); err == nil {
		*t = normalizeTypes([]string{single})
		return nil
	}

	var list []string
	if err := value.Decode(&list); err != nil {
		return err
	}
	*t = normalizeTypes(list)
	return nil
}

// MarshalYAML emits a scalar for a single type and a sequence for several, so a
// single-type location serializes as `type: npm` rather than `type: [npm]`.
func (t Types) MarshalYAML() (any, error) {
	switch len(t) {
	case 0:
		return nil, nil
	case 1:
		return t[0], nil
	default:
		return []string(t), nil
	}
}

// normalizeTypes trims whitespace, drops empty entries, and de-duplicates while
// preserving first-seen order.
func normalizeTypes(in []string) Types {
	var out Types
	seen := make(map[string]bool)
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
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
	Types    Types             `yaml:"type,omitempty"`
	Commands []Command         `yaml:"commands,omitempty"`
	Include  []string          `yaml:"include,omitempty"`
	Exclude  []string          `yaml:"exclude,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	// Focused marks this location as part of the user's "focus" set. When any
	// location is focused, the selector defaults to showing only focused ones.
	Focused bool `yaml:"focused,omitempty"`
}

// AnyFocused reports whether any location is marked focused.
func (c *Config) AnyFocused() bool {
	for i := range c.Locations {
		if c.Locations[i].Focused {
			return true
		}
	}
	return false
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

	// Expand @project[type]:command references into resolved command strings. An
	// unresolvable reference is recorded on the command (Command.Error) rather
	// than failing the whole load, so one stale saved chain never blocks the rest
	// of the config from loading and running.
	expandCommandAliases(&config)

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

// CommandLabel returns the display label for a command within its location: the
// command's name (falling back to the raw command string). When the location has
// more than one type, type-derived commands are prefixed with "[type] " so
// commands sharing a name across types (e.g. npm `build` vs docker `build`) stay
// distinguishable. Single-type locations and manually authored commands render
// exactly as before.
func CommandLabel(loc Location, cmd Command) string {
	name := cmd.Name
	if name == "" {
		name = cmd.Command
	}
	if len(loc.Types) > 1 && cmd.Type != "" {
		return "[" + cmd.Type + "] " + name
	}
	return name
}

// processProjectTypes processes project types and adds their commands to locations
func processProjectTypes(config *Config) error {
	for i, location := range config.Locations {
		if len(location.Types) == 0 {
			continue
		}

		// Accumulate discovered commands across every type the location declares.
		var discovered []Command
		for _, typeName := range location.Types {
			cmds, err := commandsForType(typeName, location.Location)
			if err != nil {
				return err
			}
			discovered = append(discovered, cmds...)
		}

		// Stable order (map iteration in the parsers is non-deterministic, and we
		// merge several maps). Sort by type, then name.
		sort.SliceStable(discovered, func(a, b int) bool {
			if discovered[a].Type != discovered[b].Type {
				return discovered[a].Type < discovered[b].Type
			}
			return discovered[a].Name < discovered[b].Name
		})

		// Filter discovered commands based on include/exclude patterns.
		filteredCommands := filterCommands(discovered, location.Include, location.Exclude)

		// Merge filtered auto-discovered commands with manual commands (manual first).
		config.Locations[i].Commands = append(location.Commands, filteredCommands...)
	}

	return nil
}

// commandsForType resolves a single project type in a directory and returns its
// commands, each tagged with the type. A type whose detect file is absent
// contributes nothing (the location may declare several types, only some of which
// apply here).
func commandsForType(typeName, directory string) ([]Command, error) {
	projectType, err := projecttypes.GetProjectType(typeName)
	if err != nil {
		return nil, fmt.Errorf("location %s has invalid type: %w", directory, err)
	}

	if !projectType.CanHandleDirectory(directory) {
		return nil, nil
	}

	// For configurable project types, get the full commands directly.
	if configurableType, ok := projectType.(*projecttypes.ConfigurableProjectType); ok {
		commands, err := configurableType.GetAllCommands(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to parse commands for location %s: %w", directory, err)
		}

		var commandList []Command
		for name, cmd := range commands {
			commandList = append(commandList, Command{
				Name:    name,
				Command: cmd,
				Type:    typeName,
			})
		}
		return commandList, nil
	}

	// Fallback to old behavior for backward compatibility.
	configFile := filepath.Join(directory, projectType.DetectConfigFile())
	projectCommands, err := projectType.ParseCommands(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse commands for location %s: %w", directory, err)
	}

	var commandList []Command
	for _, cmd := range projectCommands {
		fullCmd := cmd
		if projectType.GetCommandPrefix() != "" {
			fullCmd = fmt.Sprintf("%s %s", projectType.GetCommandPrefix(), cmd)
		}
		commandList = append(commandList, Command{
			Name:    "",
			Command: fullCmd,
			Type:    typeName,
		})
	}
	return commandList, nil
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
