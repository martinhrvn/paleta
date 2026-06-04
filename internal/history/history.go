package history

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// TimeProvider allows mocking time for tests
type TimeProvider interface {
	Now() time.Time
}

// RealTime provides actual system time
type RealTime struct{}

func (r *RealTime) Now() time.Time {
	return time.Now()
}

// CommandEntry tracks frequency and recency for a single command
type CommandEntry struct {
	Count       int       `json:"count"`
	LastAccess  time.Time `json:"last_access"`
	FirstAccess time.Time `json:"first_access"`
}

// History manages command execution history for a project
type History struct {
	ProjectRoot  string                  `json:"project_root"`
	Commands     map[string]CommandEntry `json:"commands"`
	weights      FrecencyWeights         `json:"-"`
	timeProvider TimeProvider            `json:"-"`
	mu           sync.RWMutex            `json:"-"`
}

// NewHistory creates a new history instance for a project
func NewHistory(projectRoot string) (*History, error) {
	return &History{
		ProjectRoot:  projectRoot,
		Commands:     make(map[string]CommandEntry),
		weights:      DefaultWeights,
		timeProvider: &RealTime{},
	}, nil
}

// SetWeights configures the frequency/recency balance used when scoring. Pass
// weights built with NewWeights so they are normalized.
func (h *History) SetWeights(w FrecencyWeights) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.weights = w
}

// ScoreEntry returns the frecency score for an entry using the history's
// configured weights. now is the reference time (injected for testability).
func (h *History) ScoreEntry(entry CommandEntry, now time.Time) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return calculateFrecencyScoreWithWeights(entry, now, effectiveWeights(h.weights))
}

// RecordExecution records that a command was executed
func (h *History) RecordExecution(location, command string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := location + ":" + command
	now := h.timeProvider.Now()

	entry, exists := h.Commands[key]
	if !exists {
		entry = CommandEntry{
			Count:       0,
			FirstAccess: now,
		}
	}

	entry.Count++
	entry.LastAccess = now
	h.Commands[key] = entry

	return nil
}

// All returns a copy of every recorded command entry, keyed by
// "location:command". The copy lets callers iterate without holding the lock.
func (h *History) All() map[string]CommandEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make(map[string]CommandEntry, len(h.Commands))
	for k, v := range h.Commands {
		out[k] = v
	}
	return out
}

// GetEntry retrieves the command entry for a given location and command
func (h *History) GetEntry(location, command string) (CommandEntry, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	key := location + ":" + command
	entry, exists := h.Commands[key]
	return entry, exists
}

// GetScore calculates the frecency score for a command
func (h *History) GetScore(location, command string) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	key := location + ":" + command
	entry, exists := h.Commands[key]
	if !exists {
		return 0
	}

	return calculateFrecencyScoreWithWeights(entry, h.timeProvider.Now(), effectiveWeights(h.weights))
}

// Prune keeps only the N most recent commands
func (h *History) Prune(maxEntries int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.Commands) <= maxEntries {
		return
	}

	// Create slice of entries with keys for sorting
	type entryWithKey struct {
		key   string
		entry CommandEntry
	}

	entries := make([]entryWithKey, 0, len(h.Commands))
	for key, entry := range h.Commands {
		entries = append(entries, entryWithKey{key, entry})
	}

	// Sort by LastAccess descending (most recent first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].entry.LastAccess.After(entries[j].entry.LastAccess)
	})

	// Keep only the first maxEntries
	h.Commands = make(map[string]CommandEntry, maxEntries)
	for i := 0; i < maxEntries && i < len(entries); i++ {
		h.Commands[entries[i].key] = entries[i].entry
	}
}

// FindProjectRoot finds the project root directory by looking for .pltrc or .git
func FindProjectRoot(startPath string) (string, error) {
	currentPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	// Walk up the directory tree
	for {
		// Check for .pltrc
		pltrc := filepath.Join(currentPath, ".pltrc")
		if _, err := os.Stat(pltrc); err == nil {
			return currentPath, nil
		}

		// Check for .git directory
		gitDir := filepath.Join(currentPath, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return currentPath, nil
		}

		// Move up one directory
		parent := filepath.Dir(currentPath)
		if parent == currentPath {
			// Reached filesystem root, use current working directory
			cwd, err := os.Getwd()
			if err != nil {
				return "", err
			}
			return cwd, nil
		}
		currentPath = parent
	}
}
