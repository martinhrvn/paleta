package parsers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// PackageJsonParser parses package.json scripts
type PackageJsonParser struct{}

func (p *PackageJsonParser) ParseCommands(directory string, config ParserConfig) ([]string, error) {
	packageJsonPath := filepath.Join(directory, "package.json")
	return parsePackageJsonScripts(packageJsonPath)
}

// GoStandardParser provides standard Go commands
type GoStandardParser struct{}

func (g *GoStandardParser) ParseCommands(directory string, config ParserConfig) ([]string, error) {
	// For Go projects, we don't parse the go.mod file for commands
	// Instead, we return standard Go commands that are commonly used
	return []string{
		"run",
		"build",
		"test",
		"fmt",
		"vet",
		"mod",
		"clean",
		"install",
		"get",
		"generate",
		"doc",
		"version",
	}, nil
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
