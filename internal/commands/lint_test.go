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

func TestFormatLintReport_DeprecatedKeys(t *testing.T) {
	report := FormatLintReport([]config.Warning{{
		Kind:    "deprecated",
		Scope:   "location",
		Context: "web",
		Name:    "include",
		Reason:  `"include:" is deprecated — use "include_commands:" (run 'plt lint --fix')`,
	}})

	if !strings.Contains(report, "include_commands") || !strings.Contains(report, "web") {
		t.Errorf("report missing detail, got %q", report)
	}
	// The charset guidance belongs to name issues only; a deprecated key is not one.
	if strings.Contains(report, "alias-safe") || strings.Contains(report, "@project:command aliases") {
		t.Errorf("deprecated-key report should not print name-charset guidance, got %q", report)
	}
}

func TestFormatLintReport_IgnoredKeys(t *testing.T) {
	report := FormatLintReport([]config.Warning{{
		Kind:    "ignored",
		Scope:   "location",
		Context: "services/*",
		Name:    "legacy",
		Reason:  `override key matches no folder expanded from "services/*"`,
	}})

	if !strings.Contains(report, "legacy") || !strings.Contains(report, "no folder") {
		t.Errorf("report missing detail, got %q", report)
	}
	if strings.Contains(report, "@project:command aliases") {
		t.Errorf("ignored-key report should not print name-charset guidance, got %q", report)
	}
}

func TestFixConfigFile_RenamesDeprecatedKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pltrc")
	original := `# paleta config
locations:
    - name: root
      location: .
      type: npm
      # only the build scripts
      include: # whitelist
        - build*
      exclude:
        - "*:watch"
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	fixes, err := FixConfigFile(path)
	if err != nil {
		t.Fatalf("FixConfigFile: %v", err)
	}
	if len(fixes) != 2 {
		t.Fatalf("fixes = %+v, want one per renamed key", fixes)
	}
	if fixes[0].Scope != "key" || fixes[0].Before != "include" || fixes[0].After != "include_commands" {
		t.Errorf("fixes[0] = %+v, want the include rename", fixes[0])
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(out)
	for _, want := range []string{"include_commands:", "exclude_commands:", "# only the build scripts", "# whitelist", "# paleta config"} {
		if !strings.Contains(got, want) {
			t.Errorf("rewritten config missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\n      include:") || strings.Contains(got, "\n      exclude:") {
		t.Errorf("deprecated keys still present:\n%s", got)
	}

	// Nothing left to fix on a second run.
	again, err := FixConfigFile(path)
	if err != nil {
		t.Fatalf("second FixConfigFile: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("second run reported fixes: %+v", again)
	}
}

func TestFixConfigFile_RenamesInsideOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pltrc")
	original := `locations:
    - location: "services/*"
      overrides:
        api:
          include:
            - build*
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	fixes, err := FixConfigFile(path)
	if err != nil {
		t.Fatalf("FixConfigFile: %v", err)
	}
	if len(fixes) != 1 || fixes[0].Before != "include" {
		t.Fatalf("fixes = %+v, want the override's include renamed", fixes)
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "include_commands:") {
		t.Errorf("override key not renamed:\n%s", out)
	}
}

// TestFixConfigFile_SkipsRenameWhenBothKeysPresent: renaming would collide with the
// authored canonical key, so leave the file alone and let lint keep warning.
func TestFixConfigFile_SkipsRenameWhenBothKeysPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pltrc")
	original := `locations:
    - location: .
      include: [old]
      include_commands: [new]
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	fixes, err := FixConfigFile(path)
	if err != nil {
		t.Fatalf("FixConfigFile: %v", err)
	}
	if len(fixes) != 0 {
		t.Errorf("fixes = %+v, want none", fixes)
	}
	out, _ := os.ReadFile(path)
	if string(out) != original {
		t.Errorf("file was rewritten:\n%s", out)
	}
}

func TestFormatLintReport_ParserKind(t *testing.T) {
	report := FormatLintReport([]config.Warning{{
		Kind:    "parser",
		Scope:   "location",
		Context: "infra",
		Name:    "gradle",
		Reason:  "gradle parser timed out; only its base commands are available",
	}})

	if !strings.Contains(report, "infra") || !strings.Contains(report, "timed out") {
		t.Errorf("report missing detail, got %q", report)
	}
	if strings.Contains(report, "@project:command aliases") {
		t.Errorf("parser report should not print name-charset guidance, got %q", report)
	}
}
