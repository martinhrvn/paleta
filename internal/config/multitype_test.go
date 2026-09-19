package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTypes_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want Types
	}{
		{"scalar", `type: npm`, Types{"npm"}},
		{"sequence flow", `type: [npm, docker]`, Types{"npm", "docker"}},
		{"sequence block", "type:\n  - npm\n  - docker", Types{"npm", "docker"}},
		{"empty scalar", `type: ""`, nil},
		{"empty sequence", `type: []`, nil},
		{"trims and dedups", `type: [" npm ", npm, docker]`, Types{"npm", "docker"}},
		{"absent", `name: x`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var loc Location
			if err := yaml.Unmarshal([]byte(tt.yaml), &loc); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !reflect.DeepEqual(loc.Types, tt.want) {
				t.Errorf("Types = %#v, want %#v", loc.Types, tt.want)
			}
		})
	}
}

func TestTypes_MarshalYAML(t *testing.T) {
	tests := []struct {
		name  string
		types Types
		want  string
	}{
		{"single scalar", Types{"npm"}, "type: npm\n"},
		{"multiple sequence", Types{"npm", "docker"}, "type:\n    - npm\n    - docker\n"},
		{"empty omitted", nil, "{}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal a minimal struct so omitempty applies as in Location.
			v := struct {
				Types Types `yaml:"type,omitempty"`
			}{Types: tt.types}
			out, err := yaml.Marshal(v)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(out) != tt.want {
				t.Errorf("marshal = %q, want %q", string(out), tt.want)
			}
		})
	}
}

// findCmd returns the first command whose Name and Type match, or nil.
func findCmd(cmds []Command, name, typ string) *Command {
	for i := range cmds {
		if cmds[i].Name == name && cmds[i].Type == typ {
			return &cmds[i]
		}
	}
	return nil
}

func TestProcessProjectTypes_MultiType(t *testing.T) {
	dir := t.TempDir()
	// An npm package (with a colliding "build" script) that is also a Docker service.
	if err := os.WriteFile(filepath.Join(dir, "package.json"),
		[]byte(`{"scripts":{"build":"webpack","dev":"vite"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Locations: []Location{{
		Location: dir,
		Types:    Types{"npm", "docker"},
	}}}
	if _, err := processProjectTypes(cfg); err != nil {
		t.Fatalf("processProjectTypes: %v", err)
	}

	cmds := cfg.Locations[0].Commands

	// npm-tagged commands present (base + parsed scripts).
	if c := findCmd(cmds, "dev", "npm"); c == nil || c.Command != "npm run dev" {
		t.Errorf("expected npm 'dev' -> 'npm run dev', got %+v", c)
	}
	// docker-tagged commands present.
	if c := findCmd(cmds, "build", "docker"); c == nil || c.Command != "docker build ." {
		t.Errorf("expected docker 'build' -> 'docker build .', got %+v", c)
	}
	// Collision: both npm and docker contribute a "build", kept as distinct entries.
	if findCmd(cmds, "build", "npm") == nil {
		t.Error("expected an npm-tagged 'build' command")
	}
	if findCmd(cmds, "build", "docker") == nil {
		t.Error("expected a docker-tagged 'build' command")
	}
}

func TestProcessProjectTypes_ComposeGlobOnly(t *testing.T) {
	dir := t.TempDir()
	// Only an env-specific override file (matches the docker-compose.*.yml glob).
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.prod.yml"), []byte("services: {}"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Locations: []Location{{
		Location: dir,
		Types:    Types{"compose"},
	}}}
	if _, err := processProjectTypes(cfg); err != nil {
		t.Fatalf("processProjectTypes: %v", err)
	}

	// Regression: glob-only compose dirs must still yield the base commands.
	if c := findCmd(cfg.Locations[0].Commands, "up", "compose"); c == nil || c.Command != "docker compose up" {
		t.Errorf("expected compose 'up' -> 'docker compose up', got %+v", c)
	}
}

func TestProcessProjectTypes_ComposeVariantFiles(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"docker-compose.yml", "docker-compose.dev.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("services: {}"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &Config{Locations: []Location{{
		Location: dir,
		Types:    Types{"compose"},
	}}}
	if _, err := processProjectTypes(cfg); err != nil {
		t.Fatalf("processProjectTypes: %v", err)
	}

	cmds := cfg.Locations[0].Commands
	if c := findCmd(cmds, "up", "compose"); c == nil || c.Command != "docker compose up" {
		t.Errorf("expected compose 'up' -> 'docker compose up', got %+v", c)
	}
	if c := findCmd(cmds, "dev:up", "compose"); c == nil || c.Command != "docker compose -f docker-compose.dev.yaml up" {
		t.Errorf("expected compose 'dev:up' -> 'docker compose -f docker-compose.dev.yaml up', got %+v", c)
	}
	if c := findCmd(cmds, "dev:logs", "compose"); c == nil || c.Command != "docker compose -f docker-compose.dev.yaml logs -f" {
		t.Errorf("expected compose 'dev:logs' -> 'docker compose -f docker-compose.dev.yaml logs -f', got %+v", c)
	}

	// Variant names must be alias-safe so @project:dev:up can reference them.
	collectConfigWarnings(cfg)
	for _, w := range cfg.Warnings {
		if w.Kind == "name" {
			t.Errorf("unexpected name warning for %q: %s", w.Name, w.Reason)
		}
	}
}
