package parsers

import (
	"testing"
)

func TestLoadEmbeddedDefaults(t *testing.T) {
	defaults, err := loadEmbeddedDefaults()
	if err != nil {
		t.Fatalf("Failed to load embedded defaults: %v", err)
	}

	if defaults.Parsers == nil {
		t.Fatal("Expected parsers to be initialized")
	}

	// Test that default parsers are present
	expectedParsers := []string{"npm", "yarn", "pnpm", "go", "python", "rust", "make", "docker", "gradle", "maven"}
	for _, name := range expectedParsers {
		if _, exists := defaults.Parsers[name]; !exists {
			t.Errorf("Expected parser %s to exist in defaults", name)
		}
	}

	// Test npm parser configuration
	npmParser := defaults.Parsers["npm"]
	if len(npmParser.DetectFiles) == 0 {
		t.Error("Expected npm parser to have detect files")
	}
	if npmParser.DetectFiles[0] != "package.json" {
		t.Errorf("Expected npm parser to detect package.json, got %s", npmParser.DetectFiles[0])
	}
	if npmParser.BuiltinParser != "package_json_scripts" {
		t.Errorf("Expected npm parser to use package_json_scripts, got %s", npmParser.BuiltinParser)
	}

	// Test go parser configuration
	goParser := defaults.Parsers["go"]
	if len(goParser.DetectFiles) == 0 {
		t.Error("Expected go parser to have detect files")
	}
	if goParser.DetectFiles[0] != "go.mod" {
		t.Errorf("Expected go parser to detect go.mod, got %s", goParser.DetectFiles[0])
	}
	if len(goParser.BaseCommands) == 0 {
		t.Error("Expected go parser to have base commands")
	}

	// Test python parser has multiple detect files
	pythonParser := defaults.Parsers["python"]
	if len(pythonParser.DetectFiles) == 0 {
		t.Error("Expected python parser to have detect files")
	}
	expectedFiles := []string{"pyproject.toml", "setup.py", "requirements.txt"}
	for _, expectedFile := range expectedFiles {
		found := false
		for _, detectFile := range pythonParser.DetectFiles {
			if detectFile == expectedFile {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected python parser to detect %s", expectedFile)
		}
	}
}

func TestLoadParsersConfig(t *testing.T) {
	// Test loading when no user config exists
	config, err := LoadParsersConfig()
	if err != nil {
		t.Fatalf("Failed to load parser config: %v", err)
	}

	if config.Parsers == nil {
		t.Fatal("Expected parsers to be initialized")
	}

	// Should have default parsers
	expectedParsers := []string{"npm", "yarn", "pnpm", "go"}
	for _, name := range expectedParsers {
		if _, exists := config.Parsers[name]; !exists {
			t.Errorf("Expected parser %s to exist in config", name)
		}
	}
}

func TestParserConfigGetParser(t *testing.T) {
	config, err := LoadParsersConfig()
	if err != nil {
		t.Fatalf("Failed to load parser config: %v", err)
	}

	// Test existing parser
	parser, exists := config.GetParser("npm")
	if !exists {
		t.Error("Expected npm parser to exist")
	}
	if parser.BuiltinParser != "package_json_scripts" {
		t.Errorf("Expected npm parser to use package_json_scripts, got %s", parser.BuiltinParser)
	}

	// Test non-existing parser
	_, exists = config.GetParser("nonexistent")
	if exists {
		t.Error("Expected nonexistent parser to not exist")
	}
}
