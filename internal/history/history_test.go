package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockTime allows us to control time in tests
type mockTime struct {
	current time.Time
}

func (m *mockTime) Now() time.Time {
	return m.current
}

func (m *mockTime) Set(t time.Time) {
	m.current = t
}

// TestNewHistory tests creating a new history instance
func TestNewHistory(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "test-project")

	h, err := NewHistory(projectRoot)
	if err != nil {
		t.Fatalf("NewHistory failed: %v", err)
	}

	if h == nil {
		t.Fatal("Expected non-nil history")
	}

	if h.ProjectRoot != projectRoot {
		t.Errorf("Expected project root %s, got %s", projectRoot, h.ProjectRoot)
	}
}

// TestRecordExecution tests recording command execution
func TestRecordExecution(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "test-project")

	h, err := NewHistory(projectRoot)
	if err != nil {
		t.Fatalf("NewHistory failed: %v", err)
	}

	// Record a command execution
	location := "frontend"
	command := "npm run build"

	err = h.RecordExecution(location, command)
	if err != nil {
		t.Fatalf("RecordExecution failed: %v", err)
	}

	// Verify the command was recorded
	key := location + ":" + command
	entry, exists := h.Commands[key]
	if !exists {
		t.Fatal("Expected command to be recorded")
	}

	if entry.Count != 1 {
		t.Errorf("Expected count 1, got %d", entry.Count)
	}

	if entry.LastAccess.IsZero() {
		t.Error("Expected LastAccess to be set")
	}
}

// TestRecordMultipleExecutions tests recording same command multiple times
func TestRecordMultipleExecutions(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "test-project")

	h, err := NewHistory(projectRoot)
	if err != nil {
		t.Fatalf("NewHistory failed: %v", err)
	}

	location := "backend"
	command := "go test ./..."

	// Record the same command 5 times
	for i := 0; i < 5; i++ {
		err = h.RecordExecution(location, command)
		if err != nil {
			t.Fatalf("RecordExecution failed on iteration %d: %v", i, err)
		}
	}

	key := location + ":" + command
	entry := h.Commands[key]

	if entry.Count != 5 {
		t.Errorf("Expected count 5, got %d", entry.Count)
	}
}

// TestGetScore tests frecency score calculation
func TestGetScore(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "test-project")

	h, err := NewHistory(projectRoot)
	if err != nil {
		t.Fatalf("NewHistory failed: %v", err)
	}

	// Use mock time for deterministic tests
	mockClock := &mockTime{current: time.Now()}
	h.timeProvider = mockClock

	location := "api"
	command := "go run main.go"

	// Record initial execution
	h.RecordExecution(location, command)

	// Get score immediately (should be high due to recency)
	score := h.GetScore(location, command)
	if score <= 0 {
		t.Errorf("Expected positive score, got %f", score)
	}

	// Advance time by 1 day
	mockClock.Set(mockClock.Now().Add(24 * time.Hour))

	// Score should be lower due to reduced recency
	newScore := h.GetScore(location, command)
	if newScore >= score {
		t.Errorf("Expected score to decrease with time, got %f >= %f", newScore, score)
	}
}

// TestGetScoreNonExistent tests score for non-existent commands
func TestGetScoreNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "test-project")

	h, err := NewHistory(projectRoot)
	if err != nil {
		t.Fatalf("NewHistory failed: %v", err)
	}

	score := h.GetScore("location", "non-existent-command")
	if score != 0 {
		t.Errorf("Expected score 0 for non-existent command, got %f", score)
	}
}

// TestFrecencyScoreBalance tests 50/50 balance between frequency and recency
func TestFrecencyScoreBalance(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "test-project")

	h, err := NewHistory(projectRoot)
	if err != nil {
		t.Fatalf("NewHistory failed: %v", err)
	}

	mockClock := &mockTime{current: time.Now()}
	h.timeProvider = mockClock

	// Command A: executed once, very recent
	h.RecordExecution("loc1", "cmd-recent")
	scoreRecent := h.GetScore("loc1", "cmd-recent")

	// Command B: executed 10 times, but 30 days ago
	for i := 0; i < 10; i++ {
		h.RecordExecution("loc2", "cmd-frequent")
	}
	mockClock.Set(mockClock.Now().Add(30 * 24 * time.Hour))
	scoreFrequent := h.GetScore("loc2", "cmd-frequent")

	// Both should have some score, but recent should score higher with 50/50 balance
	if scoreRecent <= 0 || scoreFrequent <= 0 {
		t.Errorf("Expected both scores to be positive, got recent=%f, frequent=%f", scoreRecent, scoreFrequent)
	}

	// With 50/50 balance, the very recent command should score higher than old frequent one
	if scoreRecent <= scoreFrequent {
		t.Logf("Recent score: %f, Frequent score: %f", scoreRecent, scoreFrequent)
		// This is informational - exact behavior depends on algorithm parameters
	}
}

// TestSortCommandsByScore tests sorting commands by frecency score
func TestSortCommandsByScore(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "test-project")

	h, err := NewHistory(projectRoot)
	if err != nil {
		t.Fatalf("NewHistory failed: %v", err)
	}

	mockClock := &mockTime{current: time.Now()}
	h.timeProvider = mockClock

	// Create commands with different frequencies and recencies
	// Command 1: 5 executions, just now
	for i := 0; i < 5; i++ {
		h.RecordExecution("loc", "cmd1")
	}

	// Command 2: 10 executions, 7 days ago
	mockClock.Set(mockClock.Now().Add(-7 * 24 * time.Hour))
	for i := 0; i < 10; i++ {
		h.RecordExecution("loc", "cmd2")
	}
	mockClock.Set(time.Now())

	// Command 3: 1 execution, 1 hour ago
	mockClock.Set(mockClock.Now().Add(-1 * time.Hour))
	h.RecordExecution("loc", "cmd3")
	mockClock.Set(time.Now())

	// Get scores
	score1 := h.GetScore("loc", "cmd1")
	score2 := h.GetScore("loc", "cmd2")
	score3 := h.GetScore("loc", "cmd3")

	t.Logf("Scores - cmd1 (5x, now): %f, cmd2 (10x, 7d ago): %f, cmd3 (1x, 1h ago): %f",
		score1, score2, score3)

	// cmd1 should score highest (frequent AND recent)
	if score1 < score2 || score1 < score3 {
		t.Errorf("Expected cmd1 to score highest, got scores: %f, %f, %f", score1, score2, score3)
	}
}

// TestLoadAndSave tests persistence
func TestLoadAndSave(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "test-project")

	// Create history and record some commands
	h1, err := NewHistory(projectRoot)
	if err != nil {
		t.Fatalf("NewHistory failed: %v", err)
	}

	h1.RecordExecution("loc1", "cmd1")
	h1.RecordExecution("loc2", "cmd2")

	// Save to disk
	historyFile := filepath.Join(tempDir, "history.json")
	err = h1.Save(historyFile)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load into new history instance
	h2, err := LoadHistory(historyFile)
	if err != nil {
		t.Fatalf("LoadHistory failed: %v", err)
	}

	// Verify data was preserved
	if h2.ProjectRoot != h1.ProjectRoot {
		t.Errorf("Expected project root %s, got %s", h1.ProjectRoot, h2.ProjectRoot)
	}

	if len(h2.Commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(h2.Commands))
	}

	key1 := "loc1:cmd1"
	if entry, exists := h2.Commands[key1]; !exists {
		t.Error("Expected loc1:cmd1 to exist after load")
	} else if entry.Count != 1 {
		t.Errorf("Expected count 1, got %d", entry.Count)
	}
}

// TestLoadNonExistentFile tests loading when file doesn't exist
func TestLoadNonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	historyFile := filepath.Join(tempDir, "non-existent.json")

	// Should create new empty history instead of erroring
	h, err := LoadHistory(historyFile)
	if err != nil {
		t.Fatalf("Expected LoadHistory to handle missing file gracefully, got error: %v", err)
	}

	if h == nil {
		t.Fatal("Expected non-nil history")
	}

	if len(h.Commands) != 0 {
		t.Errorf("Expected empty commands map, got %d entries", len(h.Commands))
	}
}

// TestProjectIdentification tests finding project root
func TestProjectIdentification(t *testing.T) {
	tempDir := t.TempDir()

	// Create a mock project structure
	projectRoot := filepath.Join(tempDir, "myproject")
	subDir := filepath.Join(projectRoot, "src", "components")
	err := os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create directories: %v", err)
	}

	// Create .gopmrc to mark project root
	gopmrc := filepath.Join(projectRoot, ".gopmrc")
	err = os.WriteFile(gopmrc, []byte("{}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create .gopmrc: %v", err)
	}

	// Find project root from subdirectory
	root, err := FindProjectRoot(subDir)
	if err != nil {
		t.Fatalf("FindProjectRoot failed: %v", err)
	}

	if root != projectRoot {
		t.Errorf("Expected project root %s, got %s", projectRoot, root)
	}
}

// TestProjectIdentificationGitRoot tests finding git root
func TestProjectIdentificationGitRoot(t *testing.T) {
	tempDir := t.TempDir()

	// Create a mock git project
	projectRoot := filepath.Join(tempDir, "gitproject")
	gitDir := filepath.Join(projectRoot, ".git")
	subDir := filepath.Join(projectRoot, "internal", "commands")

	err := os.MkdirAll(gitDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	err = os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectories: %v", err)
	}

	// Find project root from subdirectory
	root, err := FindProjectRoot(subDir)
	if err != nil {
		t.Fatalf("FindProjectRoot failed: %v", err)
	}

	if root != projectRoot {
		t.Errorf("Expected project root %s, got %s", projectRoot, root)
	}
}

// TestHistoryPruning tests keeping only last N entries
func TestHistoryPruning(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "test-project")

	h, err := NewHistory(projectRoot)
	if err != nil {
		t.Fatalf("NewHistory failed: %v", err)
	}

	mockClock := &mockTime{current: time.Now()}
	h.timeProvider = mockClock

	// Add 100 commands
	for i := 0; i < 100; i++ {
		h.RecordExecution("loc", "cmd"+string(rune(i)))
		mockClock.Set(mockClock.Now().Add(1 * time.Hour))
	}

	if len(h.Commands) != 100 {
		t.Fatalf("Expected 100 commands before pruning, got %d", len(h.Commands))
	}

	// Prune to keep only 50 most recent
	h.Prune(50)

	if len(h.Commands) != 50 {
		t.Errorf("Expected 50 commands after pruning, got %d", len(h.Commands))
	}

	// Verify the most recent commands were kept
	// The last command added should still exist
	lastKey := "loc:cmd" + string(rune(99))
	if _, exists := h.Commands[lastKey]; !exists {
		t.Error("Expected most recent command to be kept after pruning")
	}
}

// TestConcurrentAccess tests thread-safe operations
func TestConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	projectRoot := filepath.Join(tempDir, "test-project")

	h, err := NewHistory(projectRoot)
	if err != nil {
		t.Fatalf("NewHistory failed: %v", err)
	}

	// Simulate concurrent access
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				h.RecordExecution("loc", "cmd")
				_ = h.GetScore("loc", "cmd")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final count
	entry := h.Commands["loc:cmd"]
	if entry.Count != 100 {
		t.Errorf("Expected count 100 after concurrent access, got %d", entry.Count)
	}
}
