package ui

import (
	"testing"

	"github.com/martin/go-pm/internal/config"
)

func TestEnhancedSelector_prepareFilteredCommands(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Locations: []config.Location{
			{
				Name:     "frontend",
				Location: "/path/to/frontend",
				Type:     "npm",
				Commands: []string{"npm start", "npm test", "npm build"},
			},
			{
				Name:     "backend",
				Location: "/path/to/backend",
				Type:     "go",
				Commands: []string{"go run main.go", "go test ./..."},
			},
			{
				Location: "/path/to/unnamed",
				Commands: []string{"make", "make test"},
			},
		},
	}

	tests := []struct {
		name              string
		selectedLocations []string
		expectedCount     int
		expectedCommands  []string
	}{
		{
			name:              "all locations when none selected",
			selectedLocations: []string{},
			expectedCount:     7, // 3 + 2 + 2
			expectedCommands: []string{
				"frontend: npm start",
				"frontend: npm test",
				"frontend: npm build",
				"backend: go run main.go",
				"backend: go test ./...",
				"/path/to/unnamed: make",
				"/path/to/unnamed: make test",
			},
		},
		{
			name:              "only frontend location",
			selectedLocations: []string{"frontend"},
			expectedCount:     3,
			expectedCommands: []string{
				"frontend: npm start",
				"frontend: npm test",
				"frontend: npm build",
			},
		},
		{
			name:              "frontend and backend locations",
			selectedLocations: []string{"frontend", "backend"},
			expectedCount:     5,
			expectedCommands: []string{
				"frontend: npm start",
				"frontend: npm test",
				"frontend: npm build",
				"backend: go run main.go",
				"backend: go test ./...",
			},
		},
		{
			name:              "unnamed location by path",
			selectedLocations: []string{"/path/to/unnamed"},
			expectedCount:     2,
			expectedCommands: []string{
				"/path/to/unnamed: make",
				"/path/to/unnamed: make test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := &EnhancedSelector{
				config:            cfg,
				selectedLocations: tt.selectedLocations,
			}

			commands := selector.prepareFilteredCommands()

			if len(commands) != tt.expectedCount {
				t.Errorf("expected %d commands, got %d", tt.expectedCount, len(commands))
			}

			// Check command display strings
			for i, cmd := range commands {
				if i < len(tt.expectedCommands) && cmd.Display != tt.expectedCommands[i] {
					t.Errorf("command %d: expected %q, got %q", i, tt.expectedCommands[i], cmd.Display)
				}
			}
		})
	}
}

// TestEnhancedSelector_CommandInfoMetadata tests that CommandInfo contains all necessary metadata
func TestEnhancedSelector_CommandInfoMetadata(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{
			{
				Name:     "frontend",
				Location: "/path/to/frontend",
				Type:     "npm",
				Commands: []string{"npm start", "npm build"},
			},
			{
				Location: "/path/to/backend",
				Type:     "go",
				Commands: []string{"go test ./..."},
			},
		},
	}

	selector := &EnhancedSelector{
		config:            cfg,
		selectedLocations: []string{},
	}

	commands := selector.prepareFilteredCommands()

	// Test first command metadata
	if commands[0].DisplayName != "frontend" {
		t.Errorf("expected DisplayName 'frontend', got %q", commands[0].DisplayName)
	}
	if commands[0].Directory != "/path/to/frontend" {
		t.Errorf("expected Directory '/path/to/frontend', got %q", commands[0].Directory)
	}
	if commands[0].Command != "npm start" {
		t.Errorf("expected Command 'npm start', got %q", commands[0].Command)
	}
	if commands[0].Type != "npm" {
		t.Errorf("expected Type 'npm', got %q", commands[0].Type)
	}

	// Test backend command (unnamed location)
	backendCmd := commands[2] // Third command should be backend
	if backendCmd.DisplayName != "/path/to/backend" {
		t.Errorf("expected DisplayName '/path/to/backend', got %q", backendCmd.DisplayName)
	}
	if backendCmd.Type != "go" {
		t.Errorf("expected Type 'go', got %q", backendCmd.Type)
	}
}

func TestEnhancedSelector_isLocationSelected(t *testing.T) {
	selector := &EnhancedSelector{
		selectedLocations: []string{"frontend", "backend"},
	}

	tests := []struct {
		name     string
		location config.Location
		expected bool
	}{
		{
			name: "named location is selected",
			location: config.Location{
				Name:     "frontend",
				Location: "/path/to/frontend",
			},
			expected: true,
		},
		{
			name: "named location not selected",
			location: config.Location{
				Name:     "database",
				Location: "/path/to/db",
			},
			expected: false,
		},
		{
			name: "unnamed location by path",
			location: config.Location{
				Location: "backend",
			},
			expected: true, // because "backend" is in selectedLocations
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selector.isLocationSelected(tt.location)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEnhancedSelector_getLocationString(t *testing.T) {
	tests := []struct {
		name              string
		selectedLocations []string
		expected          string
	}{
		{
			name:              "all locations",
			selectedLocations: []string{},
			expected:          "All",
		},
		{
			name:              "single location",
			selectedLocations: []string{"frontend"},
			expected:          "frontend",
		},
		{
			name:              "multiple locations",
			selectedLocations: []string{"frontend", "backend", "database"},
			expected:          "frontend, backend, database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := &EnhancedSelector{
				selectedLocations: tt.selectedLocations,
			}
			result := selector.getLocationString()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestEnhancedSelector_isLocationInList(t *testing.T) {
	selector := &EnhancedSelector{}

	tests := []struct {
		name     string
		location string
		list     []string
		expected bool
	}{
		{
			name:     "location in list",
			location: "frontend",
			list:     []string{"frontend", "backend"},
			expected: true,
		},
		{
			name:     "location not in list",
			location: "database",
			list:     []string{"frontend", "backend"},
			expected: false,
		},
		{
			name:     "empty list",
			location: "frontend",
			list:     []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selector.isLocationInList(tt.location, tt.list)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
