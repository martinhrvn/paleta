package projecttypes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// NpmProjectType implements ProjectType for npm projects
type NpmProjectType struct{}

func (n *NpmProjectType) Name() string {
	return "npm"
}

func (n *NpmProjectType) DetectConfigFile() string {
	return "package.json"
}

func (n *NpmProjectType) ParseCommands(configPath string) ([]string, error) {
	return parsePackageJsonScripts(configPath)
}

func (n *NpmProjectType) GetCommandPrefix() string {
	return "npm run"
}

func (n *NpmProjectType) CanHandleDirectory(directory string) bool {
	return fileExists(filepath.Join(directory, n.DetectConfigFile()))
}

// YarnProjectType implements ProjectType for yarn projects
type YarnProjectType struct{}

func (y *YarnProjectType) Name() string {
	return "yarn"
}

func (y *YarnProjectType) DetectConfigFile() string {
	return "package.json"
}

func (y *YarnProjectType) ParseCommands(configPath string) ([]string, error) {
	return parsePackageJsonScripts(configPath)
}

func (y *YarnProjectType) GetCommandPrefix() string {
	return "yarn"
}

func (y *YarnProjectType) CanHandleDirectory(directory string) bool {
	return fileExists(filepath.Join(directory, y.DetectConfigFile()))
}

// PnpmProjectType implements ProjectType for pnpm projects
type PnpmProjectType struct{}

func (p *PnpmProjectType) Name() string {
	return "pnpm"
}

func (p *PnpmProjectType) DetectConfigFile() string {
	return "package.json"
}

func (p *PnpmProjectType) ParseCommands(configPath string) ([]string, error) {
	return parsePackageJsonScripts(configPath)
}

func (p *PnpmProjectType) GetCommandPrefix() string {
	return "pnpm run"
}

func (p *PnpmProjectType) CanHandleDirectory(directory string) bool {
	return fileExists(filepath.Join(directory, p.DetectConfigFile()))
}

// PackageJson represents the structure of a package.json file
type PackageJson struct {
	Name    string                 `json:"name"`
	Scripts map[string]interface{} `json:"scripts"`
}

// parsePackageJsonScripts parses a package.json file and extracts script names
func parsePackageJsonScripts(configPath string) ([]string, error) {
	// Read the package.json file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read package.json: %w", err)
	}

	// Parse JSON
	var packageJson PackageJson
	if err := json.Unmarshal(data, &packageJson); err != nil {
		return nil, fmt.Errorf("failed to parse package.json: %w", err)
	}

	// Extract script names (only string values)
	var commands []string
	for scriptName, scriptValue := range packageJson.Scripts {
		// Only include scripts that have string values
		if _, ok := scriptValue.(string); ok {
			commands = append(commands, scriptName)
		}
	}

	// Sort commands for consistent output
	sort.Strings(commands)

	return commands, nil
}
