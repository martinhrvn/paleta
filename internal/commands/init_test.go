package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateDefaultConfig(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".pltrc")

	// Test creating config in an empty directory
	err := CreateDefaultConfig(configPath)
	if err != nil {
		t.Fatalf("CreateDefaultConfig failed: %v", err)
	}

	// Verify the file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Verify the file has content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read created config: %v", err)
	}

	if len(content) == 0 {
		t.Error("Config file is empty")
	}

	// Verify it contains expected sections
	contentStr := string(content)
	expectedSections := []string{
		"locations:",
		"name:",
		"location:",
		"type:",
		"commands:",
	}

	for _, section := range expectedSections {
		if !contains(contentStr, section) {
			t.Errorf("Config file missing expected section: %s", section)
		}
	}
}

func TestCreateDefaultConfigFileExists(t *testing.T) {
	// Create a temporary directory with an existing config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".pltrc")

	// Create an existing file
	existingContent := "existing content"
	err := os.WriteFile(configPath, []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Try to create config when file exists (with force=false)
	err = CreateDefaultConfig(configPath)
	if err == nil {
		t.Error("Expected error when file exists, got nil")
	}

	// Verify the original content is unchanged
	content, _ := os.ReadFile(configPath)
	if string(content) != existingContent {
		t.Error("Existing file was modified")
	}
}

func TestCreateDefaultConfigWithForce(t *testing.T) {
	// Create a temporary directory with an existing config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".pltrc")

	// Create an existing file
	existingContent := "existing content"
	err := os.WriteFile(configPath, []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Force create config
	err = CreateDefaultConfigWithForce(configPath, true)
	if err != nil {
		t.Fatalf("CreateDefaultConfigWithForce failed: %v", err)
	}

	// Verify the file was overwritten
	content, _ := os.ReadFile(configPath)
	if string(content) == existingContent {
		t.Error("Existing file was not overwritten")
	}

	// Verify it contains the default config
	contentStr := string(content)
	if !contains(contentStr, "locations:") {
		t.Error("Config file missing expected content")
	}
}

func TestDefaultConfigTemplate(t *testing.T) {
	template := DefaultConfigTemplate()

	if len(template) == 0 {
		t.Error("Default config template is empty")
	}

	// Verify it's valid YAML-like structure
	expectedSections := []string{
		"locations:",
		"# Example",
		"name:",
		"location:",
		"type:",
		"commands:",
	}

	for _, section := range expectedSections {
		if !contains(template, section) {
			t.Errorf("Template missing expected section: %s", section)
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
