package config

import "testing"

// findWarning returns the first warning matching scope and name, or nil.
func findWarning(ws []Warning, scope, name string) *Warning {
	for i := range ws {
		if ws[i].Scope == scope && ws[i].Name == name {
			return &ws[i]
		}
	}
	return nil
}

func TestValidateNames_ValidNamesProduceNoWarnings(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name:     "root",
		Location: "/abs/root",
		Commands: []Command{
			{Name: "build", Command: "go build ./..."},
			{Name: "test:watch", Command: "npm run test:watch"},
		},
	}}}

	collectConfigWarnings(cfg)

	if len(cfg.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %d: %+v", len(cfg.Warnings), cfg.Warnings)
	}
	if cfg.Locations[0].NameError != "" {
		t.Errorf("location NameError = %q, want empty", cfg.Locations[0].NameError)
	}
	for i, c := range cfg.Locations[0].Commands {
		if c.NameError != "" {
			t.Errorf("command %d NameError = %q, want empty", i, c.NameError)
		}
	}
}

func TestValidateNames_CommandNameWithSpace(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name:     "root",
		Location: "/abs/root",
		Commands: []Command{
			{Name: "test ui", Command: "go test ./ui/..."},
			{Name: "build", Command: "go build ./..."},
		},
	}}}

	collectConfigWarnings(cfg)

	if w := findWarning(cfg.Warnings, "command", "test ui"); w == nil {
		t.Fatalf("expected a command warning for %q, got %+v", "test ui", cfg.Warnings)
	}
	if cfg.Locations[0].Commands[0].NameError == "" {
		t.Errorf("expected NameError set on 'test ui' command")
	}
	// The valid sibling command and the location itself are untouched.
	if cfg.Locations[0].Commands[1].NameError != "" {
		t.Errorf("valid command 'build' unexpectedly flagged: %q", cfg.Locations[0].Commands[1].NameError)
	}
	if cfg.Locations[0].NameError != "" {
		t.Errorf("location unexpectedly flagged: %q", cfg.Locations[0].NameError)
	}
}

func TestValidateNames_OutOfCharsetNames(t *testing.T) {
	cases := []string{"build*", "a&b"}
	for _, name := range cases {
		cfg := &Config{Locations: []Location{{
			Name:     "root",
			Location: "/abs/root",
			Commands: []Command{{Name: name, Command: "echo hi"}},
		}}}
		collectConfigWarnings(cfg)
		if w := findWarning(cfg.Warnings, "command", name); w == nil {
			t.Errorf("expected warning for command name %q, got %+v", name, cfg.Warnings)
		}
	}
}

func TestValidateNames_LocationNameWithSpace(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name:     "my proj",
		Location: "/abs/my-proj",
		Commands: []Command{{Name: "build", Command: "go build ./..."}},
	}}}

	collectConfigWarnings(cfg)

	if w := findWarning(cfg.Warnings, "location", "my proj"); w == nil {
		t.Fatalf("expected a location warning for %q, got %+v", "my proj", cfg.Warnings)
	}
	if cfg.Locations[0].NameError == "" {
		t.Errorf("expected NameError set on location 'my proj'")
	}
}

// A glob in the location *path* is legitimate and must not be flagged: only the
// authored name is validated, not the path.
func TestValidateNames_GlobPathNotFlagged(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name:     "pkg",
		Location: "packages/*",
		Commands: []Command{{Name: "build", Command: "go build ./..."}},
	}}}

	collectConfigWarnings(cfg)

	if len(cfg.Warnings) != 0 {
		t.Fatalf("glob path should not be flagged, got warnings: %+v", cfg.Warnings)
	}
}

// An unresolved-alias error recorded on a command (by expandCommandAliases)
// surfaces as a Kind "alias" warning.
func TestCollectWarnings_UnresolvedAlias(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name:     "root",
		Location: "/abs/root",
		Commands: []Command{
			{Name: "ci", Command: "@root:missing", Error: `command "ci" in project "root": unknown command "missing"`},
			{Name: "build", Command: "go build ./..."},
		},
	}}}

	collectConfigWarnings(cfg)

	var alias *Warning
	for i := range cfg.Warnings {
		if cfg.Warnings[i].Kind == "alias" {
			alias = &cfg.Warnings[i]
		}
	}
	if alias == nil {
		t.Fatalf("expected an alias warning, got %+v", cfg.Warnings)
	}
	if alias.Scope != "command" || alias.Reason == "" {
		t.Errorf("alias warning malformed: %+v", *alias)
	}
}

func TestValidateNames_EmptyNamesSkipped(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name:     "", // location displays by path; nothing to validate
		Location: "/abs/root",
		Commands: []Command{{Name: "", Command: "go build ./..."}},
	}}}

	collectConfigWarnings(cfg)

	if len(cfg.Warnings) != 0 {
		t.Fatalf("empty names should be skipped, got warnings: %+v", cfg.Warnings)
	}
}
