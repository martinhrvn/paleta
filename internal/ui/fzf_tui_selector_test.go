package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/history"
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
				Types:    config.Types{"npm"},
				Commands: stringsToCommands([]string{"npm start", "npm test", "npm build"}),
			},
			{
				Name:     "backend",
				Location: "/path/to/backend",
				Types:    config.Types{"go"},
				Commands: stringsToCommands([]string{"go run main.go", "go test ./..."}),
			},
		},
	}
}

// Helper to create a Model with test config (without starting program)
func createTestModel(cfg *config.Config) Model {
	m := NewModel(cfg, nil)
	m.loadCommands()
	m.filteredCommands = make([]CommandInfo, len(m.commands))
	copy(m.filteredCommands, m.commands)
	return m
}

func TestModel_ToggleSelection(t *testing.T) {
	m := createTestModel(createTestConfig())

	// Initially empty queue
	if len(m.queue) != 0 {
		t.Errorf("expected empty queue initially, got %d", len(m.queue))
	}

	// Toggle queues the item at position 1
	m.toggleSelection(0)
	if m.queuePosAt(0) != 1 {
		t.Errorf("expected index 0 queued at position 1, got %d", m.queuePosAt(0))
	}

	// Toggle again removes it from the queue
	m.toggleSelection(0)
	if m.queuePosAt(0) != 0 {
		t.Error("expected index 0 removed from queue after second toggle")
	}
}

func TestModel_ToggleMultipleSelections(t *testing.T) {
	m := createTestModel(createTestConfig())

	// Queue multiple items
	m.toggleSelection(0)
	m.toggleSelection(2)
	m.toggleSelection(4)

	if m.queuePosAt(0) == 0 {
		t.Error("expected index 0 to be queued")
	}
	if m.queuePosAt(2) == 0 {
		t.Error("expected index 2 to be queued")
	}
	if m.queuePosAt(4) == 0 {
		t.Error("expected index 4 to be queued")
	}

	// Index 1 and 3 should not be queued
	if m.queuePosAt(1) != 0 {
		t.Error("expected index 1 to NOT be queued")
	}
	if m.queuePosAt(3) != 0 {
		t.Error("expected index 3 to NOT be queued")
	}
}

func TestModel_ToggleOutOfBounds(t *testing.T) {
	m := createTestModel(createTestConfig())

	// Should not panic on out of bounds
	m.toggleSelection(-1)
	m.toggleSelection(100)

	// Should have nothing queued
	if len(m.queue) != 0 {
		t.Errorf("expected empty queue for out of bounds, got %d", len(m.queue))
	}
}

func TestModel_GetSelectedCommands_WithSelections(t *testing.T) {
	m := createTestModel(createTestConfig())

	// Queue items at indices 0 and 2
	m.toggleSelection(0)
	m.toggleSelection(2)

	results := m.getSelectedCommands()

	if len(results) != 2 {
		t.Errorf("expected 2 queued commands, got %d", len(results))
	}

	// Verify order (enqueue order)
	if results[0].Command != "npm start" {
		t.Errorf("expected first command to be 'npm start', got %q", results[0].Command)
	}
	if results[1].Command != "npm build" {
		t.Errorf("expected second command to be 'npm build', got %q", results[1].Command)
	}
}

func TestModel_GetSelectedCommands_UsesEnqueueOrderNotListOrder(t *testing.T) {
	m := createTestModel(createTestConfig())

	// Queue index 2 (npm build) BEFORE index 0 (npm start): the result order must
	// follow the enqueue order, not the list order.
	m.toggleSelection(2)
	m.toggleSelection(0)

	results := m.getSelectedCommands()
	if len(results) != 2 {
		t.Fatalf("expected 2 queued commands, got %d", len(results))
	}
	if results[0].Command != "npm build" {
		t.Errorf("expected first command 'npm build' (enqueued first), got %q", results[0].Command)
	}
	if results[1].Command != "npm start" {
		t.Errorf("expected second command 'npm start', got %q", results[1].Command)
	}
}

func TestModel_QueuePersistsAcrossFilter(t *testing.T) {
	m := createTestModel(createTestConfig())

	// Queue "npm start" (index 0), then filter to the "go" commands.
	m.toggleSelection(0)
	m.searchInput.SetValue("go")
	m.updateFilteredCommands()

	// The queued command is filtered out of view but must remain queued.
	if m.getSelectedCount() != 1 {
		t.Fatalf("expected queue to survive filtering, got %d", m.getSelectedCount())
	}
	results := m.getSelectedCommands()
	if len(results) != 1 || results[0].Command != "npm start" {
		t.Errorf("expected queued 'npm start' to persist, got %+v", results)
	}
}

func TestModel_GetSelectedCommands_CarriesEnv(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{
			{
				Name:     "api",
				Location: "/path/to/api",
				Env:      map[string]string{"PORT": "3000", "REGION": "eu"},
				Commands: []config.Command{
					{
						Name:    "dev",
						Command: "npm run dev",
						Env:     map[string]string{"PORT": "3001"}, // overrides location
					},
				},
			},
		},
	}

	m := createTestModel(cfg)
	m.toggleSelection(0)
	results := m.getSelectedCommands()

	if len(results) != 1 {
		t.Fatalf("expected 1 selected command, got %d", len(results))
	}
	// Command-level PORT overrides location-level; REGION inherited from location.
	if results[0].Env["PORT"] != "3001" {
		t.Errorf("PORT = %q, want 3001 (command overrides location)", results[0].Env["PORT"])
	}
	if results[0].Env["REGION"] != "eu" {
		t.Errorf("REGION = %q, want eu (inherited from location)", results[0].Env["REGION"])
	}
}

func TestModel_GetSelectedCommands_NoSelection_ReturnsCurrent(t *testing.T) {
	m := createTestModel(createTestConfig())

	// Set current cursor position to index 1 (npm test)
	m.currentIndex = 1

	// No explicit selections, should return current item
	results := m.getSelectedCommands()

	if len(results) != 1 {
		t.Errorf("expected 1 command (current item), got %d", len(results))
	}

	if results[0].Command != "npm test" {
		t.Errorf("expected command 'npm test', got %q", results[0].Command)
	}
}

func TestModel_GetSelectedCommands_EmptyList(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{},
	}
	m := createTestModel(cfg)

	results := m.getSelectedCommands()

	if len(results) != 0 {
		t.Errorf("expected 0 commands for empty list, got %d", len(results))
	}
}

func TestModel_GeneratePreview(t *testing.T) {
	m := createTestModel(createTestConfig())

	cmd := m.filteredCommands[0]
	preview := m.generatePreview(cmd)

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

func TestModel_GeneratePreview_EmptyType(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{
			{
				Name:     "scripts",
				Location: "/scripts",
				// No type
				Commands: stringsToCommands([]string{"./deploy.sh"}),
			},
		},
	}
	m := createTestModel(cfg)

	cmd := m.filteredCommands[0]
	preview := m.generatePreview(cmd)

	// Should still work without type
	if !contains(preview, "scripts") {
		t.Error("preview should contain location name")
	}
	if !contains(preview, "./deploy.sh") {
		t.Error("preview should contain command")
	}
}

func TestModel_FormatListItemPlain(t *testing.T) {
	t.Setenv("PLT_NO_ICONS", "1") // deterministic ASCII, no glyphs
	m := createTestModel(createTestConfig())
	display := m.filteredCommands[0].Display

	// Unqueued item: two-space badge + plain row text.
	if got := queueBadgePlain(0) + rowPlain(display, ""); got != "  frontend: npm start" {
		t.Errorf("expected '  frontend: npm start', got %q", got)
	}

	// Queued item: position 1 badge.
	if got := queueBadgePlain(1) + rowPlain(display, ""); got != "1 frontend: npm start" {
		t.Errorf("expected '1 frontend: npm start', got %q", got)
	}

	// Out of bounds yields empty.
	if got := m.formatListItem(-1, 0, nil); got != "" {
		t.Errorf("expected empty string for out of bounds, got %q", got)
	}
}

func TestModel_FormatListItemStyled(t *testing.T) {
	t.Setenv("PLT_NO_ICONS", "1")
	m := createTestModel(createTestConfig())

	// Visible (ANSI-stripped) text should carry the location and command.
	visible := ansi.Strip(m.formatListItem(0, 0, nil))
	if !contains(visible, "frontend:") {
		t.Errorf("styled item should contain 'frontend:', got %q", visible)
	}
	if !contains(visible, "npm start") {
		t.Errorf("styled item should contain 'npm start', got %q", visible)
	}

	// Queued item should carry its position badge.
	visible = ansi.Strip(m.formatListItem(0, 1, nil))
	if !contains(visible, "1") {
		t.Errorf("styled queued item should contain position '1', got %q", visible)
	}
}

// rowPlain (with icons enabled) should keep the location/folder glyph but no
// longer prefix the command with a terminal glyph.
func TestRowPlain_DropsCommandIcon(t *testing.T) {
	t.Setenv("PLT_NO_ICONS", "") // icons enabled
	got := rowPlain("frontend: npm start", "")
	want := locIcon() + "frontend: " + "npm start"
	if got != want {
		t.Errorf("rowPlain = %q, want %q (command icon should be removed)", got, want)
	}
}

// The project type appears in the list row as a trailing "[type]" badge, so it's
// visible without opening the preview.
func TestModel_ListRowShowsType(t *testing.T) {
	t.Setenv("PLT_NO_ICONS", "1")
	cfg := &config.Config{
		Locations: []config.Location{{
			Name:     "api",
			Location: "/api",
			Commands: []config.Command{{Name: "build", Command: "go build ./...", Type: "go"}},
		}},
	}
	m := createTestModel(cfg)

	// Plain (measurement) text includes the badge.
	if got := rowPlain(m.filteredCommands[0].Display, m.filteredCommands[0].Type); got != "api: build [go]" {
		t.Errorf("rowPlain = %q, want %q", got, "api: build [go]")
	}
	// Styled row renders the badge too.
	visible := ansi.Strip(m.formatListItem(0, 0, nil))
	if !contains(visible, "[go]") {
		t.Errorf("styled row should show type badge '[go]', got %q", visible)
	}
}

// Checked (queued, non-cursor) rows get a subtle surface background with
// accent-colored text so they stand out from unchecked rows.
func TestQueuedRowStyle_HasBackgroundAndAccent(t *testing.T) {
	bg, ok := queuedBaseStyle.GetBackground().(lipgloss.Color)
	if !ok || !strings.HasPrefix(string(bg), "#") {
		t.Errorf("queued row should have a background fill, got %#v", queuedBaseStyle.GetBackground())
	}
	fg, _ := queuedBaseStyle.GetForeground().(lipgloss.Color)
	if string(fg) != ccLavender {
		t.Errorf("queued row text should be accent (lavender %s), got %q", ccLavender, fg)
	}
}

func TestModel_RenderQueuedRow(t *testing.T) {
	t.Setenv("PLT_NO_ICONS", "1")
	m := createTestModel(createTestConfig())

	visible := ansi.Strip(m.renderQueuedRow(0, 1, nil, 40))
	if !contains(visible, "frontend:") || !contains(visible, "npm start") {
		t.Errorf("queued row missing content, got %q", visible)
	}
	if !contains(visible, "1") {
		t.Errorf("queued row should show position badge '1', got %q", visible)
	}
}

func TestModel_LoadCommands(t *testing.T) {
	cfg := createTestConfig()
	m := NewModel(cfg, nil)
	m.loadCommands()

	// Should have 5 commands total (3 from frontend, 2 from backend)
	if len(m.commands) != 5 {
		t.Errorf("expected 5 commands, got %d", len(m.commands))
	}

	// Check first command
	if m.commands[0].DisplayName != "frontend" {
		t.Errorf("expected first command DisplayName 'frontend', got %q", m.commands[0].DisplayName)
	}
	if m.commands[0].Command != "npm start" {
		t.Errorf("expected first command 'npm start', got %q", m.commands[0].Command)
	}
}

// A command (or location) name flagged by validateNames should surface as an
// invalid CommandInfo, while a valid sibling stays clean.
func TestModel_LoadCommands_MarksInvalidNames(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{{
			Name:     "root",
			Location: "/root",
			Commands: []config.Command{
				{Name: "test ui", Command: "go test ./ui/...", NameError: "contains a space"},
				{Name: "build", Command: "go build ./..."},
			},
		}},
	}
	m := NewModel(cfg, nil)
	m.loadCommands()

	if !m.commands[0].Invalid {
		t.Errorf("command with flagged name should be Invalid")
	}
	if m.commands[0].InvalidReason != "contains a space" {
		t.Errorf("InvalidReason = %q, want %q", m.commands[0].InvalidReason, "contains a space")
	}
	if m.commands[1].Invalid {
		t.Errorf("valid command 'build' should not be Invalid")
	}
}

// The warning banner appears only when the config carries name warnings.
func TestModel_WarningBanner(t *testing.T) {
	m := createTestModel(createTestConfig())
	if got := m.renderWarningBanner(); got != "" {
		t.Errorf("expected no banner for clean config, got %q", got)
	}

	m.config.Warnings = []config.Warning{
		{Kind: "name", Scope: "command", Context: "root: test ui", Name: "test ui", Reason: "contains a space"},
	}
	banner := ansi.Strip(m.renderWarningBanner())
	if !contains(banner, "config issue") || !contains(banner, "plt lint") {
		t.Errorf("expected config-issue warning in banner, got %q", banner)
	}
}

// A command whose @project:command reference couldn't be resolved (Command.Error)
// is marked Invalid in the list, just like an un-aliasable name.
func TestModel_LoadCommands_MarksUnresolvedAlias(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{{
			Name:     "root",
			Location: "/root",
			Commands: []config.Command{
				{Name: "ci", Command: "@root:missing", Error: `command "ci" in project "root": unknown command "missing"`},
				{Name: "build", Command: "go build ./..."},
			},
		}},
	}
	m := NewModel(cfg, nil)
	m.loadCommands()

	if !m.commands[0].Invalid {
		t.Errorf("command with unresolved alias should be Invalid")
	}
	if !contains(m.commands[0].InvalidReason, "unknown command") {
		t.Errorf("InvalidReason should carry the alias error, got %q", m.commands[0].InvalidReason)
	}
	if m.commands[1].Invalid {
		t.Errorf("valid command 'build' should not be Invalid")
	}
}

func TestModel_FuzzyFilter(t *testing.T) {
	m := createTestModel(createTestConfig())

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
			filtered := m.fuzzyFilter(m.commands, tt.query)
			if len(filtered) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(filtered))
			}
		})
	}
}

func TestModel_ClearSelections(t *testing.T) {
	m := createTestModel(createTestConfig())

	// Queue some commands
	m.toggleSelection(0)
	m.toggleSelection(1)
	m.toggleSelection(2)

	if len(m.queue) == 0 {
		t.Error("expected a non-empty queue before clear")
	}

	// Clear the queue
	m.clearSelections()

	if len(m.queue) != 0 {
		t.Errorf("expected empty queue after clear, got %d", len(m.queue))
	}
}

func TestModel_ConfirmSelection_DefaultAction(t *testing.T) {
	m := createTestModel(createTestConfig())
	m.currentIndex = 0

	// confirmSelection should leave Action empty (default/execute behavior)
	m.confirmSelection()

	if len(m.results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(m.results))
	}
	if m.results[0].Action != "" {
		t.Errorf("expected empty Action for confirmSelection, got %q", m.results[0].Action)
	}
}

func TestModel_EnterEditMode_SetsEditingState(t *testing.T) {
	m := createTestModel(createTestConfig())
	m.currentIndex = 0

	// Should not be in edit mode initially
	if m.editing {
		t.Error("expected editing to be false initially")
	}

	// Enter edit mode
	m.enterEditMode()

	// Should now be in edit mode
	if !m.editing {
		t.Error("expected editing to be true after enterEditMode")
	}

	// The editInput should be pre-filled with the current command
	if m.editCommand != "npm start" {
		t.Errorf("expected editCommand 'npm start', got %q", m.editCommand)
	}
	if m.editDirectory != "/path/to/frontend" {
		t.Errorf("expected editDirectory '/path/to/frontend', got %q", m.editDirectory)
	}
	if m.editDisplayName != "frontend" {
		t.Errorf("expected editDisplayName 'frontend', got %q", m.editDisplayName)
	}
}

func TestModel_ConfirmEdit_SetsEditAction(t *testing.T) {
	m := createTestModel(createTestConfig())
	m.currentIndex = 0

	// Enter edit mode
	m.enterEditMode()

	// Simulate editing the command
	m.editCommand = "npm start --port 3001"

	// Confirm the edit
	m.confirmEdit()

	if len(m.results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(m.results))
	}
	if m.results[0].Action != "edit" {
		t.Errorf("expected Action 'edit', got %q", m.results[0].Action)
	}
	if m.results[0].Command != "npm start --port 3001" {
		t.Errorf("expected modified command, got %q", m.results[0].Command)
	}
	if m.results[0].Directory != "/path/to/frontend" {
		t.Errorf("expected directory '/path/to/frontend', got %q", m.results[0].Directory)
	}
}

func TestModel_CancelEdit_ReturnsToNormalMode(t *testing.T) {
	m := createTestModel(createTestConfig())
	m.currentIndex = 0

	// Enter edit mode
	m.enterEditMode()
	if !m.editing {
		t.Fatal("expected editing to be true")
	}

	// Cancel edit
	m.cancelEdit()

	// Should be back to normal mode with no results
	if m.editing {
		t.Error("expected editing to be false after cancelEdit")
	}
	if len(m.results) != 0 {
		t.Errorf("expected 0 results after cancel, got %d", len(m.results))
	}
}

func TestModel_GetSelectedCount(t *testing.T) {
	m := createTestModel(createTestConfig())

	// Initially 0
	if m.getSelectedCount() != 0 {
		t.Errorf("expected 0 selected count initially")
	}

	// Select some
	m.toggleSelection(0)
	m.toggleSelection(2)

	if m.getSelectedCount() != 2 {
		t.Errorf("expected 2 selected count, got %d", m.getSelectedCount())
	}

	// Deselect one
	m.toggleSelection(0)

	if m.getSelectedCount() != 1 {
		t.Errorf("expected 1 selected count, got %d", m.getSelectedCount())
	}
}

func TestModel_ToggleSelectAll(t *testing.T) {
	m := createTestModel(createTestConfig())

	// Select all
	m.toggleSelectAll()
	if m.getSelectedCount() != 5 {
		t.Errorf("expected 5 selected after select all, got %d", m.getSelectedCount())
	}

	// Toggle again should deselect all
	m.toggleSelectAll()
	if m.getSelectedCount() != 0 {
		t.Errorf("expected 0 selected after deselect all, got %d", m.getSelectedCount())
	}
}

func TestModel_MoveCursorDown(t *testing.T) {
	m := createTestModel(createTestConfig())

	if m.currentIndex != 0 {
		t.Errorf("expected initial cursor at 0, got %d", m.currentIndex)
	}

	m.moveCursorDown()
	if m.currentIndex != 1 {
		t.Errorf("expected cursor at 1 after down, got %d", m.currentIndex)
	}

	// Move past end should clamp
	for i := 0; i < 10; i++ {
		m.moveCursorDown()
	}
	if m.currentIndex != 4 {
		t.Errorf("expected cursor clamped at 4, got %d", m.currentIndex)
	}
}

func TestModel_MoveCursorUp(t *testing.T) {
	m := createTestModel(createTestConfig())
	m.currentIndex = 3

	m.moveCursorUp()
	if m.currentIndex != 2 {
		t.Errorf("expected cursor at 2 after up, got %d", m.currentIndex)
	}

	// Move past beginning should clamp
	for i := 0; i < 10; i++ {
		m.moveCursorUp()
	}
	if m.currentIndex != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", m.currentIndex)
	}
}

func TestModel_KeyDown_MovesCursor(t *testing.T) {
	m := createTestModel(createTestConfig())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	um := updated.(Model)

	if um.currentIndex != 1 {
		t.Errorf("expected cursor at 1 after KeyDown, got %d", um.currentIndex)
	}
}

func TestModel_KeyUp_MovesCursor(t *testing.T) {
	m := createTestModel(createTestConfig())
	m.currentIndex = 2

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	um := updated.(Model)

	if um.currentIndex != 1 {
		t.Errorf("expected cursor at 1 after KeyUp, got %d", um.currentIndex)
	}
}

func TestModel_KeyTab_TogglesSelection(t *testing.T) {
	m := createTestModel(createTestConfig())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	um := updated.(Model)

	// Tab should queue index 0 and move cursor down
	if um.queuePosAt(0) != 1 {
		t.Error("expected index 0 to be queued after Tab")
	}
	if um.currentIndex != 1 {
		t.Errorf("expected cursor at 1 after Tab, got %d", um.currentIndex)
	}
}

func TestModel_KeyEsc_Quits(t *testing.T) {
	m := createTestModel(createTestConfig())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	um := updated.(Model)

	if !um.quitting {
		t.Error("expected quitting to be true after Esc with empty search")
	}
}

func TestModel_KeyEsc_ClearsSearchThenQuits(t *testing.T) {
	m := createTestModel(createTestConfig())
	m.searchInput.SetValue("npm")
	m.updateFilteredCommands()

	// First Esc: text present, so clear the search instead of quitting.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)
	if m.quitting {
		t.Error("expected Esc with text to clear search, not quit")
	}
	if m.searchInput.Value() != "" {
		t.Errorf("expected search cleared, got %q", m.searchInput.Value())
	}
	if len(m.filteredCommands) != len(m.commands) {
		t.Errorf("expected all %d commands restored, got %d", len(m.commands), len(m.filteredCommands))
	}

	// Second Esc: search empty, so quit.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)
	if !m.quitting {
		t.Error("expected second Esc with empty search to quit")
	}
}

func TestModel_KeyCtrlC_Quits(t *testing.T) {
	m := createTestModel(createTestConfig())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	um := updated.(Model)

	if !um.quitting {
		t.Error("expected quitting to be true after Ctrl+C")
	}
}

func TestModel_KeyCtrlA_SelectsAll(t *testing.T) {
	m := createTestModel(createTestConfig())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	um := updated.(Model)

	if um.getSelectedCount() != 5 {
		t.Errorf("expected 5 selected after Ctrl+A, got %d", um.getSelectedCount())
	}
}

func TestModel_HelpLine_FrecencyTogglesShowKeyAndState(t *testing.T) {
	m := createTestModel(createTestConfig())

	// Frecency ON should show the shortcut, label and ON state in the help line.
	m.frecencyEnabled = true
	got := ansi.Strip(m.View())
	for _, want := range []string{"^F", "frecency", "ON"} {
		if !strings.Contains(got, want) {
			t.Errorf("frecency-on help = %q, want to contain %q", got, want)
		}
	}

	// Frecency OFF should still show the shortcut and label, but OFF state.
	m.frecencyEnabled = false
	got = ansi.Strip(m.View())
	for _, want := range []string{"^F", "frecency", "OFF"} {
		if !strings.Contains(got, want) {
			t.Errorf("frecency-off help = %q, want to contain %q", got, want)
		}
	}
}

func TestModel_HelpLine_FocusTogglesShowKeyAndState(t *testing.T) {
	cfg := createTestConfig()
	cfg.Locations[0].Focused = true // ensure focus is available
	m := createTestModel(cfg)

	m.focusActive = true
	got := ansi.Strip(m.View())
	for _, want := range []string{"^T", "focus", "ON"} {
		if !strings.Contains(got, want) {
			t.Errorf("focus-on help = %q, want to contain %q", got, want)
		}
	}

	m.focusActive = false
	got = ansi.Strip(m.View())
	for _, want := range []string{"^T", "focus", "OFF"} {
		if !strings.Contains(got, want) {
			t.Errorf("focus-off help = %q, want to contain %q", got, want)
		}
	}
}

func TestModel_KeyCtrlF_TogglesFrecency(t *testing.T) {
	m := createTestModel(createTestConfig())
	initial := m.frecencyEnabled

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	um := updated.(Model)

	if um.frecencyEnabled == initial {
		t.Error("expected frecencyEnabled to toggle after Ctrl+F")
	}
}

func TestModel_EnterEditMode_EmptyList(t *testing.T) {
	cfg := &config.Config{
		Locations: []config.Location{},
	}
	m := createTestModel(cfg)

	// Should not panic on empty list
	m.enterEditMode()

	if m.editing {
		t.Error("should not enter edit mode with empty list")
	}
}

func TestModel_ViewportScrolling(t *testing.T) {
	m := createTestModel(createTestConfig())
	m.height = 10 // Small terminal: 10 - 3 lines chrome = 7 visible rows

	// With 5 items and 7 visible rows, no scrolling needed
	if m.viewportOffset != 0 {
		t.Errorf("expected viewport offset 0 with enough space, got %d", m.viewportOffset)
	}

	// Create a model with many commands to test scrolling
	cmds := make([]string, 20)
	for i := range cmds {
		cmds[i] = "cmd" + string(rune('a'+i%26))
	}
	bigCfg := &config.Config{
		Locations: []config.Location{
			{
				Name:     "big",
				Location: "/big",
				Commands: stringsToCommands(cmds),
			},
		},
	}
	m2 := createTestModel(bigCfg)
	m2.height = 10 // 7 visible rows

	// Move cursor past visible area
	for i := 0; i < 8; i++ {
		m2.moveCursorDown()
	}

	// Viewport should have scrolled
	if m2.viewportOffset == 0 {
		t.Error("expected viewport to scroll when cursor moves past visible area")
	}
}

func TestModel_UpdateFilteredCommands(t *testing.T) {
	m := createTestModel(createTestConfig())

	// Set search query and update
	m.searchInput.SetValue("npm")
	m.updateFilteredCommands()

	if len(m.filteredCommands) != 3 {
		t.Errorf("expected 3 filtered commands for 'npm', got %d", len(m.filteredCommands))
	}

	// Cursor should be reset to 0
	if m.currentIndex != 0 {
		t.Errorf("expected cursor reset to 0, got %d", m.currentIndex)
	}
}

func TestModel_View_ReturnsString(t *testing.T) {
	m := createTestModel(createTestConfig())
	m.width = 80
	m.height = 24

	view := m.View()

	if view == "" {
		t.Error("expected non-empty view output")
	}

	// Should contain command text
	if !contains(view, "frontend") {
		t.Error("view should contain 'frontend'")
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

func TestModel_LoadCommands_MultiType(t *testing.T) {
	t.Setenv("PLT_NO_ICONS", "1") // deterministic ASCII for row-text assertions
	cfg := &config.Config{
		Locations: []config.Location{
			{
				Name:  "dotfiles",
				Types: config.Types{"npm", "compose"},
				Commands: []config.Command{
					{Name: "build", Command: "npm run build", Type: "npm"},
					{Name: "up", Command: "docker compose up", Type: "compose"},
				},
			},
		},
	}
	m := createTestModel(cfg)

	var build, up *CommandInfo
	for i := range m.commands {
		switch m.commands[i].Command {
		case "npm run build":
			build = &m.commands[i]
		case "docker compose up":
			up = &m.commands[i]
		}
	}
	if build == nil || up == nil {
		t.Fatalf("expected both commands loaded, got %+v", m.commands)
	}
	// The type is no longer folded into Display; it renders as a trailing badge
	// (see rowContent / rowPlain), so Display carries just the bare name.
	if build.Display != "dotfiles: build" {
		t.Errorf("build display = %q, want 'dotfiles: build'", build.Display)
	}
	if up.Display != "dotfiles: up" {
		t.Errorf("up display = %q, want 'dotfiles: up'", up.Display)
	}
	// CommandInfo.Type is per-command, driving both the type badge and the preview.
	if build.Type != "npm" || up.Type != "compose" {
		t.Errorf("per-command types = %q/%q, want npm/compose", build.Type, up.Type)
	}
	// The disambiguating type shows in the row via the badge.
	if got := rowPlain(build.Display, build.Type); got != "dotfiles: build [npm]" {
		t.Errorf("build row = %q, want 'dotfiles: build [npm]'", got)
	}
	if got := rowPlain(up.Display, up.Type); got != "dotfiles: up [compose]" {
		t.Errorf("up row = %q, want 'dotfiles: up [compose]'", got)
	}
}

func TestModel_GeneratePreview_ShowsStats(t *testing.T) {
	m := createTestModel(createTestConfig())

	// Inject a controlled history with one run of "frontend: npm start".
	h, _ := history.NewHistory("/tmp")
	_ = h.RecordExecution("frontend", "npm start")
	m.history = h

	// filteredCommands[0] is "frontend: npm start".
	preview := m.generatePreview(m.filteredCommands[0])
	for _, want := range []string{"Runs", "Last used", "First run", "Score"} {
		if !contains(preview, want) {
			t.Errorf("preview missing %q:\n%s", want, preview)
		}
	}

	// A command with no history shows no stats lines.
	noHist := m.generatePreview(m.filteredCommands[1]) // "frontend: npm test", never run
	if contains(noHist, "Runs") {
		t.Errorf("expected no stats for an unrun command:\n%s", noHist)
	}
}

func TestFuzzySubsequenceIndices(t *testing.T) {
	cases := []struct {
		text, query string
		want        []int
	}{
		{"npm run dev", "nrd", []int{0, 4, 8}}, // n(0) r(4) d(8)
		{"npm run dev", "npm", []int{0, 1, 2}},
		{"npm run dev", "xyz", nil}, // no match
		{"npm run dev", "", nil},    // empty query
		{"abcabc", "cc", []int{2, 5}},
	}
	for _, c := range cases {
		got := fuzzySubsequenceIndices(c.text, c.query)
		if len(got) != len(c.want) {
			t.Errorf("indices(%q,%q) = %v, want %v", c.text, c.query, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("indices(%q,%q) = %v, want %v", c.text, c.query, got, c.want)
				break
			}
		}
	}
}

func TestHighlightMatches_PreservesVisibleText(t *testing.T) {
	base := listCommandStyle
	hl := matchStyle
	text := "npm run dev"

	// Non-empty match set.
	matched := map[int]bool{0: true, 4: true, 8: true}
	got := ansi.Strip(highlightMatches(text, matched, base, hl))
	if got != text {
		t.Errorf("highlightMatches stripped = %q, want %q", got, text)
	}

	// Empty match set still yields the same visible text.
	gotEmpty := ansi.Strip(highlightMatches(text, nil, base, hl))
	if gotEmpty != text {
		t.Errorf("highlightMatches(nil) stripped = %q, want %q", gotEmpty, text)
	}
}

func TestTheme_TrueColorAndEffects(t *testing.T) {
	if !previewTitleStyle.GetBold() {
		t.Error("previewTitleStyle should be bold")
	}
	if !matchStyle.GetBold() {
		t.Error("matchStyle should be bold")
	}
	fg, ok := matchStyle.GetForeground().(lipgloss.Color)
	if !ok || !strings.HasPrefix(string(fg), "#") {
		t.Errorf("matchStyle foreground should be a truecolor hex, got %#v", matchStyle.GetForeground())
	}
}
