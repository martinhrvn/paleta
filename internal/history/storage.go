package history

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// HistoryFile represents the JSON structure for persistence
type HistoryFile struct {
	Version     string                  `json:"version"`
	ProjectRoot string                  `json:"project_root"`
	Commands    map[string]CommandEntry `json:"commands"`
}

const HistoryVersion = "1.0"

// Save writes the history to a JSON file with file locking
func (h *History) Save(filePath string) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// Create history file structure
	hf := HistoryFile{
		Version:     HistoryVersion,
		ProjectRoot: h.ProjectRoot,
		Commands:    h.Commands,
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(hf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	// Write to file with exclusive lock
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open history file: %w", err)
	}
	defer file.Close()

	// Acquire exclusive lock
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to lock history file: %w", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	// Write data
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	return nil
}

// LoadHistory loads history from a JSON file
// If the file doesn't exist, returns a new empty history
func LoadHistory(filePath string) (*History, error) {
	// If file doesn't exist, return empty history
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &History{
			Commands:     make(map[string]CommandEntry),
			timeProvider: &RealTime{},
		}, nil
	}

	// Open file with shared lock for reading
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open history file: %w", err)
	}
	defer file.Close()

	// Acquire shared lock
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_SH); err != nil {
		return nil, fmt.Errorf("failed to lock history file: %w", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	// Decode JSON
	var hf HistoryFile
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&hf); err != nil {
		return nil, fmt.Errorf("failed to decode history file: %w", err)
	}

	// Create history instance
	h := &History{
		ProjectRoot:  hf.ProjectRoot,
		Commands:     hf.Commands,
		timeProvider: &RealTime{},
	}

	// Ensure commands map is initialized
	if h.Commands == nil {
		h.Commands = make(map[string]CommandEntry)
	}

	return h, nil
}

// GetHistoryPath returns the path to the history file for a project
func GetHistoryPath(projectRoot string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to /tmp if home directory can't be determined
		return filepath.Join("/tmp", ".paleta", "history", hashProjectRoot(projectRoot)+".json")
	}

	historyDir := filepath.Join(homeDir, ".paleta", "history")
	return filepath.Join(historyDir, hashProjectRoot(projectRoot)+".json")
}

// hashProjectRoot creates a consistent hash for a project root path
func hashProjectRoot(projectRoot string) string {
	hash := sha256.Sum256([]byte(projectRoot))
	return hex.EncodeToString(hash[:])[:16] // Use first 16 chars for brevity
}

// LoadOrCreateHistory loads existing history or creates a new one for a project
func LoadOrCreateHistory(projectRoot string) (*History, error) {
	historyPath := GetHistoryPath(projectRoot)
	return LoadHistory(historyPath)
}

// SaveToDefaultLocation saves history to the default location based on project root
func (h *History) SaveToDefaultLocation() error {
	historyPath := GetHistoryPath(h.ProjectRoot)
	return h.Save(historyPath)
}
