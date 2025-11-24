package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindConfigFile(t *testing.T) {
	tests := []struct {
		name           string
		setupDirs      []string
		setupConfigs   map[string]string
		startDir       string
		expectedConfig string
		wantErr        bool
	}{
		{
			name:      "config in current directory",
			setupDirs: []string{"project"},
			setupConfigs: map[string]string{
				"project/.gopmrc": `locations:
  - location: "src"
    commands: ["build"]`,
			},
			startDir:       "project",
			expectedConfig: "project/.gopmrc",
			wantErr:        false,
		},
		{
			name:      "config in parent directory",
			setupDirs: []string{"project", "project/subdir"},
			setupConfigs: map[string]string{
				"project/.gopmrc": `locations:
  - location: "src"
    commands: ["build"]`,
			},
			startDir:       "project/subdir",
			expectedConfig: "project/.gopmrc",
			wantErr:        false,
		},
		{
			name:      "config in grandparent directory",
			setupDirs: []string{"project", "project/subdir", "project/subdir/nested"},
			setupConfigs: map[string]string{
				"project/.gopmrc": `locations:
  - location: "src"
    commands: ["build"]`,
			},
			startDir:       "project/subdir/nested",
			expectedConfig: "project/.gopmrc",
			wantErr:        false,
		},
		{
			name:      "config in closer directory takes precedence",
			setupDirs: []string{"project", "project/subdir"},
			setupConfigs: map[string]string{
				"project/.gopmrc": `locations:
  - location: "root"
    commands: ["build"]`,
				"project/subdir/.gopmrc": `locations:
  - location: "subdir"
    commands: ["test"]`,
			},
			startDir:       "project/subdir",
			expectedConfig: "project/subdir/.gopmrc",
			wantErr:        false,
		},
		{
			name:      "no config file found",
			setupDirs: []string{"project", "project/subdir"},
			startDir:  "project/subdir",
			wantErr:   true,
		},
		{
			name:      "config in root directory",
			setupDirs: []string{"project", "project/subdir", "project/subdir/nested", "project/subdir/nested/deep"},
			setupConfigs: map[string]string{
				"project/.gopmrc": `locations:
  - location: "root"
    commands: ["build"]`,
			},
			startDir:       "project/subdir/nested/deep",
			expectedConfig: "project/.gopmrc",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "gopm-discovery-test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create directory structure
			for _, dir := range tt.setupDirs {
				err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
				if err != nil {
					t.Fatalf("Failed to create dir %s: %v", dir, err)
				}
			}

			// Create config files
			for configPath, content := range tt.setupConfigs {
				fullPath := filepath.Join(tmpDir, configPath)
				err := os.WriteFile(fullPath, []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to create config file %s: %v", configPath, err)
				}
			}

			// Change to start directory
			startPath := filepath.Join(tmpDir, tt.startDir)
			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get working directory: %v", err)
			}
			defer os.Chdir(oldWd)

			err = os.Chdir(startPath)
			if err != nil {
				t.Fatalf("Failed to change to start directory: %v", err)
			}

			// Test FindConfigFile
			configPath, err := FindConfigFile()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("FindConfigFile() error = %v", err)
				return
			}

			// Convert expected path to absolute for comparison
			expectedPath := filepath.Join(tmpDir, tt.expectedConfig)
			if configPath != expectedPath {
				t.Errorf("FindConfigFile() = %q, expected %q", configPath, expectedPath)
			}
		})
	}
}

func TestLoadConfigFromDiscovery(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gopm-discovery-integration-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directory structure
	dirs := []string{"project", "project/subdir", "project/subdir/nested"}
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		if err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	// Create config in parent directory
	configContent := `locations:
  - name: "frontend"
    location: "packages/frontend"
    type: "npm"
    commands: ["start", "build"]`

	configPath := filepath.Join(tmpDir, "project/.gopmrc")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Create test directories for glob expansion
	testDirs := []string{"project/packages/frontend", "project/packages/backend"}
	for _, dir := range testDirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		if err != nil {
			t.Fatalf("Failed to create test dir %s: %v", dir, err)
		}
	}

	// Change to nested directory
	nestedPath := filepath.Join(tmpDir, "project/subdir/nested")
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	err = os.Chdir(nestedPath)
	if err != nil {
		t.Fatalf("Failed to change to nested directory: %v", err)
	}

	// Test LoadConfigFromDiscovery
	config, err := LoadConfigFromDiscovery()
	if err != nil {
		t.Fatalf("LoadConfigFromDiscovery() error = %v", err)
	}

	// Verify config was loaded correctly
	if len(config.Locations) != 1 {
		t.Errorf("Expected 1 location, got %d", len(config.Locations))
	}

	if config.Locations[0].Name != "frontend" {
		t.Errorf("Expected location name 'frontend', got %q", config.Locations[0].Name)
	}

	if config.Locations[0].Location != "packages/frontend" {
		t.Errorf("Expected location path 'packages/frontend', got %q", config.Locations[0].Location)
	}
}

func TestLoadGlobalProjects(t *testing.T) {
	tests := []struct {
		name           string
		projectFiles   map[string]string // filename -> content
		expectedCount  int
		validateFunc   func(*testing.T, []*Config)
		wantErr        bool
	}{
		{
			name: "load multiple project files",
			projectFiles: map[string]string{
				"project1.yaml": `root: /home/user/projects/project1
locations:
  - location: "src"
    commands: ["build"]`,
				"project2.yaml": `root: /home/user/projects/project2
locations:
  - location: "app"
    commands: ["test"]`,
			},
			expectedCount: 2,
			validateFunc: func(t *testing.T, configs []*Config) {
				if configs[0].Root != "/home/user/projects/project1" && configs[1].Root != "/home/user/projects/project1" {
					t.Error("Expected to find project1 root")
				}
				if configs[0].Root != "/home/user/projects/project2" && configs[1].Root != "/home/user/projects/project2" {
					t.Error("Expected to find project2 root")
				}
			},
			wantErr: false,
		},
		{
			name:          "empty projects directory",
			projectFiles:  map[string]string{},
			expectedCount: 0,
			wantErr:       false,
		},
		{
			name: "single project file",
			projectFiles: map[string]string{
				"myproject.yaml": `root: /home/user/myproject
locations:
  - location: "."
    commands: ["npm test"]`,
			},
			expectedCount: 1,
			validateFunc: func(t *testing.T, configs []*Config) {
				if configs[0].Root != "/home/user/myproject" {
					t.Errorf("Expected root '/home/user/myproject', got %q", configs[0].Root)
				}
				if len(configs[0].Locations) != 1 {
					t.Errorf("Expected 1 location, got %d", len(configs[0].Locations))
				}
			},
			wantErr: false,
		},
		{
			name: "ignore non-yaml files",
			projectFiles: map[string]string{
				"project1.yaml": `root: /home/user/project1
locations:
  - location: "src"`,
				"readme.txt": "This is a readme",
				"project2.yml": `root: /home/user/project2
locations:
  - location: "app"`,
			},
			expectedCount: 2, // Only .yaml and .yml files
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary projects directory
			tmpDir, err := os.MkdirTemp("", "gopm-global-projects-test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			projectsDir := filepath.Join(tmpDir, "projects")
			err = os.MkdirAll(projectsDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create projects dir: %v", err)
			}

			// Create project files
			for filename, content := range tt.projectFiles {
				filePath := filepath.Join(projectsDir, filename)
				err := os.WriteFile(filePath, []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to create project file %s: %v", filename, err)
				}
			}

			// Test LoadGlobalProjects
			configs, err := loadGlobalProjectsFromDir(projectsDir)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("loadGlobalProjectsFromDir() error = %v", err)
				return
			}

			if len(configs) != tt.expectedCount {
				t.Errorf("Expected %d configs, got %d", tt.expectedCount, len(configs))
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, configs)
			}
		})
	}
}

func TestFindMatchingProject(t *testing.T) {
	tests := []struct {
		name      string
		cwd       string
		projects  []*Config
		wantMatch bool
		wantRoot  string
	}{
		{
			name: "exact match",
			cwd:  "/home/user/project1",
			projects: []*Config{
				{Root: "/home/user/project1", Locations: []Location{{Location: "src"}}},
			},
			wantMatch: true,
			wantRoot:  "/home/user/project1",
		},
		{
			name: "parent match",
			cwd:  "/home/user/project1/subdir/nested",
			projects: []*Config{
				{Root: "/home/user/project1", Locations: []Location{{Location: "src"}}},
			},
			wantMatch: true,
			wantRoot:  "/home/user/project1",
		},
		{
			name: "no match",
			cwd:  "/home/user/project2",
			projects: []*Config{
				{Root: "/home/user/project1", Locations: []Location{{Location: "src"}}},
			},
			wantMatch: false,
		},
		{
			name: "multiple projects - find correct one",
			cwd:  "/home/user/project2/src",
			projects: []*Config{
				{Root: "/home/user/project1", Locations: []Location{{Location: "src"}}},
				{Root: "/home/user/project2", Locations: []Location{{Location: "app"}}},
				{Root: "/home/user/project3", Locations: []Location{{Location: "lib"}}},
			},
			wantMatch: true,
			wantRoot:  "/home/user/project2",
		},
		{
			name: "prefer closest match",
			cwd:  "/home/user/workspace/project/subdir",
			projects: []*Config{
				{Root: "/home/user/workspace", Locations: []Location{{Location: "."}}},
				{Root: "/home/user/workspace/project", Locations: []Location{{Location: "src"}}},
			},
			wantMatch: true,
			wantRoot:  "/home/user/workspace/project", // Closest match
		},
		{
			name:      "empty projects list",
			cwd:       "/home/user/project",
			projects:  []*Config{},
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := FindMatchingProject(tt.cwd, tt.projects)

			if tt.wantMatch {
				if config == nil {
					t.Error("Expected to find matching project but got nil")
					return
				}
				if config.Root != tt.wantRoot {
					t.Errorf("Expected root %q, got %q", tt.wantRoot, config.Root)
				}
			} else {
				if config != nil {
					t.Errorf("Expected no match but got project with root %q", config.Root)
				}
			}
		})
	}
}

func TestLoadConfigFromDiscoveryWithGlobalFallback(t *testing.T) {
	tests := []struct {
		name               string
		setupLocalConfig   bool
		localConfigContent string
		globalProjects     map[string]string
		startDir           string
		projectRoot        string
		expectLocal        bool
		expectGlobal       bool
		wantErr            bool
	}{
		{
			name:             "local .gopmrc takes precedence",
			setupLocalConfig: true,
			localConfigContent: `locations:
  - location: "local-src"
    commands: ["local-build"]`,
			globalProjects: map[string]string{
				"project1.yaml": `root: /tmp/test-project
locations:
  - location: "global-src"
    commands: ["global-build"]`,
			},
			startDir:     "test-project",
			projectRoot:  "/tmp/test-project",
			expectLocal:  true,
			expectGlobal: false,
			wantErr:      false,
		},
		{
			name:             "fallback to global when no local config",
			setupLocalConfig: false,
			globalProjects: map[string]string{
				"project1.yaml": `root: /tmp/test-project
locations:
  - location: "global-src"
    commands: ["global-build"]`,
			},
			startDir:     "test-project/subdir",
			projectRoot:  "/tmp/test-project",
			expectLocal:  false,
			expectGlobal: true,
			wantErr:      false,
		},
		{
			name:             "no config found - neither local nor global",
			setupLocalConfig: false,
			globalProjects:   map[string]string{},
			startDir:         "test-project",
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory structure
			tmpDir, err := os.MkdirTemp("", "gopm-discovery-fallback-test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create project directory
			projectDir := filepath.Join(tmpDir, "test-project")
			err = os.MkdirAll(filepath.Join(projectDir, "subdir"), 0755)
			if err != nil {
				t.Fatalf("Failed to create project dir: %v", err)
			}

			// Setup local config if needed
			if tt.setupLocalConfig {
				configPath := filepath.Join(projectDir, ".gopmrc")
				err = os.WriteFile(configPath, []byte(tt.localConfigContent), 0644)
				if err != nil {
					t.Fatalf("Failed to create local config: %v", err)
				}
			}

			// Setup global projects directory
			globalProjectsDir := filepath.Join(tmpDir, "global-projects")
			err = os.MkdirAll(globalProjectsDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create global projects dir: %v", err)
			}

			// Create global project files (with adjusted root paths to use tmpDir)
			for filename, content := range tt.globalProjects {
				// Replace /tmp with tmpDir in the content
				adjustedContent := content
				if tt.projectRoot != "" {
					adjustedContent = `root: ` + projectDir + `
locations:
  - location: "global-src"
    commands: ["global-build"]`
				}
				filePath := filepath.Join(globalProjectsDir, filename)
				err := os.WriteFile(filePath, []byte(adjustedContent), 0644)
				if err != nil {
					t.Fatalf("Failed to create global project file: %v", err)
				}
			}

			// Change to start directory
			startPath := filepath.Join(tmpDir, tt.startDir)
			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get working directory: %v", err)
			}
			defer os.Chdir(oldWd)

			err = os.Chdir(startPath)
			if err != nil {
				t.Fatalf("Failed to change to start directory: %v", err)
			}

			// Test loadConfigFromDiscoveryWithGlobalFallback
			config, err := loadConfigFromDiscoveryWithGlobalFallback(globalProjectsDir)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("loadConfigFromDiscoveryWithGlobalFallback() error = %v", err)
				return
			}

			// Verify correct config was loaded
			if tt.expectLocal {
				if len(config.Locations) == 0 || config.Locations[0].Location != "local-src" {
					t.Error("Expected local config to be loaded")
				}
			}

			if tt.expectGlobal {
				if len(config.Locations) == 0 || config.Locations[0].Location != "global-src" {
					t.Error("Expected global config to be loaded")
				}
			}
		})
	}
}