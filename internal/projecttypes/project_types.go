package projecttypes

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/martinhrvn/paleta/internal/parsers"
)

// ProjectType represents a project type that can discover and provide commands
type ProjectType interface {
	// Name returns the name of the project type (e.g., "npm", "go")
	Name() string
	
	// DetectConfigFile returns the config file name to look for (e.g., "package.json", "go.mod")
	DetectConfigFile() string
	
	// ParseCommands parses the config file and returns available commands
	ParseCommands(configPath string) ([]string, error)
	
	// GetCommandPrefix returns the command prefix for this project type (e.g., "npm run", "go")
	GetCommandPrefix() string
}

// ProjectTypeRegistry holds all registered project types
var ProjectTypeRegistry = map[string]ProjectType{}
var registryMutex sync.RWMutex
var registryInitialized bool

// initializeRegistry initializes the registry with parsers from configuration
func initializeRegistry() error {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	if registryInitialized {
		return nil
	}

	// Load parser configurations
	parsersConfig, err := parsers.LoadParsersConfig()
	if err != nil {
		return fmt.Errorf("failed to load parsers config: %w", err)
	}

	// Create configurable project types from parser configurations
	for name, config := range parsersConfig.Parsers {
		ProjectTypeRegistry[name] = NewConfigurableProjectType(name, config)
	}

	registryInitialized = true
	return nil
}

// GetProjectType returns a project type by name
func GetProjectType(name string) (ProjectType, error) {
	if err := initializeRegistry(); err != nil {
		return nil, err
	}

	registryMutex.RLock()
	defer registryMutex.RUnlock()

	projectType, exists := ProjectTypeRegistry[name]
	if !exists {
		return nil, fmt.Errorf("unknown project type: %s", name)
	}
	return projectType, nil
}

// DiscoverProjectType attempts to discover the project type in a directory
func DiscoverProjectType(directory string) (ProjectType, error) {
	if err := initializeRegistry(); err != nil {
		return nil, err
	}

	registryMutex.RLock()
	defer registryMutex.RUnlock()

	for _, projectType := range ProjectTypeRegistry {
		configFile := filepath.Join(directory, projectType.DetectConfigFile())
		if fileExists(configFile) {
			return projectType, nil
		}
	}
	return nil, fmt.Errorf("no project type detected in directory: %s", directory)
}

// ListAvailableTypes returns all available project types
func ListAvailableTypes() ([]string, error) {
	if err := initializeRegistry(); err != nil {
		return nil, err
	}

	registryMutex.RLock()
	defer registryMutex.RUnlock()

	var types []string
	for name := range ProjectTypeRegistry {
		types = append(types, name)
	}
	return types, nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

