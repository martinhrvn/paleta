package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPathResolutionFromSubdirectory tests that paths in config are resolved
// relative to the project root, not the current working directory.
// This reproduces a bug where running commands from a subdirectory would fail
// because paths were resolved relative to CWD instead of config root.
func TestPathResolutionFromSubdirectory(t *testing.T) {
	// Create a temp directory structure:
	// tmpDir/
	//   project/
	//     .gopmrc (with location: "packages/frontend")
	//     packages/
	//       frontend/
	//         package.json
	//     subdir/
	//       nested/ (we'll run from here)

	tmpDir, err := os.MkdirTemp("", "gopm-path-resolution-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directory structure
	projectDir := filepath.Join(tmpDir, "project")
	packagesDir := filepath.Join(projectDir, "packages")
	frontendDir := filepath.Join(packagesDir, "frontend")
	nestedDir := filepath.Join(projectDir, "subdir", "nested")

	dirs := []string{projectDir, packagesDir, frontendDir, nestedDir}
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	// Create a package.json in packages/frontend
	packageJSON := `{
  "name": "frontend",
  "scripts": {
    "start": "vite",
    "build": "vite build",
    "test": "vitest"
  }
}`
	packageJSONPath := filepath.Join(frontendDir, "package.json")
	err = os.WriteFile(packageJSONPath, []byte(packageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	// Create .gopmrc in project root with relative path
	configContent := `locations:
  - name: "frontend"
    location: "packages/frontend"
    type: "npm"`

	configPath := filepath.Join(projectDir, ".gopmrc")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Change to nested directory (NOT the project root)
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	err = os.Chdir(nestedDir)
	if err != nil {
		t.Fatalf("Failed to change to nested directory: %v", err)
	}

	// Load config from discovery (which should find .gopmrc in parent)
	config, err := LoadConfigFromDiscovery()
	if err != nil {
		t.Fatalf("LoadConfigFromDiscovery() error = %v", err)
	}

	// Verify config was loaded
	if len(config.Locations) != 1 {
		t.Fatalf("Expected 1 location, got %d", len(config.Locations))
	}

	location := config.Locations[0]

	// Verify location has the npm commands from package.json
	// This will only work if paths were resolved correctly relative to project root
	hasStartCommand := false
	hasBuildCommand := false
	hasTestCommand := false

	for _, cmd := range location.Commands {
		if cmd.Name == "start" {
			hasStartCommand = true
		}
		if cmd.Name == "build" {
			hasBuildCommand = true
		}
		if cmd.Name == "test" {
			hasTestCommand = true
		}
	}

	if !hasStartCommand {
		t.Error("Expected 'start' command from package.json, but it was not found")
		t.Logf("Available commands: %v", location.Commands)
		t.Logf("Current working directory: %s", nestedDir)
		t.Logf("Config root should be: %s", projectDir)
		t.Logf("Location path: %s", location.Location)
	}

	if !hasBuildCommand {
		t.Error("Expected 'build' command from package.json, but it was not found")
	}

	if !hasTestCommand {
		t.Error("Expected 'test' command from package.json, but it was not found")
	}

	// Verify the location path is absolute or correctly resolved
	// After the fix, Location should be an absolute path
	if !filepath.IsAbs(location.Location) {
		t.Logf("Warning: Location path is still relative: %s", location.Location)
		t.Logf("After the fix, this should be an absolute path")
	} else {
		// Verify it points to the correct directory
		expectedPath := frontendDir
		if location.Location != expectedPath {
			t.Errorf("Expected location to be %s, got %s", expectedPath, location.Location)
		}
	}
}
