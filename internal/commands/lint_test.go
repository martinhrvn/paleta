package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhrvn/paleta/internal/config"
)

func TestSanitizeName(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		isCommand bool
		want      string
	}{
		{"space in command", "test ui", true, "test_ui"},
		{"asterisk in command", "build*", true, "build_"},
		{"ampersand in command", "a&b", true, "a_b"},
		{"colon allowed in command", "test:watch", true, "test:watch"},
		{"dot dash underscore kept", "a.b-c_d", true, "a.b-c_d"},
		{"space in location", "my proj", false, "my_proj"},
		{"slash allowed in location", "packages/search", false, "packages/search"},
		{"colon not allowed in location", "a:b", false, "a_b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeName(tc.in, tc.isCommand); got != tc.want {
				t.Errorf("SanitizeName(%q, %v) = %q, want %q", tc.in, tc.isCommand, got, tc.want)
			}
		})
	}
}

func TestFixConfigFile_SanitizesNamesAndPreservesComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pltrc")
	original := `# paleta config
locations:
    - name: root
      location: .
      commands:
        - name: test ui # run the ui tests
          command: go test ./...
        - name: build
          command: go build ./...
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	fixes, err := FixConfigFile(path)
	if err != nil {
		t.Fatalf("FixConfigFile: %v", err)
	}
	if len(fixes) != 1 {
		t.Fatalf("expected 1 fix, got %d: %+v", len(fixes), fixes)
	}
	if fixes[0].Before != "test ui" || fixes[0].After != "test_ui" {
		t.Errorf("fix = %+v, want test ui -> test_ui", fixes[0])
	}

	fixed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(fixed)
	if !strings.Contains(got, "test_ui") {
		t.Errorf("fixed file should contain 'test_ui', got:\n%s", got)
	}
	if strings.Contains(got, "test ui") {
		t.Errorf("fixed file should no longer contain 'test ui', got:\n%s", got)
	}
	// Comments survive the round-trip.
	if !strings.Contains(got, "# paleta config") {
		t.Errorf("head comment lost, got:\n%s", got)
	}
	if !strings.Contains(got, "run the ui tests") {
		t.Errorf("line comment lost, got:\n%s", got)
	}

	// The fixed file re-validates clean.
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig after fix: %v", err)
	}
	if len(cfg.Warnings) != 0 {
		t.Errorf("expected no warnings after fix, got %+v", cfg.Warnings)
	}
}

func TestFixConfigFile_NoChangesWhenClean(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pltrc")
	clean := `locations:
    - name: root
      location: .
      commands:
        - name: build
          command: go build ./...
`
	if err := os.WriteFile(path, []byte(clean), 0644); err != nil {
		t.Fatal(err)
	}
	fixes, err := FixConfigFile(path)
	if err != nil {
		t.Fatalf("FixConfigFile: %v", err)
	}
	if len(fixes) != 0 {
		t.Errorf("expected no fixes for clean config, got %+v", fixes)
	}
}

func TestFormatLintReport(t *testing.T) {
	if got := FormatLintReport(nil); !strings.Contains(got, "No") {
		t.Errorf("clean report should say no issues, got %q", got)
	}
	report := FormatLintReport([]config.Warning{
		{Scope: "command", Context: "root: test ui", Name: "test ui", Reason: "contains a space"},
	})
	if !strings.Contains(report, "test ui") || !strings.Contains(report, "contains a space") {
		t.Errorf("report missing detail, got %q", report)
	}
	if !strings.Contains(report, "--fix") {
		t.Errorf("report should mention --fix, got %q", report)
	}
}
