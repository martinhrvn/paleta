package projecttypes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNpmProjectType(t *testing.T) {
	npmType := &NpmProjectType{}

	// Test basic properties
	if npmType.Name() != "npm" {
		t.Errorf("Expected name 'npm', got %s", npmType.Name())
	}

	if npmType.DetectConfigFile() != "package.json" {
		t.Errorf("Expected config file 'package.json', got %s", npmType.DetectConfigFile())
	}

	if npmType.GetCommandPrefix() != "npm run" {
		t.Errorf("Expected command prefix 'npm run', got %s", npmType.GetCommandPrefix())
	}
}

func TestNpmParseCommands(t *testing.T) {
	npmType := &NpmProjectType{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "plt-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name         string
		packageJson  map[string]interface{}
		expectedCmds []string
		expectError  bool
	}{
		{
			name: "basic scripts",
			packageJson: map[string]interface{}{
				"name": "test-package",
				"scripts": map[string]interface{}{
					"start": "node server.js",
					"build": "webpack",
					"test":  "jest",
				},
			},
			expectedCmds: []string{"start", "build", "test"},
			expectError:  false,
		},
		{
			name: "no scripts section",
			packageJson: map[string]interface{}{
				"name": "test-package",
			},
			expectedCmds: []string{},
			expectError:  false,
		},
		{
			name: "empty scripts section",
			packageJson: map[string]interface{}{
				"name":    "test-package",
				"scripts": map[string]interface{}{},
			},
			expectedCmds: []string{},
			expectError:  false,
		},
		{
			name: "scripts with non-string values",
			packageJson: map[string]interface{}{
				"name": "test-package",
				"scripts": map[string]interface{}{
					"start": "node server.js",
					"build": 123, // Invalid non-string value
					"test":  "jest",
				},
			},
			expectedCmds: []string{"start", "test"}, // Should skip invalid entries
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create package.json file
			packageJsonPath := filepath.Join(tempDir, "package.json")
			jsonData, err := json.Marshal(tt.packageJson)
			if err != nil {
				t.Fatalf("Failed to marshal JSON: %v", err)
			}

			err = os.WriteFile(packageJsonPath, jsonData, 0644)
			if err != nil {
				t.Fatalf("Failed to write package.json: %v", err)
			}

			// Test parsing
			commands, err := npmType.ParseCommands(packageJsonPath)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if len(commands) != len(tt.expectedCmds) {
				t.Errorf("Expected %d commands, got %d", len(tt.expectedCmds), len(commands))
			}

			// Check that all expected commands are present
			commandSet := make(map[string]bool)
			for _, cmd := range commands {
				commandSet[cmd] = true
			}

			for _, expectedCmd := range tt.expectedCmds {
				if !commandSet[expectedCmd] {
					t.Errorf("Expected command %s not found in result", expectedCmd)
				}
			}
		})
	}
}

func TestNpmParseCommandsFileNotFound(t *testing.T) {
	npmType := &NpmProjectType{}

	_, err := npmType.ParseCommands("/nonexistent/package.json")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestNpmParseCommandsInvalidJSON(t *testing.T) {
	npmType := &NpmProjectType{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "plt-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create invalid JSON file
	packageJsonPath := filepath.Join(tempDir, "package.json")
	err = os.WriteFile(packageJsonPath, []byte("invalid json content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	_, err = npmType.ParseCommands(packageJsonPath)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestYarnProjectType(t *testing.T) {
	yarnType := &YarnProjectType{}

	if yarnType.Name() != "yarn" {
		t.Errorf("Expected name 'yarn', got %s", yarnType.Name())
	}

	if yarnType.DetectConfigFile() != "package.json" {
		t.Errorf("Expected config file 'package.json', got %s", yarnType.DetectConfigFile())
	}

	if yarnType.GetCommandPrefix() != "yarn" {
		t.Errorf("Expected command prefix 'yarn', got %s", yarnType.GetCommandPrefix())
	}
}

func TestPnpmProjectType(t *testing.T) {
	pnpmType := &PnpmProjectType{}

	if pnpmType.Name() != "pnpm" {
		t.Errorf("Expected name 'pnpm', got %s", pnpmType.Name())
	}

	if pnpmType.DetectConfigFile() != "package.json" {
		t.Errorf("Expected config file 'package.json', got %s", pnpmType.DetectConfigFile())
	}

	if pnpmType.GetCommandPrefix() != "pnpm run" {
		t.Errorf("Expected command prefix 'pnpm run', got %s", pnpmType.GetCommandPrefix())
	}
}

func TestGetProjectType(t *testing.T) {
	tests := []struct {
		name        string
		typeName    string
		expectError bool
	}{
		{"npm type", "npm", false},
		{"yarn type", "yarn", false},
		{"pnpm type", "pnpm", false},
		{"unknown type", "unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectType, err := GetProjectType(tt.typeName)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && projectType.Name() != tt.typeName {
				t.Errorf("Expected type name %s, got %s", tt.typeName, projectType.Name())
			}
		})
	}
}

func TestDiscoverProjectType(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "plt-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test with package.json
	packageJsonPath := filepath.Join(tempDir, "package.json")
	err = os.WriteFile(packageJsonPath, []byte(`{"name": "test"}`), 0644)
	if err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	projectType, err := DiscoverProjectType(tempDir)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should discover one of the package.json-based types (npm, yarn, or pnpm)
	validTypes := map[string]bool{"npm": true, "yarn": true, "pnpm": true}
	if !validTypes[projectType.Name()] {
		t.Errorf("Expected npm, yarn, or pnpm type, got %s", projectType.Name())
	}

	// Test with no config files
	emptyDir, err := os.MkdirTemp("", "plt-test-empty-")
	if err != nil {
		t.Fatalf("Failed to create empty temp dir: %v", err)
	}
	defer os.RemoveAll(emptyDir)

	_, err = DiscoverProjectType(emptyDir)
	if err == nil {
		t.Error("Expected error for directory with no project files")
	}
}

func TestCanHandleDirectory_Glob(t *testing.T) {
	compose, err := GetProjectType("compose")
	if err != nil {
		t.Fatalf("GetProjectType(compose): %v", err)
	}

	// Directory with only an env-specific override file (glob match).
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.prod.yml"), []byte("services: {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if !compose.CanHandleDirectory(dir) {
		t.Error("compose should handle a dir with docker-compose.prod.yml")
	}

	// Empty directory must not match.
	empty := t.TempDir()
	if compose.CanHandleDirectory(empty) {
		t.Error("compose should not handle an empty dir")
	}
}
