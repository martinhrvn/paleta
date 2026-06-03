package parsers

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestUserConfigOverride(t *testing.T) {
	// Create a temporary directory for test
	tmpDir, err := ioutil.TempDir("", "plt_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	// Create .paleta directory
	paletaDir := filepath.Join(tmpDir, ".paleta")
	if err := os.MkdirAll(paletaDir, 0755); err != nil {
		t.Fatalf("Failed to create .paleta dir: %v", err)
	}
	
	// Create user config that overrides npm and adds a custom parser
	userConfig := `parsers:
  npm:
    detect_files: ["package.json"]
    base_commands:
      install: "npm install --legacy-peer-deps"
      custom: "npm run custom-command"
    builtin_parser: "package_json_scripts"
    command_template: "npm run {key}"
  
  custom:
    detect_files: ["custom.json"]
    base_commands:
      hello: "echo hello"
    command_template: "custom {key}"`
	
	configPath := filepath.Join(paletaDir, "parsers.yaml")
	if err := ioutil.WriteFile(configPath, []byte(userConfig), 0644); err != nil {
		t.Fatalf("Failed to write user config: %v", err)
	}
	
	// Mock the home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)
	
	// Load the config
	config, err := LoadParsersConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	// Test that npm parser was overridden
	npmParser, exists := config.GetParser("npm")
	if !exists {
		t.Fatal("Expected npm parser to exist")
	}
	
	// Check that the custom base command is present
	if npmParser.BaseCommands["custom"] != "npm run custom-command" {
		t.Errorf("Expected custom command to be overridden, got %s", npmParser.BaseCommands["custom"])
	}
	
	// Check that install command was overridden
	if npmParser.BaseCommands["install"] != "npm install --legacy-peer-deps" {
		t.Errorf("Expected install command to be overridden, got %s", npmParser.BaseCommands["install"])
	}
	
	// Test that custom parser was added
	customParser, exists := config.GetParser("custom")
	if !exists {
		t.Fatal("Expected custom parser to exist")
	}
	
	if customParser.BaseCommands["hello"] != "echo hello" {
		t.Errorf("Expected hello command in custom parser, got %s", customParser.BaseCommands["hello"])
	}
	
	// Test that default parsers still exist
	goParser, exists := config.GetParser("go")
	if !exists {
		t.Error("Expected go parser to still exist from defaults")
	}
	
	// Verify go parser wasn't overridden (should have default values)
	if goParser.BaseCommands["build"] != "go build ./..." {
		t.Errorf("Expected go build command to be default, got %s", goParser.BaseCommands["build"])
	}
	
	// Test that other default parsers exist
	defaultParsers := []string{"yarn", "pnpm", "python", "rust"}
	for _, name := range defaultParsers {
		if _, exists := config.GetParser(name); !exists {
			t.Errorf("Expected default parser %s to exist", name)
		}
	}
}

func TestUserConfigPartialOverride(t *testing.T) {
	// Create a temporary directory for test
	tmpDir, err := ioutil.TempDir("", "plt_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	// Create .paleta directory
	paletaDir := filepath.Join(tmpDir, ".paleta")
	if err := os.MkdirAll(paletaDir, 0755); err != nil {
		t.Fatalf("Failed to create .paleta dir: %v", err)
	}
	
	// Create user config that only overrides one parser
	userConfig := `parsers:
  go:
    detect_files: ["go.mod"]
    base_commands:
      build: "go build -v ./..."
      test: "go test -v ./..."
    builtin_parser: "go_standard"`
	
	configPath := filepath.Join(paletaDir, "parsers.yaml")
	if err := ioutil.WriteFile(configPath, []byte(userConfig), 0644); err != nil {
		t.Fatalf("Failed to write user config: %v", err)
	}
	
	// Mock the home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)
	
	// Load the config
	config, err := LoadParsersConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	// Test that go parser was overridden
	goParser, exists := config.GetParser("go")
	if !exists {
		t.Fatal("Expected go parser to exist")
	}
	
	// Check that build command was overridden
	if goParser.BaseCommands["build"] != "go build -v ./..." {
		t.Errorf("Expected build command to be overridden, got %s", goParser.BaseCommands["build"])
	}
	
	// Test that npm parser still has default values
	npmParser, exists := config.GetParser("npm")
	if !exists {
		t.Fatal("Expected npm parser to exist from defaults")
	}
	
	if npmParser.BaseCommands["install"] != "npm install" {
		t.Errorf("Expected npm install to be default, got %s", npmParser.BaseCommands["install"])
	}
}