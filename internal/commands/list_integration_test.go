package commands_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/martinhrvn/paleta/internal/commands"
	"github.com/martinhrvn/paleta/internal/config"
)

func TestListCommandsIntegration(t *testing.T) {
	// Save current working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	tests := []struct {
		name     string
		dir      string
		expected []string
	}{
		{
			name: "npm-monorepo",
			dir:  "../../examples/npm-monorepo",
			expected: []string{
				// Command names from npm (base commands and scripts)
				"backend:install",
				"backend:audit",
				"backend:outdated",
				"backend:update",
				// Scripts from package.json (names only)
				"backend:start",
				"backend:build",
				"backend:test",
				"backend:dev",
				// Command names from npm (base commands and scripts)
				"frontend:install",
				"frontend:audit",
				"frontend:outdated",
				"frontend:update",
				// Scripts from package.json (names only)
				"frontend:start",
				"frontend:build",
				"frontend:test",
				"frontend:dev",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Change to the test directory
			testDir := filepath.Join(originalWd, tt.dir)
			if err := os.Chdir(testDir); err != nil {
				t.Fatalf("Failed to change to test directory: %v", err)
			}

			// Load config from the test directory
			cfg, err := config.LoadConfigFromDiscovery()
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			// Get command list
			cmdList := commands.ListCommands(cfg)

			// Sort both expected and actual for consistent comparison
			sort.Strings(cmdList)
			sort.Strings(tt.expected)

			// Check that all expected commands are present
			if len(cmdList) != len(tt.expected) {
				t.Errorf("Expected %d commands, got %d", len(tt.expected), len(cmdList))
				t.Errorf("Expected: %v", tt.expected)
				t.Errorf("Got: %v", cmdList)
			}

			// Check each command
			for i, expected := range tt.expected {
				if i >= len(cmdList) {
					t.Errorf("Missing command: %s", expected)
					continue
				}
				if cmdList[i] != expected {
					t.Errorf("Command mismatch at index %d:\nExpected: %s\nGot: %s", i, expected, cmdList[i])
				}
			}
		})
	}
}

func TestFormatForFzfIntegration(t *testing.T) {
	// Save current working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Change to npm-monorepo directory
	testDir := filepath.Join(originalWd, "../../examples/npm-monorepo")
	if err := os.Chdir(testDir); err != nil {
		t.Fatalf("Failed to change to test directory: %v", err)
	}

	// Load config
	cfg, err := config.LoadConfigFromDiscovery()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Get fzf formatted commands
	fzfList := commands.FormatForFzf(cfg)

	expectedEntries := []string{
		// Command names from npm (base commands and scripts)
		"[backend] install",
		"[backend] audit",
		"[backend] outdated",
		"[backend] update",
		// Scripts from package.json (names only)
		"[backend] start",
		"[backend] build",
		"[backend] test",
		"[backend] dev",
		// Command names from npm (base commands and scripts)
		"[frontend] install",
		"[frontend] audit",
		"[frontend] outdated",
		"[frontend] update",
		// Scripts from package.json (names only)
		"[frontend] start",
		"[frontend] build",
		"[frontend] test",
		"[frontend] dev",
	}

	// Sort for consistent comparison
	sort.Strings(fzfList)
	sort.Strings(expectedEntries)

	if len(fzfList) != len(expectedEntries) {
		t.Errorf("Expected %d fzf entries, got %d", len(expectedEntries), len(fzfList))
		t.Errorf("Expected: %v", expectedEntries)
		t.Errorf("Got: %v", fzfList)
	}

	for i, expected := range expectedEntries {
		if i >= len(fzfList) {
			t.Errorf("Missing fzf entry: %s", expected)
			continue
		}
		if fzfList[i] != expected {
			t.Errorf("Fzf entry mismatch at index %d:\nExpected: %s\nGot: %s", i, expected, fzfList[i])
		}
	}
}
