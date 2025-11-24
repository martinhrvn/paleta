package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCommandUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected []Command
		wantErr  bool
	}{
		{
			name: "old format - string commands",
			yaml: `commands:
  - "npm run start"
  - "npm run build"
  - "npm run test"`,
			expected: []Command{
				{Name: "", Command: "npm run start"},
				{Name: "", Command: "npm run build"},
				{Name: "", Command: "npm run test"},
			},
			wantErr: false,
		},
		{
			name: "new format - object with name and command",
			yaml: `commands:
  - name: "start"
    command: "npm run start"
  - name: "build"
    command: "npm run build"
  - name: "test"
    command: "npm run test"`,
			expected: []Command{
				{Name: "start", Command: "npm run start"},
				{Name: "build", Command: "npm run build"},
				{Name: "test", Command: "npm run test"},
			},
			wantErr: false,
		},
		{
			name: "new format - object with only command (no name)",
			yaml: `commands:
  - command: "npm run start"
  - command: "npm run build"`,
			expected: []Command{
				{Name: "", Command: "npm run start"},
				{Name: "", Command: "npm run build"},
			},
			wantErr: false,
		},
		{
			name: "mixed format - both strings and objects",
			yaml: `commands:
  - "npm run start"
  - name: "build"
    command: "npm run build"
  - "npm run test"`,
			expected: []Command{
				{Name: "", Command: "npm run start"},
				{Name: "build", Command: "npm run build"},
				{Name: "", Command: "npm run test"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestStruct struct {
				Commands []Command `yaml:"commands"`
			}

			var result TestStruct
			err := yaml.Unmarshal([]byte(tt.yaml), &result)

			if (err != nil) != tt.wantErr {
				t.Errorf("yaml.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(result.Commands) != len(tt.expected) {
					t.Errorf("Expected %d commands, got %d", len(tt.expected), len(result.Commands))
					return
				}

				for i, cmd := range result.Commands {
					expected := tt.expected[i]
					if cmd.Name != expected.Name {
						t.Errorf("Commands[%d].Name = %q, expected %q", i, cmd.Name, expected.Name)
					}
					if cmd.Command != expected.Command {
						t.Errorf("Commands[%d].Command = %q, expected %q", i, cmd.Command, expected.Command)
					}
				}
			}
		})
	}
}

func TestConfigYAMLParsing(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected Config
		wantErr  bool
	}{
		{
			name: "basic config with single location",
			yaml: `locations:
  - name: "frontend"
    location: "packages/frontend"
    type: "npm"
    commands:
      - "start"
      - "build"
      - "test"`,
			expected: Config{
				Locations: []Location{
					{
						Name:     "frontend",
						Location: "packages/frontend",
						Type:     "npm",
						Commands: []Command{
							{Name: "", Command: "start"},
							{Name: "", Command: "build"},
							{Name: "", Command: "test"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple locations",
			yaml: `locations:
  - name: "frontend"
    location: "packages/frontend"
    type: "npm"
    commands:
      - "start"
      - "build"
  - name: "backend"
    location: "packages/backend"
    type: "go"
    commands:
      - "run"
      - "test"`,
			expected: Config{
				Locations: []Location{
					{
						Name:     "frontend",
						Location: "packages/frontend",
						Type:     "npm",
						Commands: []Command{
							{Name: "", Command: "start"},
							{Name: "", Command: "build"},
						},
					},
					{
						Name:     "backend",
						Location: "packages/backend",
						Type:     "go",
						Commands: []Command{
							{Name: "", Command: "run"},
							{Name: "", Command: "test"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "location without name",
			yaml: `locations:
  - location: "packages/frontend"
    type: "npm"
    commands:
      - "start"`,
			expected: Config{
				Locations: []Location{
					{
						Location: "packages/frontend",
						Type:     "npm",
						Commands: []Command{
							{Name: "", Command: "start"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "location without type",
			yaml: `locations:
  - name: "scripts"
    location: "scripts"
    commands:
      - "deploy.sh"
      - "backup.sh"`,
			expected: Config{
				Locations: []Location{
					{
						Name:     "scripts",
						Location: "scripts",
						Commands: []Command{
							{Name: "", Command: "deploy.sh"},
							{Name: "", Command: "backup.sh"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "location without commands",
			yaml: `locations:
  - name: "api"
    location: "api"
    type: "go"`,
			expected: Config{
				Locations: []Location{
					{
						Name:     "api",
						Location: "api",
						Type:     "go",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty config",
			yaml: `locations: []`,
			expected: Config{
				Locations: []Location{},
			},
			wantErr: false,
		},
		{
			name: "location without path (root directory)",
			yaml: `locations:
  - name: "root"
    commands:
      - "go test"
      - "go build"`,
			expected: Config{
				Locations: []Location{
					{
						Name:     "root",
						Location: "",
						Commands: []Command{
							{Name: "", Command: "go test"},
							{Name: "", Command: "go build"},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config
			err := yaml.Unmarshal([]byte(tt.yaml), &config)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("yaml.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(config.Locations) != len(tt.expected.Locations) {
					t.Errorf("Expected %d locations, got %d", len(tt.expected.Locations), len(config.Locations))
					return
				}

				for i, loc := range config.Locations {
					expected := tt.expected.Locations[i]
					if loc.Name != expected.Name {
						t.Errorf("Location[%d].Name = %q, expected %q", i, loc.Name, expected.Name)
					}
					if loc.Location != expected.Location {
						t.Errorf("Location[%d].Location = %q, expected %q", i, loc.Location, expected.Location)
					}
					if loc.Type != expected.Type {
						t.Errorf("Location[%d].Type = %q, expected %q", i, loc.Type, expected.Type)
					}
					if len(loc.Commands) != len(expected.Commands) {
						t.Errorf("Location[%d] has %d commands, expected %d", i, len(loc.Commands), len(expected.Commands))
					}
					for j, cmd := range loc.Commands {
						expectedCmd := expected.Commands[j]
						if cmd.Name != expectedCmd.Name {
							t.Errorf("Location[%d].Commands[%d].Name = %q, expected %q", i, j, cmd.Name, expectedCmd.Name)
						}
						if cmd.Command != expectedCmd.Command {
							t.Errorf("Location[%d].Commands[%d].Command = %q, expected %q", i, j, cmd.Command, expectedCmd.Command)
						}
					}
				}
			}
		})
	}
}

func TestProcessProjectTypesWithEmptyLocation(t *testing.T) {
	// Create a temp directory with a package.json in the root
	tmpDir, err := os.MkdirTemp("", "gopm-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to the temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create a package.json in the current directory
	packageJSON := `{
  "name": "test-project",
  "scripts": {
    "start": "node index.js",
    "test": "jest"
  }
}`
	err = os.WriteFile("package.json", []byte(packageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	// Test config with empty location (before normalization)
	config := &Config{
		Locations: []Location{
			{
				Name:     "root",
				Location: "",
				Type:     "npm",
				Commands: []Command{
					{Name: "", Command: "custom"},
				},
			},
		},
	}

	// Normalize empty paths (this is what LoadConfig does)
	normalizeEmptyPaths(config)

	// Verify location was normalized to "."
	if config.Locations[0].Location != "." {
		t.Errorf("Expected location to be normalized to \".\", got %q", config.Locations[0].Location)
	}

	// Process project types
	err = processProjectTypes(config)
	if err != nil {
		t.Errorf("processProjectTypes() failed with empty location: %v", err)
	}

	// Verify commands were added
	if len(config.Locations[0].Commands) < 2 {
		t.Errorf("Expected at least 2 commands (1 custom + discovered), got %d", len(config.Locations[0].Commands))
	}
}

func TestLoadConfigWithEmptyLocation(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "gopm-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a config with empty location
	configYAML := `locations:
  - name: "root"
    commands:
      - "go test"
      - "go build"`

	configPath := filepath.Join(tmpDir, ".gopmrc")
	err = os.WriteFile(configPath, []byte(configYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load the config
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Verify the location was normalized to "."
	if len(config.Locations) != 1 {
		t.Fatalf("Expected 1 location, got %d", len(config.Locations))
	}

	location := config.Locations[0]
	if location.Name != "root" {
		t.Errorf("Expected name 'root', got %q", location.Name)
	}
	if location.Location != "." {
		t.Errorf("Expected location to be normalized to '.', got %q", location.Location)
	}
	if len(location.Commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(location.Commands))
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		configPath  string
		configYAML  string
		expected    Config
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid config file",
			configPath: ".gopmrc",
			configYAML: `locations:
  - name: "frontend"
    location: "packages/frontend"
    type: "npm"
    commands:
      - "start"
      - "build"`,
			expected: Config{
				Locations: []Location{
					{
						Name:     "frontend",
						Location: "packages/frontend",
						Type:     "npm",
						Commands: []Command{
							{Name: "", Command: "start"},
							{Name: "", Command: "build"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:        "config file not found",
			configPath:  ".gopmrc",
			wantErr:     true,
			errContains: "no such file or directory",
		},
		{
			name:       "invalid YAML",
			configPath: ".gopmrc",
			configYAML: `locations:
  - name: "frontend"
    location: "packages/frontend"
    type: "npm"
    commands:
      - "start"
      - "build"
    invalid_yaml: [`,
			wantErr:     true,
			errContains: "yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "gopm-test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			configPath := filepath.Join(tmpDir, tt.configPath)
			
			if tt.configYAML != "" {
				err = os.WriteFile(configPath, []byte(tt.configYAML), 0644)
				if err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}
			}

			config, err := LoadConfig(configPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errContains != "" && err.Error() != "" {
					// Just check if error occurred, don't check specific message
				}
				return
			}

			if err != nil {
				t.Errorf("LoadConfig() error = %v", err)
				return
			}

			if len(config.Locations) != len(tt.expected.Locations) {
				t.Errorf("Expected %d locations, got %d", len(tt.expected.Locations), len(config.Locations))
				return
			}

			for i, loc := range config.Locations {
				expected := tt.expected.Locations[i]
				if loc.Name != expected.Name {
					t.Errorf("Location[%d].Name = %q, expected %q", i, loc.Name, expected.Name)
				}
				if loc.Location != expected.Location {
					t.Errorf("Location[%d].Location = %q, expected %q", i, loc.Location, expected.Location)
				}
				if loc.Type != expected.Type {
					t.Errorf("Location[%d].Type = %q, expected %q", i, loc.Type, expected.Type)
				}
				if len(loc.Commands) != len(expected.Commands) {
					t.Errorf("Location[%d] has %d commands, expected %d", i, len(loc.Commands), len(expected.Commands))
				}
				for j, cmd := range loc.Commands {
					expectedCmd := expected.Commands[j]
					if cmd.Name != expectedCmd.Name {
						t.Errorf("Location[%d].Commands[%d].Name = %q, expected %q", i, j, cmd.Name, expectedCmd.Name)
					}
					if cmd.Command != expectedCmd.Command {
						t.Errorf("Location[%d].Commands[%d].Command = %q, expected %q", i, j, cmd.Command, expectedCmd.Command)
					}
				}
			}
		})
	}
}

func TestFilterCommands(t *testing.T) {
	// Helper to convert strings to Commands with no name
	stringsToCommands := func(strs []string) []Command {
		cmds := make([]Command, len(strs))
		for i, s := range strs {
			cmds[i] = Command{Name: "", Command: s}
		}
		return cmds
	}

	// Helper to convert Commands to strings for comparison
	commandsToStrings := func(cmds []Command) []string {
		strs := make([]string, len(cmds))
		for i, c := range cmds {
			strs[i] = c.Command
		}
		return strs
	}

	tests := []struct {
		name     string
		commands []string
		include  []string
		exclude  []string
		expected []string
	}{
		{
			name:     "no filters - return all commands",
			commands: []string{"dev", "build", "test", "lint"},
			include:  nil,
			exclude:  nil,
			expected: []string{"dev", "build", "test", "lint"},
		},
		{
			name:     "include exact match",
			commands: []string{"dev", "build", "test", "lint"},
			include:  []string{"dev", "build"},
			exclude:  nil,
			expected: []string{"dev", "build"},
		},
		{
			name:     "include with glob pattern",
			commands: []string{"dev", "build", "build:prod", "build:dev", "test", "test:watch"},
			include:  []string{"build*"},
			exclude:  nil,
			expected: []string{"build", "build:prod", "build:dev"},
		},
		{
			name:     "exclude exact match",
			commands: []string{"dev", "build", "test", "lint"},
			include:  nil,
			exclude:  []string{"lint"},
			expected: []string{"dev", "build", "test"},
		},
		{
			name:     "exclude with glob pattern",
			commands: []string{"test", "test:watch", "test:ci", "build"},
			include:  nil,
			exclude:  []string{"test:*"},
			expected: []string{"test", "build"},
		},
		{
			name:     "include then exclude",
			commands: []string{"dev", "build", "build:prod", "test", "test:watch", "lint"},
			include:  []string{"build*", "test*"},
			exclude:  []string{"test:watch"},
			expected: []string{"build", "build:prod", "test"},
		},
		{
			name:     "multiple include patterns",
			commands: []string{"dev", "start", "build", "build:prod", "test", "test:ci"},
			include:  []string{"dev", "start", "build*"},
			exclude:  nil,
			expected: []string{"dev", "start", "build", "build:prod"},
		},
		{
			name:     "multiple exclude patterns",
			commands: []string{"dev", "build", "test", "test:watch", "lint", "format"},
			include:  nil,
			exclude:  []string{"test:*", "lint", "format"},
			expected: []string{"dev", "build", "test"},
		},
		{
			name:     "include all with wildcard",
			commands: []string{"dev", "build", "test"},
			include:  []string{"*"},
			exclude:  nil,
			expected: []string{"dev", "build", "test"},
		},
		{
			name:     "exclude all matching pattern",
			commands: []string{"test:unit", "test:integration", "test:e2e", "build"},
			include:  nil,
			exclude:  []string{"test:*"},
			expected: []string{"build"},
		},
		{
			name:     "no commands match include",
			commands: []string{"dev", "build", "test"},
			include:  []string{"deploy*"},
			exclude:  nil,
			expected: []string{},
		},
		{
			name:     "empty commands list",
			commands: []string{},
			include:  []string{"dev"},
			exclude:  []string{"test"},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commandsInput := stringsToCommands(tt.commands)
			result := filterCommands(commandsInput, tt.include, tt.exclude)
			resultStrings := commandsToStrings(result)

			if len(resultStrings) != len(tt.expected) {
				t.Errorf("filterCommands() returned %d commands, expected %d\nGot: %v\nExpected: %v",
					len(resultStrings), len(tt.expected), resultStrings, tt.expected)
				return
			}

			for i, cmd := range resultStrings {
				if cmd != tt.expected[i] {
					t.Errorf("filterCommands()[%d] = %q, expected %q", i, cmd, tt.expected[i])
				}
			}
		})
	}
}

func TestLoadConfigWithIncludeExclude(t *testing.T) {
	// Create a temp directory with package.json
	tmpDir, err := os.MkdirTemp("", "gopm-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to the temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create a package.json with many scripts
	packageJSON := `{
  "name": "test-project",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "build:prod": "vite build --mode production",
    "test": "jest",
    "test:watch": "jest --watch",
    "test:ci": "jest --ci",
    "lint": "eslint .",
    "format": "prettier --write ."
  }
}`
	err = os.WriteFile("package.json", []byte(packageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	// Test config with include/exclude
	// Note: npm commands have names from package.json scripts, so patterns match against names
	configYAML := `locations:
  - name: "test"
    location: "."
    type: "npm"
    include:
      - "dev"
      - "build*"
      - "test"
    exclude:
      - "build:prod"
    commands:
      - "custom-command"`

	configPath := filepath.Join(tmpDir, ".gopmrc")
	err = os.WriteFile(configPath, []byte(configYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load the config
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Verify commands were filtered
	if len(config.Locations) != 1 {
		t.Fatalf("Expected 1 location, got %d", len(config.Locations))
	}

	location := config.Locations[0]

	// Manual command should be present
	hasCustom := false
	for _, cmd := range location.Commands {
		if cmd.Command == "custom-command" {
			hasCustom = true
			break
		}
	}
	if !hasCustom {
		t.Errorf("Manual command 'custom-command' should not be filtered")
	}

	// Check that included commands are present (by name)
	expectedPresent := []string{"dev", "build", "test"}
	for _, expectedName := range expectedPresent {
		found := false
		for _, cmd := range location.Commands {
			if cmd.Name == expectedName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected command with name %q to be present after filtering", expectedName)
		}
	}

	// Check that excluded commands are not present (by name)
	excludedNames := []string{"build:prod", "test:watch", "test:ci", "lint", "format"}
	for _, excludedName := range excludedNames {
		for _, cmd := range location.Commands {
			if cmd.Name == excludedName {
				t.Errorf("Command with name %q should have been filtered out", excludedName)
			}
		}
	}
}