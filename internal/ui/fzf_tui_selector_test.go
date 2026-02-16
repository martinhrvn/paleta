package ui

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

// Helper function to create test config
func createTestConfig() *config.Config {
	return &config.Config{
		Locations: []config.Location{
			{
				Name:     "frontend",
				Location: "/path/to/frontend",
				Type:     "npm",
				Commands: stringsToCommands([]string{"npm start", "npm test", "npm build"}),
			},
			{
				Name:     "backend",
				Location: "/path/to/backend",
				Type:     "go",
				Commands: stringsToCommands([]string{"go run main.go", "go test ./..."}),
			},
		},
	}
}

// Helper to create FzfTUISelector with test config (without starting app)
func createTestFzfSelector(cfg *config.Config) *FzfTUISelector {
	return NewFzfTUISelector(cfg)
}

func TestFzfTUISelector_ToggleSelection(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	// Load commands to populate filteredCommands
	selector.loadCommands()
	selector.filteredCommands = selector.commands // No filtering for test

	// Initially no selections
	if len(selector.selectedIndices) != 0 {
		t.Errorf("expected 0 selections initially, got %d", len(selector.selectedIndices))
	}

	// Toggle selection on
	selector.toggleSelection(0)
	if !selector.selectedIndices[0] {
		t.Error("expected index 0 to be selected after toggle")
	}

	// Toggle selection off
	selector.toggleSelection(0)
	if selector.selectedIndices[0] {
		t.Error("expected index 0 to be unselected after second toggle")
	}
}

func TestFzfTUISelector_ToggleMultipleSelections(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()
	selector.filteredCommands = selector.commands

	// Select multiple items
	selector.toggleSelection(0)
	selector.toggleSelection(2)
	selector.toggleSelection(4)

	if !selector.selectedIndices[0] {
		t.Error("expected index 0 to be selected")
	}
	if !selector.selectedIndices[2] {
		t.Error("expected index 2 to be selected")
	}
	if !selector.selectedIndices[4] {
		t.Error("expected index 4 to be selected")
	}

	// Index 1 and 3 should not be selected
	if selector.selectedIndices[1] {
		t.Error("expected index 1 to NOT be selected")
	}
	if selector.selectedIndices[3] {
		t.Error("expected index 3 to NOT be selected")
	}
}

func TestFzfTUISelector_ToggleOutOfBounds(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()
	selector.filteredCommands = selector.commands

	// Should not panic on out of bounds
	selector.toggleSelection(-1)
	selector.toggleSelection(100)

	// Should have no selections
	if len(selector.selectedIndices) != 0 {
		t.Errorf("expected 0 selections for out of bounds, got %d", len(selector.selectedIndices))
	}
}

func TestFzfTUISelector_GetSelectedCommands_WithSelections(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()
	selector.filteredCommands = selector.commands

	// Select items at indices 0 and 2
	selector.toggleSelection(0)
	selector.toggleSelection(2)

	results := selector.getSelectedCommands()

	if len(results) != 2 {
		t.Errorf("expected 2 selected commands, got %d", len(results))
	}

	// Verify order (should be in appearance order in filtered list)
	if results[0].Command != "npm start" {
		t.Errorf("expected first command to be 'npm start', got %q", results[0].Command)
	}
	if results[1].Command != "npm build" {
		t.Errorf("expected second command to be 'npm build', got %q", results[1].Command)
	}
}

func TestFzfTUISelector_GetSelectedCommands_NoSelection_ReturnsCurrent(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()
	selector.filteredCommands = selector.commands

	// Set current cursor position to index 1 (npm test)
	selector.currentIndex = 1

	// No explicit selections, should return current item
	results := selector.getSelectedCommands()

	if len(results) != 1 {
		t.Errorf("expected 1 command (current item), got %d", len(results))
	}

	if results[0].Command != "npm test" {
		t.Errorf("expected command 'npm test', got %q", results[0].Command)
	}
}

func TestFzfTUISelector_GetSelectedCommands_EmptyList(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{},
	}
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()
	selector.filteredCommands = selector.commands

	results := selector.getSelectedCommands()

	if len(results) != 0 {
		t.Errorf("expected 0 commands for empty list, got %d", len(results))
	}
}

func TestFzfTUISelector_GeneratePreview(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()
	selector.filteredCommands = selector.commands

	cmd := selector.filteredCommands[0]
	preview := selector.generatePreview(cmd)

	// Check that preview contains expected information
	if !contains(preview, "frontend") {
		t.Error("preview should contain location name 'frontend'")
	}
	if !contains(preview, "/path/to/frontend") {
		t.Error("preview should contain path '/path/to/frontend'")
	}
	if !contains(preview, "npm start") {
		t.Error("preview should contain command 'npm start'")
	}
	if !contains(preview, "npm") {
		t.Error("preview should contain type 'npm'")
	}
}

func TestFzfTUISelector_GeneratePreview_EmptyType(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{
			{
				Name:     "scripts",
				Location: "/scripts",
				Type:     "", // No type
				Commands: stringsToCommands([]string{"./deploy.sh"}),
			},
		},
	}
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()
	selector.filteredCommands = selector.commands

	cmd := selector.filteredCommands[0]
	preview := selector.generatePreview(cmd)

	// Should still work without type
	if !contains(preview, "scripts") {
		t.Error("preview should contain location name")
	}
	if !contains(preview, "./deploy.sh") {
		t.Error("preview should contain command")
	}
}

func TestFzfTUISelector_FormatSelectedItem(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()
	selector.filteredCommands = selector.commands

	// Test unselected item
	formatted := selector.formatListItem(0, false)
	if formatted != "  frontend: npm start" {
		t.Errorf("expected '  frontend: npm start', got %q", formatted)
	}

	// Test selected item
	formatted = selector.formatListItem(0, true)
	if formatted != "* frontend: npm start" {
		t.Errorf("expected '* frontend: npm start', got %q", formatted)
	}
}

func TestFzfTUISelector_LoadCommands(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()

	// Should have 5 commands total (3 from frontend, 2 from backend)
	if len(selector.commands) != 5 {
		t.Errorf("expected 5 commands, got %d", len(selector.commands))
	}

	// Check first command
	if selector.commands[0].DisplayName != "frontend" {
		t.Errorf("expected first command DisplayName 'frontend', got %q", selector.commands[0].DisplayName)
	}
	if selector.commands[0].Command != "npm start" {
		t.Errorf("expected first command 'npm start', got %q", selector.commands[0].Command)
	}
}

func TestFzfTUISelector_FuzzyFilter(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()

	tests := []struct {
		name          string
		query         string
		expectedCount int
	}{
		{
			name:          "empty query returns all",
			query:         "",
			expectedCount: 5,
		},
		{
			name:          "filter by npm",
			query:         "npm",
			expectedCount: 3,
		},
		{
			name:          "filter by go",
			query:         "go",
			expectedCount: 2,
		},
		{
			name:          "filter by test",
			query:         "test",
			expectedCount: 3, // "npm test", "go test ./...", and "frontend: npm start" (fuzzy matches t-e-s-t)
		},
		{
			name:          "fuzzy match",
			query:         "frt", // should match "frontend"
			expectedCount: 3,
		},
		{
			name:          "no matches",
			query:         "xyz",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := selector.fuzzyFilter(selector.commands, tt.query)
			if len(filtered) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(filtered))
			}
		})
	}
}

func TestFzfTUISelector_ClearSelections(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()
	selector.filteredCommands = selector.commands

	// Make some selections
	selector.toggleSelection(0)
	selector.toggleSelection(1)
	selector.toggleSelection(2)

	if len(selector.selectedIndices) == 0 {
		t.Error("expected some selections before clear")
	}

	// Clear selections
	selector.clearSelections()

	if len(selector.selectedIndices) != 0 {
		t.Errorf("expected 0 selections after clear, got %d", len(selector.selectedIndices))
	}
}

func TestFzfTUISelector_ConfirmSelection_DefaultAction(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()
	selector.filteredCommands = selector.commands
	selector.currentIndex = 0

	// confirmSelection should leave Action empty (default/execute behavior)
	selector.confirmSelection()

	if len(selector.results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(selector.results))
	}
	if selector.results[0].Action != "" {
		t.Errorf("expected empty Action for confirmSelection, got %q", selector.results[0].Action)
	}
}

func TestFzfTUISelector_EnterEditMode_SetsEditingState(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()
	selector.filteredCommands = selector.commands
	selector.currentIndex = 0

	// Should not be in edit mode initially
	if selector.editing {
		t.Error("expected editing to be false initially")
	}

	// Enter edit mode
	selector.enterEditMode()

	// Should now be in edit mode
	if !selector.editing {
		t.Error("expected editing to be true after enterEditMode")
	}

	// The editInput should be pre-filled with the current command
	if selector.editCommand != "npm start" {
		t.Errorf("expected editCommand 'npm start', got %q", selector.editCommand)
	}
	if selector.editDirectory != "/path/to/frontend" {
		t.Errorf("expected editDirectory '/path/to/frontend', got %q", selector.editDirectory)
	}
	if selector.editDisplayName != "frontend" {
		t.Errorf("expected editDisplayName 'frontend', got %q", selector.editDisplayName)
	}
}

func TestFzfTUISelector_ConfirmEdit_SetsEditAction(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()
	selector.filteredCommands = selector.commands
	selector.currentIndex = 0

	// Enter edit mode
	selector.enterEditMode()

	// Simulate editing the command
	selector.editCommand = "npm start --port 3001"

	// Confirm the edit
	selector.confirmEdit()

	if len(selector.results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(selector.results))
	}
	if selector.results[0].Action != "edit" {
		t.Errorf("expected Action 'edit', got %q", selector.results[0].Action)
	}
	if selector.results[0].Command != "npm start --port 3001" {
		t.Errorf("expected modified command, got %q", selector.results[0].Command)
	}
	if selector.results[0].Directory != "/path/to/frontend" {
		t.Errorf("expected directory '/path/to/frontend', got %q", selector.results[0].Directory)
	}
}

func TestFzfTUISelector_CancelEdit_ReturnsToNormalMode(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()
	selector.filteredCommands = selector.commands
	selector.currentIndex = 0

	// Enter edit mode
	selector.enterEditMode()
	if !selector.editing {
		t.Fatal("expected editing to be true")
	}

	// Cancel edit
	selector.cancelEdit()

	// Should be back to normal mode with no results
	if selector.editing {
		t.Error("expected editing to be false after cancelEdit")
	}
	if len(selector.results) != 0 {
		t.Errorf("expected 0 results after cancel, got %d", len(selector.results))
	}
}

func TestFzfTUISelector_GetSelectedCount(t *testing.T) {
	cfg := createTestConfig()
	selector := createTestFzfSelector(cfg)

	selector.loadCommands()
	selector.filteredCommands = selector.commands

	// Initially 0
	if selector.getSelectedCount() != 0 {
		t.Errorf("expected 0 selected count initially")
	}

	// Select some
	selector.toggleSelection(0)
	selector.toggleSelection(2)

	if selector.getSelectedCount() != 2 {
		t.Errorf("expected 2 selected count, got %d", selector.getSelectedCount())
	}

	// Deselect one
	selector.toggleSelection(0)

	if selector.getSelectedCount() != 1 {
		t.Errorf("expected 1 selected count, got %d", selector.getSelectedCount())
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
