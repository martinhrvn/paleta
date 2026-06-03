package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigWithGlobExpansion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "plt-config-glob-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test directories
	testDirs := []string{
		"packages/frontend",
		"packages/backend", 
		"packages/shared",
	}
	for _, dir := range testDirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		if err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	// Create config file with glob pattern
	configYAML := `locations:
  - name: "services"
    location: "packages/*"
    type: "npm"
    commands:
      - "start"
      - "build"`

	configPath := filepath.Join(tmpDir, ".pltrc")
	err = os.WriteFile(configPath, []byte(configYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Change to temp directory for glob expansion
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Load config
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify glob expansion worked
	// Paths should now be absolute
	expectedLocations := []string{
		filepath.Join(tmpDir, "packages/backend"),
		filepath.Join(tmpDir, "packages/frontend"),
		filepath.Join(tmpDir, "packages/shared"),
	}

	if len(config.Locations) != len(expectedLocations) {
		t.Errorf("Expected %d locations, got %d", len(expectedLocations), len(config.Locations))
	}

	for i, loc := range config.Locations {
		if loc.Name != "services" {
			t.Errorf("Location[%d].Name = %q, expected %q", i, loc.Name, "services")
		}
		if loc.Location != expectedLocations[i] {
			t.Errorf("Location[%d].Location = %q, expected %q", i, loc.Location, expectedLocations[i])
		}
		if loc.Type != "npm" {
			t.Errorf("Location[%d].Type = %q, expected %q", i, loc.Type, "npm")
		}
		expectedCommands := []string{"start", "build"}
		if len(loc.Commands) != len(expectedCommands) {
			t.Errorf("Location[%d] has %d commands, expected %d", i, len(loc.Commands), len(expectedCommands))
		}
		for j, cmd := range loc.Commands {
			if cmd.Command != expectedCommands[j] {
				t.Errorf("Location[%d].Commands[%d].Command = %q, expected %q", i, j, cmd.Command, expectedCommands[j])
			}
		}
	}
}

func TestLoadConfigWithInvalidGlobPattern(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "plt-config-invalid-glob-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config file with invalid glob pattern
	configYAML := `locations:
  - name: "services"
    location: "foo/*/bar/*"
    type: "npm"
    commands:
      - "start"`

	configPath := filepath.Join(tmpDir, ".pltrc")
	err = os.WriteFile(configPath, []byte(configYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Load config should fail with validation error
	_, err = LoadConfig(configPath)
	if err == nil {
		t.Errorf("Expected error for invalid glob pattern, but got none")
	}

	if !strings.Contains(err.Error(), "multiple asterisks not supported") {
		t.Errorf("Expected error message about multiple asterisks, got: %v", err)
	}
}