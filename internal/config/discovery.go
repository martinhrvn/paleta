package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const ConfigFileName = ".pltrc"

// FindConfigFile searches for .pltrc starting from the current working directory
// and traversing up the directory tree until it finds one or reaches the root.
func FindConfigFile() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	return findConfigFileFromPath(cwd)
}

// findConfigFileFromPath searches for .pltrc starting from the given path
// and traversing up the directory tree.
func findConfigFileFromPath(startPath string) (string, error) {
	currentPath := startPath
	
	for {
		configPath := filepath.Join(currentPath, ConfigFileName)
		
		// Check if config file exists
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
		
		// Move to parent directory
		parentPath := filepath.Dir(currentPath)
		
		// Check if we've reached the root directory
		if parentPath == currentPath {
			break
		}
		
		currentPath = parentPath
	}
	
	return "", fmt.Errorf("no %s found in current directory or any parent directories", ConfigFileName)
}

// LoadConfigFromDiscovery finds and loads the nearest .pltrc file
// If no local .pltrc is found, it falls back to global project configurations
func LoadConfigFromDiscovery() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	projectsDir := filepath.Join(homeDir, ".config", "paleta", "projects")
	return loadConfigFromDiscoveryWithGlobalFallback(projectsDir)
}

// loadConfigFromDiscoveryWithGlobalFallback is a helper that allows testing with custom projects directory
func loadConfigFromDiscoveryWithGlobalFallback(projectsDir string) (*Config, error) {
	// Try to find local .pltrc first
	configPath, err := FindConfigFile()

	if err == nil {
		// Local config found - use it with global frecency settings
		configDir := filepath.Dir(configPath)
		oldWd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %w", err)
		}
		defer os.Chdir(oldWd)

		err = os.Chdir(configDir)
		if err != nil {
			return nil, fmt.Errorf("failed to change to config directory %q: %w", configDir, err)
		}

		return LoadConfigWithGlobal(configPath)
	}

	// No local config found - try global projects
	projects, err := loadGlobalProjectsFromDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load global projects: %w", err)
	}

	// Find matching project for current directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	matchedProject := FindMatchingProject(cwd, projects)
	if matchedProject == nil {
		return nil, fmt.Errorf("no %s found in current directory or any parent directories, and no matching global project configuration", ConfigFileName)
	}

	// Change to the project root directory for glob expansion
	oldWd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}
	defer os.Chdir(oldWd)

	err = os.Chdir(matchedProject.Root)
	if err != nil {
		return nil, fmt.Errorf("failed to change to project root directory %q: %w", matchedProject.Root, err)
	}

	// The matched project is already loaded and processed
	// We need to merge with global frecency settings
	globalConfig, _ := LoadGlobalConfig()
	matchedProject.Frecency = MergeFrecencyConfig(globalConfig.Frecency, matchedProject.Frecency)

	return matchedProject, nil
}

// LoadGlobalProjects loads all project configurations from ~/.config/paleta/projects/
func LoadGlobalProjects() ([]*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	projectsDir := filepath.Join(homeDir, ".config", "paleta", "projects")
	return loadGlobalProjectsFromDir(projectsDir)
}

// loadGlobalProjectsFromDir loads all project configurations from the specified directory
func loadGlobalProjectsFromDir(projectsDir string) ([]*Config, error) {
	// Check if directory exists
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		// Directory doesn't exist, return empty list
		return []*Config{}, nil
	}

	// Read directory entries
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read projects directory: %w", err)
	}

	var configs []*Config

	// Load each YAML file
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .yaml and .yml files
		name := entry.Name()
		if filepath.Ext(name) != ".yaml" && filepath.Ext(name) != ".yml" {
			continue
		}

		projectPath := filepath.Join(projectsDir, name)
		config, err := LoadConfig(projectPath)
		if err != nil {
			// Skip files that fail to load
			continue
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// FindMatchingProject finds a project configuration whose root is a parent of the current working directory
// If multiple projects match, returns the one with the longest (most specific) root path
func FindMatchingProject(cwd string, projects []*Config) *Config {
	var bestMatch *Config
	var bestMatchDepth int

	for _, project := range projects {
		if project.Root == "" {
			continue
		}

		// Check if project root is a parent of or equal to cwd
		rel, err := filepath.Rel(project.Root, cwd)
		if err != nil {
			continue
		}

		// If rel starts with "..", then cwd is not under project.Root
		if len(rel) >= 2 && rel[0] == '.' && rel[1] == '.' {
			continue
		}

		// Calculate depth (number of path components in root)
		// Clean the path and count separators
		cleanRoot := filepath.Clean(project.Root)
		depth := len(filepath.SplitList(cleanRoot))
		if filepath.IsAbs(cleanRoot) {
			// For absolute paths, count the number of directory levels
			depth = 0
			for p := cleanRoot; p != "/" && p != "."; p = filepath.Dir(p) {
				depth++
			}
		}

		// Keep the match with the longest (deepest) root path
		if bestMatch == nil || depth > bestMatchDepth {
			bestMatch = project
			bestMatchDepth = depth
		}
	}

	return bestMatch
}