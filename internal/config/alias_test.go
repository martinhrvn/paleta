package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cmd(pos) accessor for readability.
func expandedCmd(t *testing.T, cfg *Config, loc, idx int) string {
	t.Helper()
	return cfg.Locations[loc].Commands[idx].Command
}

func TestExpandAliases_SameProjectStaysBare(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name: "web", Location: "/abs/web", Types: Types{"pnpm"},
		Commands: []Command{
			{Name: "build", Command: "pnpm run build", Type: "pnpm"},
			{Name: "dev", Command: "pnpm run dev", Type: "pnpm"},
			{Name: "ci", Command: "@web:build && @web:dev"},
		},
	}}}

	if err := expandCommandAliases(cfg); err != nil {
		t.Fatalf("expandCommandAliases: %v", err)
	}
	if got, want := expandedCmd(t, cfg, 0, 2), "pnpm run build && pnpm run dev"; got != want {
		t.Errorf("ci expanded to %q, want %q", got, want)
	}
	// The referenced commands themselves are untouched.
	if got := expandedCmd(t, cfg, 0, 0); got != "pnpm run build" {
		t.Errorf("build command mutated: %q", got)
	}
}

func TestExpandAliases_TypeDisambiguation(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name: "svc", Location: "/abs/svc", Types: Types{"npm", "docker"},
		Commands: []Command{
			{Name: "build", Command: "npm run build", Type: "npm"},
			{Name: "build", Command: "docker build .", Type: "docker"},
			{Name: "image", Command: "@svc[docker]:build"},
		},
	}}}

	if err := expandCommandAliases(cfg); err != nil {
		t.Fatalf("expandCommandAliases: %v", err)
	}
	if got, want := expandedCmd(t, cfg, 0, 2), "docker build ."; got != want {
		t.Errorf("image expanded to %q, want %q", got, want)
	}
}

func TestExpandAliases_AmbiguousWithoutTypeErrors(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name: "svc", Location: "/abs/svc", Types: Types{"npm", "docker"},
		Commands: []Command{
			{Name: "build", Command: "npm run build", Type: "npm"},
			{Name: "build", Command: "docker build .", Type: "docker"},
			{Name: "x", Command: "@svc:build"},
		},
	}}}

	err := expandCommandAliases(cfg)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected an ambiguity error, got %v", err)
	}
}

func TestExpandAliases_CrossProjectWrapsInSubshell(t *testing.T) {
	cfg := &Config{Locations: []Location{
		{Name: "web", Location: "/abs/web", Types: Types{"pnpm"}, Commands: []Command{
			{Name: "build", Command: "pnpm run build", Type: "pnpm"},
		}},
		{Name: "api", Location: "/abs/api", Types: Types{"go"}, Commands: []Command{
			{Name: "chain", Command: "@web:build && go test ./..."},
		}},
	}}

	if err := expandCommandAliases(cfg); err != nil {
		t.Fatalf("expandCommandAliases: %v", err)
	}
	got := expandedCmd(t, cfg, 1, 0)
	want := "(cd '/abs/web' && pnpm run build) && go test ./..."
	if got != want {
		t.Errorf("chain expanded to %q, want %q", got, want)
	}
}

func TestExpandAliases_TransitiveAcrossProjects(t *testing.T) {
	cfg := &Config{Locations: []Location{
		{Name: "web", Location: "/abs/web", Types: Types{"pnpm"}, Commands: []Command{
			{Name: "build", Command: "pnpm run build", Type: "pnpm"},
			{Name: "meta", Command: "@web:build"}, // same-project reference
		}},
		{Name: "api", Location: "/abs/api", Commands: []Command{
			{Name: "deploy", Command: "@web:meta"},
		}},
	}}

	if err := expandCommandAliases(cfg); err != nil {
		t.Fatalf("expandCommandAliases: %v", err)
	}
	// api/deploy -> web/meta -> web/build; the inner same-project ref stays bare,
	// and the whole thing is wrapped once for the api->web hop.
	if got, want := expandedCmd(t, cfg, 1, 0), "(cd '/abs/web' && pnpm run build)"; got != want {
		t.Errorf("deploy expanded to %q, want %q", got, want)
	}
}

func TestExpandAliases_CycleErrors(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name: "web", Location: "/abs/web",
		Commands: []Command{
			{Name: "a", Command: "@web:b"},
			{Name: "b", Command: "@web:a"},
		},
	}}}

	err := expandCommandAliases(cfg)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected a cycle error, got %v", err)
	}
}

func TestExpandAliases_UnknownProjectAndCommand(t *testing.T) {
	base := func(command string) *Config {
		return &Config{Locations: []Location{{
			Name: "web", Location: "/abs/web", Types: Types{"pnpm"},
			Commands: []Command{
				{Name: "build", Command: "pnpm run build", Type: "pnpm"},
				{Name: "x", Command: command},
			},
		}}}
	}

	if err := expandCommandAliases(base("@nope:build")); err == nil || !strings.Contains(err.Error(), "unknown project") {
		t.Errorf("expected unknown-project error, got %v", err)
	}
	if err := expandCommandAliases(base("@web:nope")); err == nil || !strings.Contains(err.Error(), "no command") {
		t.Errorf("expected unknown-command error, got %v", err)
	}
}

func TestExpandAliases_LeavesNonTokensIntact(t *testing.T) {
	inputs := []string{
		"git clone git@github.com:user/repo.git",
		"npm i @scope/pkg",
		"deploy@v2 up",
	}
	for _, in := range inputs {
		cfg := &Config{Locations: []Location{{
			Name: "web", Location: "/abs/web",
			Commands: []Command{{Name: "x", Command: in}},
		}}}
		if err := expandCommandAliases(cfg); err != nil {
			t.Fatalf("expandCommandAliases(%q): %v", in, err)
		}
		if got := expandedCmd(t, cfg, 0, 0); got != in {
			t.Errorf("expected %q unchanged, got %q", in, got)
		}
	}
}

func TestExpandAliases_TokenAlongsideNonToken(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name: "web", Location: "/abs/web", Types: Types{"pnpm"},
		Commands: []Command{
			{Name: "build", Command: "pnpm run build", Type: "pnpm"},
			{Name: "x", Command: "npm i @scope/pkg && @web:build"},
		},
	}}}

	if err := expandCommandAliases(cfg); err != nil {
		t.Fatalf("expandCommandAliases: %v", err)
	}
	if got, want := expandedCmd(t, cfg, 0, 1), "npm i @scope/pkg && pnpm run build"; got != want {
		t.Errorf("expanded to %q, want %q", got, want)
	}
}

func TestExpandAliases_GlobUniqueFolderNamesResolve(t *testing.T) {
	// This mirrors how `location: "packages/*"` (no explicit name) expands: each
	// concrete location gets a unique name = its folder base name.
	cfg := &Config{Locations: []Location{
		{Name: "web", Location: "/abs/packages/web", Types: Types{"pnpm"}, Commands: []Command{
			{Name: "build", Command: "pnpm run build", Type: "pnpm"},
		}},
		{Name: "api", Location: "/abs/packages/api", Types: Types{"pnpm"}, Commands: []Command{
			{Name: "build", Command: "pnpm run build", Type: "pnpm"},
			{Name: "ci", Command: "@web:build && @api:build"},
		}},
	}}

	if err := expandCommandAliases(cfg); err != nil {
		t.Fatalf("expandCommandAliases: %v", err)
	}
	got := expandedCmd(t, cfg, 1, 1)
	want := "(cd '/abs/packages/web' && pnpm run build) && pnpm run build"
	if got != want {
		t.Errorf("ci expanded to %q, want %q", got, want)
	}
}

func TestExpandAliases_BasenameFallbackForUnnamedLocation(t *testing.T) {
	// A location with no name is still referenceable by its folder base name.
	cfg := &Config{Locations: []Location{
		{Location: "/abs/tools", Commands: []Command{
			{Name: "build", Command: "make build"},
		}},
		{Name: "api", Location: "/abs/api", Commands: []Command{
			{Name: "x", Command: "@tools:build"},
		}},
	}}

	if err := expandCommandAliases(cfg); err != nil {
		t.Fatalf("expandCommandAliases: %v", err)
	}
	if got, want := expandedCmd(t, cfg, 1, 0), "(cd '/abs/tools' && make build)"; got != want {
		t.Errorf("expanded to %q, want %q", got, want)
	}
}

func TestExpandAliases_SharedProjectNameIsAmbiguous(t *testing.T) {
	// What a glob given an explicit shared name produces: several locations, same
	// name. A reference to that name must fail loudly, not silently pick one.
	cfg := &Config{Locations: []Location{
		{Name: "pkg", Location: "/abs/packages/web", Commands: []Command{
			{Name: "build", Command: "pnpm run build"},
		}},
		{Name: "pkg", Location: "/abs/packages/api", Commands: []Command{
			{Name: "build", Command: "pnpm run build"},
		}},
		{Name: "root", Location: "/abs", Commands: []Command{
			{Name: "x", Command: "@pkg:build"},
		}},
	}}

	err := expandCommandAliases(cfg)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected an ambiguous-project error, got %v", err)
	}
}

func TestExpandAliases_PathTailDisambiguatesClash(t *testing.T) {
	// packages/search and services/search both reduce to the name "search".
	cfg := &Config{Locations: []Location{
		{Name: "search", Location: "/abs/packages/search", Types: Types{"pnpm"}, Commands: []Command{
			{Name: "build", Command: "pnpm run build", Type: "pnpm"},
		}},
		{Name: "search", Location: "/abs/services/search", Types: Types{"go"}, Commands: []Command{
			{Name: "build", Command: "go build ./...", Type: "go"},
		}},
		{Name: "root", Location: "/abs", Commands: []Command{
			{Name: "both", Command: "@packages/search:build && @services/search:build"},
		}},
	}}

	if err := expandCommandAliases(cfg); err != nil {
		t.Fatalf("expandCommandAliases: %v", err)
	}
	got := expandedCmd(t, cfg, 2, 0)
	want := "(cd '/abs/packages/search' && pnpm run build) && (cd '/abs/services/search' && go build ./...)"
	if got != want {
		t.Errorf("both expanded to %q, want %q", got, want)
	}
}

func TestExpandAliases_BareClashStillAmbiguous(t *testing.T) {
	cfg := &Config{Locations: []Location{
		{Name: "search", Location: "/abs/packages/search", Commands: []Command{
			{Name: "build", Command: "pnpm run build"},
		}},
		{Name: "search", Location: "/abs/services/search", Commands: []Command{
			{Name: "build", Command: "go build ./..."},
		}},
		{Name: "root", Location: "/abs", Commands: []Command{
			{Name: "x", Command: "@search:build"},
		}},
	}}

	err := expandCommandAliases(cfg)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected an ambiguity error for the bare name, got %v", err)
	}
}

func TestLoadConfig_ExpandsReferences_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pltrc")
	content := `locations:
  - name: tools
    commands:
      - name: build
        command: echo build
      - name: ci
        command: "@tools:build && echo done"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	var ci string
	for _, c := range cfg.Locations[0].Commands {
		if c.Name == "ci" {
			ci = c.Command
		}
	}
	if ci != "echo build && echo done" {
		t.Errorf("ci expanded to %q, want %q", ci, "echo build && echo done")
	}
}

func TestLoadConfig_BadReferenceIsNonFatal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pltrc")
	content := `locations:
  - name: tools
    commands:
      - name: ci
        command: "@tools:buld"
      - name: ok
        command: "echo ok"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// A single unresolvable reference must not fail the whole load: the config
	// loads, the bad command carries an Error (and keeps its authored text), and
	// the other commands are unaffected.
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	var ci, ok Command
	for _, c := range cfg.Locations[0].Commands {
		switch c.Name {
		case "ci":
			ci = c
		case "ok":
			ok = c
		}
	}
	if ci.Error == "" {
		t.Error("expected the bad reference to record Command.Error")
	}
	if ci.Command != "@tools:buld" {
		t.Errorf("bad command mutated: %q", ci.Command)
	}
	if ok.Error != "" || ok.Command != "echo ok" {
		t.Errorf("unrelated command affected: cmd=%q err=%q", ok.Command, ok.Error)
	}
}

func TestReferenceToken_BareWhenUnique(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name: "web", Location: "/abs/web", Types: Types{"pnpm"},
		Commands: []Command{{Name: "build", Command: "pnpm run build", Type: "pnpm"}},
	}}}
	if tok, ok := cfg.ReferenceToken("/abs/web", "build", "pnpm"); !ok || tok != "@web:build" {
		t.Errorf("ReferenceToken = %q ok=%v, want @web:build", tok, ok)
	}
}

func TestReferenceToken_TypeWhenAmbiguous(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name: "svc", Location: "/abs/svc", Types: Types{"npm", "docker"},
		Commands: []Command{
			{Name: "build", Command: "npm run build", Type: "npm"},
			{Name: "build", Command: "docker build .", Type: "docker"},
		},
	}}}
	if tok, ok := cfg.ReferenceToken("/abs/svc", "build", "docker"); !ok || tok != "@svc[docker]:build" {
		t.Errorf("ReferenceToken = %q ok=%v, want @svc[docker]:build", tok, ok)
	}
}

func TestReferenceToken_UntypedClashUnreferenceable(t *testing.T) {
	// Mirrors the local .pltrc: a manual (untyped) fmt alongside a go-typed fmt.
	cfg := &Config{Locations: []Location{{
		Name: "root", Location: "/repo", Types: Types{"go"},
		Commands: []Command{
			{Name: "fmt", Command: "go fmt ./...", Type: ""}, // manual
			{Name: "fmt", Command: "gofmt -w .", Type: "go"}, // auto
		},
	}}}
	// The typed one is referenceable with [go]...
	if tok, ok := cfg.ReferenceToken("/repo", "fmt", "go"); !ok || tok != "@root[go]:fmt" {
		t.Errorf("typed fmt token = %q ok=%v, want @root[go]:fmt", tok, ok)
	}
	// ...but the untyped manual one cannot be named uniquely -> raw fallback.
	if tok, ok := cfg.ReferenceToken("/repo", "fmt", ""); ok {
		t.Errorf("expected untyped fmt to be unreferenceable, got %q", tok)
	}
}

func TestReferenceToken_NameWithSpaceUnreferenceable(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name: "root", Location: "/repo",
		Commands: []Command{{Name: "test ui", Command: "go test ./internal/ui/..."}},
	}}}
	if tok, ok := cfg.ReferenceToken("/repo", "test ui", ""); ok {
		t.Errorf("expected a spaced name to be unreferenceable, got %q", tok)
	}
}

func TestReferenceToken_UnnamedUnreferenceable(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name: "root", Location: "/repo",
		Commands: []Command{{Name: "", Command: "make thing"}},
	}}}
	if tok, ok := cfg.ReferenceToken("/repo", "", ""); ok {
		t.Errorf("expected an unnamed command to be unreferenceable, got %q", tok)
	}
}

func TestReferenceToken_ByFolderBaseWhenNoAuthoredName(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Location: "/abs/tools", Types: Types{"npm"}, // no Name
		Commands: []Command{{Name: "build", Command: "npm run build", Type: "npm"}},
	}}}
	if tok, ok := cfg.ReferenceToken("/abs/tools", "build", "npm"); !ok || tok != "@tools:build" {
		t.Errorf("ReferenceToken = %q ok=%v, want @tools:build", tok, ok)
	}
}

func TestReferenceToken_PathTailWhenBaseClashes(t *testing.T) {
	cfg := &Config{Locations: []Location{
		{Location: "/abs/packages/search", Types: Types{"npm"},
			Commands: []Command{{Name: "build", Command: "npm run build", Type: "npm"}}},
		{Location: "/abs/services/search", Types: Types{"npm"},
			Commands: []Command{{Name: "build", Command: "npm run build", Type: "npm"}}},
	}}
	// Base "search" is ambiguous, so the token must use a path tail.
	if tok, ok := cfg.ReferenceToken("/abs/packages/search", "build", "npm"); !ok || tok != "@packages/search:build" {
		t.Errorf("ReferenceToken = %q ok=%v, want @packages/search:build", tok, ok)
	}
}

func TestExpandAliases_NonFatalRecordsError(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name: "web", Location: "/abs/web", Types: Types{"pnpm"},
		Commands: []Command{
			{Name: "build", Command: "pnpm run build", Type: "pnpm"},
			{Name: "ci", Command: "@web:build"},
			{Name: "broken", Command: "@web:nope"},
		},
	}}}

	// A returned error is fine for strict callers, but the good command must
	// still expand and the bad one must carry an error without blocking it.
	_ = expandCommandAliases(cfg)
	if got := cfg.Locations[0].Commands[1].Command; got != "pnpm run build" {
		t.Errorf("ci = %q, want expanded", got)
	}
	if cfg.Locations[0].Commands[1].Error != "" {
		t.Errorf("ci unexpectedly errored: %q", cfg.Locations[0].Commands[1].Error)
	}
	if got := cfg.Locations[0].Commands[2].Command; got != "@web:nope" {
		t.Errorf("broken command mutated: %q", got)
	}
	if cfg.Locations[0].Commands[2].Error == "" {
		t.Error("expected broken command to record an error")
	}
}
