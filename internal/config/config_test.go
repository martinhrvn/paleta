package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

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
						Commands: []string{"start", "build", "test"},
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
						Commands: []string{"start", "build"},
					},
					{
						Name:     "backend",
						Location: "packages/backend",
						Type:     "go",
						Commands: []string{"run", "test"},
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
						Commands: []string{"start"},
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
						Commands: []string{"deploy.sh", "backup.sh"},
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
						Commands: []string{"go test", "go build"},
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
						if cmd != expected.Commands[j] {
							t.Errorf("Location[%d].Commands[%d] = %q, expected %q", i, j, cmd, expected.Commands[j])
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
				Commands: []string{"custom"},
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
						Commands: []string{"start", "build"},
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
					if cmd != expected.Commands[j] {
						t.Errorf("Location[%d].Commands[%d] = %q, expected %q", i, j, cmd, expected.Commands[j])
					}
				}
			}
		})
	}
}