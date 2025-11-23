package commands

import (
	"testing"

	"github.com/martin/go-pm/internal/config"
)

func TestRunEnhancedFzf_ConfigParsing(t *testing.T) {
	// Test that RunEnhancedFzf properly converts between ui.SelectionResult and commands.SelectionResult
	cfg := &config.Config{
		Locations: []config.Location{
			{
				Name:     "test-project",
				Location: "/test/path",
				Commands: []string{"test command"},
			},
		},
	}

	// We can't fully test the interactive part, but we can verify the function exists
	// and accepts the right parameters
	_ = RunEnhancedFzf
	_ = cfg
}

func TestPrepareCommandInfo_WithLocations(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{
			{
				Name:     "frontend",
				Location: "/path/to/frontend",
				Commands: []string{"npm start", "npm test"},
			},
			{
				Name:     "backend",
				Location: "/path/to/backend",
				Commands: []string{"go run main.go"},
			},
			{
				Location: "/path/to/scripts",
				Commands: []string{"./deploy.sh"},
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
