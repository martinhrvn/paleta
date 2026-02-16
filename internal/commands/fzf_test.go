package commands

import (
	"testing"

	"github.com/martin/go-pm/internal/config"
)

// Helper to convert strings to Commands for tests
func stringsToCommands(strs []string) []config.Command {
	cmds := make([]config.Command, len(strs))
	for i, s := range strs {
		cmds[i] = config.Command{Name: "", Command: s}
	}
	return cmds
}

func TestParseFzfSelection(t *testing.T) {
	tests := []struct {
		name        string
		selection   string
		expectedCmd string
		expectedLoc string
		wantErr     bool
	}{
		{
			name:        "valid selection with name",
			selection:   "frontend: start",
			expectedCmd: "start",
			expectedLoc: "frontend",
			wantErr:     false,
		},
		{
			name:        "valid selection with path",
			selection:   "packages/backend: run",
			expectedCmd: "run",
			expectedLoc: "packages/backend",
			wantErr:     false,
		},
		{
			name:        "selection with spaces in command",
			selection:   "scripts: npm run build:prod",
			expectedCmd: "npm run build:prod",
			expectedLoc: "scripts",
			wantErr:     false,
		},
		{
			name:      "invalid format - no colon",
			selection: "frontend start",
			wantErr:   true,
		},
		{
			name:      "invalid format - no command",
			selection: "frontend:",
			wantErr:   true,
		},
		{
			name:      "empty selection",
			selection: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, loc, err := ParseFzfSelection(tt.selection)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFzfSelection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if cmd != tt.expectedCmd {
					t.Errorf("ParseFzfSelection() command = %q, expected %q", cmd, tt.expectedCmd)
				}
				if loc != tt.expectedLoc {
					t.Errorf("ParseFzfSelection() location = %q, expected %q", loc, tt.expectedLoc)
				}
			}
		})
	}
}

func TestFindLocationByDisplayName(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{
			{
				Name:     "frontend",
				Location: "packages/frontend",
				Commands: stringsToCommands([]string{"start", "build"}),
			},
			{
				Location: "packages/backend",
				Commands: stringsToCommands([]string{"run", "test"}),
			},
			{
				Name:     "scripts",
				Location: "scripts",
				Commands: stringsToCommands([]string{"deploy.sh"}),
			},
			{
				Location: ".",
				Commands: stringsToCommands([]string{"go test", "go build"}),
			},
		},
	}

	tests := []struct {
		name         string
		displayName  string
		expectedPath string
		wantErr      bool
	}{
		{
			name:         "find by name",
			displayName:  "frontend",
			expectedPath: "packages/frontend",
			wantErr:      false,
		},
		{
			name:         "find by location path",
			displayName:  "packages/backend",
			expectedPath: "packages/backend",
			wantErr:      false,
		},
		{
			name:         "find by name when location differs",
			displayName:  "scripts",
			expectedPath: "scripts",
			wantErr:      false,
		},
		{
			name:         "find root location by (root) display name",
			displayName:  "(root)",
			expectedPath: ".",
			wantErr:      false,
		},
		{
			name:        "location not found",
			displayName: "nonexistent",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			location, err := FindLocationByDisplayName(cfg, tt.displayName)

			if (err != nil) != tt.wantErr {
				t.Errorf("FindLocationByDisplayName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if location.Location != tt.expectedPath {
					t.Errorf("FindLocationByDisplayName() path = %q, expected %q", location.Location, tt.expectedPath)
				}
			}
		})
	}
}

func TestPrepareCommandInfo_WithLocations(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{
			{
				Name:     "frontend",
				Location: "/path/to/frontend",
				Commands: stringsToCommands([]string{"npm start", "npm test"}),
			},
			{
				Name:     "backend",
				Location: "/path/to/backend",
				Commands: stringsToCommands([]string{"go run main.go"}),
			},
			{
				Location: "/path/to/scripts",
				Commands: stringsToCommands([]string{"./deploy.sh"}),
			},
		},
	}

	infos := PrepareCommandInfo(cfg)

	if len(infos) != 4 {
		t.Errorf("Expected 4 command infos, got %d", len(infos))
	}

	// Test that display names are formatted correctly
	expectedDisplays := []string{
		"frontend: npm start",
		"frontend: npm test",
		"backend: go run main.go",
		"/path/to/scripts: ./deploy.sh",
	}

	for i, expected := range expectedDisplays {
		if i < len(infos) && infos[i].Display != expected {
			t.Errorf("Expected display %q, got %q", expected, infos[i].Display)
		}
	}
}

func TestSelectionResult(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{
			{
				Name:     "frontend",
				Location: "packages/frontend",
				Commands: stringsToCommands([]string{"start", "build"}),
			},
			{
				Location: "packages/backend",
				Commands: stringsToCommands([]string{"run"}),
			},
		},
	}

	tests := []struct {
		name             string
		fzfSelection     string
		expectedDir      string
		expectedCommand  string
		expectedDisplay  string
		wantErr          bool
	}{
		{
			name:             "valid selection with name",
			fzfSelection:     "frontend: start",
			expectedDir:      "packages/frontend",
			expectedCommand:  "start",
			expectedDisplay:  "frontend",
			wantErr:          false,
		},
		{
			name:             "valid selection with path as display",
			fzfSelection:     "packages/backend: run",
			expectedDir:      "packages/backend",
			expectedCommand:  "run",
			expectedDisplay:  "packages/backend",
			wantErr:          false,
		},
		{
			name:         "invalid selection format",
			fzfSelection: "invalid format",
			wantErr:      true,
		},
		{
			name:         "location not found in config",
			fzfSelection: "nonexistent: command",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ProcessFzfSelection(cfg, tt.fzfSelection)

			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessFzfSelection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result.Directory != tt.expectedDir {
					t.Errorf("ProcessFzfSelection() directory = %q, expected %q", result.Directory, tt.expectedDir)
				}
				if result.Command != tt.expectedCommand {
					t.Errorf("ProcessFzfSelection() command = %q, expected %q", result.Command, tt.expectedCommand)
				}
				if result.DisplayName != tt.expectedDisplay {
					t.Errorf("ProcessFzfSelection() displayName = %q, expected %q", result.DisplayName, tt.expectedDisplay)
				}
			}
		})
	}
}