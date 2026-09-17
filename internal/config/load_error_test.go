package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadConfig_InvalidFileNamesPath: discovery may load a .pltrc several
// directories up, so a parse error has to say which file is broken — a bare
// "yaml: line 4" leaves the user hunting.
func TestLoadConfig_InvalidFileNamesPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigFileName)
	if err := os.WriteFile(path, []byte("locations:\n  - name: a\n   location: b\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() succeeded on a malformed file")
	}

	var invalid *InvalidConfigError
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v (%T), want an *InvalidConfigError", err, err)
	}
	if invalid.Path != path {
		t.Errorf("Path = %q, want the absolute path %q", invalid.Path, path)
	}
	if !strings.Contains(invalid.Err.Error(), "yaml") {
		t.Errorf("wrapped error = %v, want the parse failure", invalid.Err)
	}
}

// TestLoadConfig_InvalidGlobNamesPath: the same applies to a file that parses but
// can't be processed — it is still this file the user has to fix.
func TestLoadConfig_InvalidGlobNamesPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigFileName)
	if err := os.WriteFile(path, []byte("locations:\n  - name: a\n    location: \"a/*/b\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var invalid *InvalidConfigError
	if _, err := LoadConfig(path); !errors.As(err, &invalid) {
		t.Fatalf("error = %v (%T), want an *InvalidConfigError", err, err)
	}
}

// TestLoadConfig_MissingFileStaysNotExist: "no file here" is not "broken file" —
// callers test for it with errors.Is, and the wizard treats it as an empty start.
func TestLoadConfig_MissingFileStaysNotExist(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), ConfigFileName))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error = %v, want it to unwrap to fs.ErrNotExist", err)
	}
	var invalid *InvalidConfigError
	if errors.As(err, &invalid) {
		t.Error("a missing file was reported as an invalid one")
	}
}
