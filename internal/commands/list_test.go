package commands

import (
	"strings"
	"testing"

	"github.com/martinhrvn/paleta/internal/config"
)

func TestListCommands(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected []string
	}{
		{
			name: "single location with multiple commands",
			cfg: &config.Config{
				Locations: []config.Location{
					{
						Name:     "frontend",
						Location: "packages/frontend",
						Commands: stringsToCommands([]string{"start", "build", "test"}),
					},
				},
			},
			expected: []string{
				"frontend:start",
				"frontend:build", 
				"frontend:test",
			},
		},
		{
			name: "multiple locations with commands",
			cfg: &config.Config{
				Locations: []config.Location{
					{
						Name:     "frontend",
						Location: "packages/frontend",
						Commands: stringsToCommands([]string{"start", "build"}),
					},
					{
						Name:     "backend",
						Location: "packages/backend",
						Commands: stringsToCommands([]string{"run", "test"}),
					},
				},
			},
			expected: []string{
				"frontend:start",
				"frontend:build",
				"backend:run",
				"backend:test",
			},
		},
		{
			name: "location without name uses location path",
			cfg: &config.Config{
				Locations: []config.Location{
					{
						Location: "packages/frontend",
						Commands: stringsToCommands([]string{"start", "build"}),
					},
					{
						Name:     "backend",
						Location: "packages/backend",
						Commands: stringsToCommands([]string{"run"}),
					},
				},
			},
			expected: []string{
				"packages/frontend:start",
				"packages/frontend:build",
				"backend:run",
			},
		},
		{
			name: "location without commands",
			cfg: &config.Config{
				Locations: []config.Location{
					{
						Name:     "frontend",
						Location: "packages/frontend",
					},
					{
						Name:     "backend",
						Location: "packages/backend",
						Commands: stringsToCommands([]string{"run"}),
					},
				},
			},
			expected: []string{
				"backend:run",
			},
		},
		{
			name: "empty config",
			cfg: &config.Config{
				Locations: []config.Location{},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ListCommands(tt.cfg)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d commands, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Command[%d] = %q, expected %q", i, result[i], expected)
				}
			}
		})
	}
}

func TestFormatForFzf(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected []string
	}{
		{
			name: "single location with commands",
			cfg: &config.Config{
				Locations: []config.Location{
					{
						Name:     "frontend",
						Location: "packages/frontend",
						Commands: stringsToCommands([]string{"start", "build"}),
					},
				},
			},
			expected: []string{
				"[frontend] start",
				"[frontend] build",
			},
		},
		{
			name: "multiple locations",
			cfg: &config.Config{
				Locations: []config.Location{
					{
						Name:     "frontend",
						Location: "packages/frontend",
						Commands: stringsToCommands([]string{"start"}),
					},
					{
						Location: "packages/backend",
						Commands: stringsToCommands([]string{"run"}),
					},
				},
			},
			expected: []string{
				"[frontend] start",
				"[packages/backend] run",
			},
		},
		{
			name: "empty config",
			cfg: &config.Config{
				Locations: []config.Location{},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatForFzf(tt.cfg)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d commands, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Command[%d] = %q, expected %q", i, result[i], expected)
				}
			}
		})
	}
}

func TestListCommandsOutput(t *testing.T) {
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
		},
	}

	tests := []struct {
		name           string
		format         string
		expectedOutput string
	}{
		{
			name:   "default format",
			format: "default",
			expectedOutput: `frontend:start
frontend:build
packages/backend:run
packages/backend:test`,
		},
		{
			name:   "fzf format",
			format: "fzf",
			expectedOutput: `[frontend] start
[frontend] build
[packages/backend] run
[packages/backend] test`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			
			if tt.format == "fzf" {
				commands := FormatForFzf(cfg)
				result = strings.Join(commands, "\n")
			} else {
				commands := ListCommands(cfg)
				result = strings.Join(commands, "\n")
			}

			if result != tt.expectedOutput {
				t.Errorf("Expected output:\n%s\n\nGot:\n%s", tt.expectedOutput, result)
			}
		})
	}
}