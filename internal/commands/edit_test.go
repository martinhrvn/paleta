package commands

import (
	"os"
	"testing"
)

func TestGetEditor(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "uses EDITOR env var when set",
			envValue: "nvim",
			expected: "nvim",
		},
		{
			name:     "uses full path from EDITOR",
			envValue: "/usr/bin/code",
			expected: "/usr/bin/code",
		},
		{
			name:     "falls back to vi when EDITOR is empty",
			envValue: "",
			expected: "vi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore original EDITOR
			originalEditor := os.Getenv("EDITOR")
			defer os.Setenv("EDITOR", originalEditor)

			if tt.envValue != "" {
				os.Setenv("EDITOR", tt.envValue)
			} else {
				os.Unsetenv("EDITOR")
			}

			editor := GetEditor()
			if editor != tt.expected {
				t.Errorf("GetEditor() = %q, expected %q", editor, tt.expected)
			}
		})
	}
}

func TestFindConfigForEdit(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(tmpDir string) error
		wantErr bool
	}{
		{
			name: "finds .pltrc in current directory",
			setup: func(tmpDir string) error {
				return os.WriteFile(tmpDir+"/.pltrc", []byte("locations: []"), 0644)
			},
			wantErr: false,
		},
		{
			name:    "returns error when no .pltrc found",
			setup:   func(tmpDir string) error { return nil },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "plt-edit-test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			if err := tt.setup(tmpDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get working directory: %v", err)
			}
			defer os.Chdir(oldWd)

			os.Chdir(tmpDir)

			path, err := FindConfigForEdit()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("FindConfigForEdit() error = %v", err)
				return
			}

			if path == "" {
				t.Error("Expected non-empty config path")
			}
		})
	}
}
