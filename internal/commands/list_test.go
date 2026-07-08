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

func multiTypeConfig() *config.Config {
	return &config.Config{
		Locations: []config.Location{
			{
				Name:  "dotfiles",
				Types: config.Types{"npm", "compose"},
				Commands: []config.Command{
					{Name: "build", Command: "npm run build", Type: "npm"},
					{Name: "up", Command: "docker compose up", Type: "compose"},
					{Name: "deploy", Command: "./deploy.sh"}, // manual, no type
				},
			},
		},
	}
}

func TestListCommands_MultiType(t *testing.T) {
	got := ListCommands(multiTypeConfig())
	want := []string{"dotfiles:[npm] build", "dotfiles:[compose] up", "dotfiles:deploy"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("ListCommands multi-type =\n%v\nwant\n%v", got, want)
	}
}

func TestFormatForFzf_MultiType(t *testing.T) {
	got := FormatForFzf(multiTypeConfig())
	want := []string{"[dotfiles] [npm] build", "[dotfiles] [compose] up", "[dotfiles] deploy"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("FormatForFzf multi-type =\n%v\nwant\n%v", got, want)
	}
}

// Resolved tools render after the location commands, at the end of the list.
func TestListCommands_ToolsAppendedLast(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{
			{Name: "root", Location: ".", Commands: stringsToCommands([]string{"build"})},
		},
		ResolvedTools: []config.ResolvedTool{
			{Tool: "lazygit", Display: "lazygit", Command: "lazygit", Directory: "/w"},
			{Tool: "docker", Display: "docker: up", Command: "docker compose up", Directory: "/w"},
		},
	}

	got := ListCommands(cfg)
	want := []string{"root:build", "lazygit", "docker: up"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("ListCommands with tools =\n%v\nwant\n%v", got, want)
	}
}

// In fzf format, tools are grouped by tool name after the locations.
func TestFormatForFzf_ToolsAppendedLast(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{
			{Name: "root", Location: ".", Commands: stringsToCommands([]string{"build"})},
		},
		ResolvedTools: []config.ResolvedTool{
			{Tool: "lazygit", Display: "lazygit", Command: "lazygit"},
			{Tool: "docker", Display: "docker: up", Command: "docker compose up"},
		},
	}

	got := FormatForFzf(cfg)
	want := []string{"[root] build", "[lazygit] lazygit", "[docker] up"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("FormatForFzf with tools =\n%v\nwant\n%v", got, want)
	}
}

// A single-type location must render exactly as before (no [type] prefix).
func TestFormatForFzf_SingleTypeUnchanged(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{
			{
				Name:  "frontend",
				Types: config.Types{"npm"},
				Commands: []config.Command{
					{Name: "build", Command: "npm run build", Type: "npm"},
				},
			},
		},
	}
	got := FormatForFzf(cfg)
	want := []string{"[frontend] build"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("FormatForFzf single-type = %v, want %v", got, want)
	}
}
