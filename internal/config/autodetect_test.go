package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// writeFiles creates each relative path under dir with the given content.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// loadAutodetectFixture writes .pltrc plus files into a fresh project (HOME
// isolated, so no user parsers.yaml or cache leaks in) and loads it resolved.
func loadAutodetectFixture(t *testing.T, pltrc string, files map[string]string) *Config {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dir := writeLoadFixture(t, pltrc)
	writeFiles(t, dir, files)
	cfg, err := Load(LoadOptions{Workdir: dir})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

const packageJSON = `{"scripts": {"dev": "vite", "lint": "eslint ."}}`

// A location without `type:` gets the types its folder's files point to.
func TestAutodetect_OmittedTypeDetectsFromFiles(t *testing.T) {
	cfg := loadAutodetectFixture(t, "locations:\n  - name: web\n    location: .\n", map[string]string{
		"package.json": packageJSON,
	})

	loc := cfg.Locations[0]
	if !reflect.DeepEqual(loc.DetectedTypes, []string{"npm"}) {
		t.Errorf("DetectedTypes = %v, want [npm]", loc.DetectedTypes)
	}
	if len(loc.Types) != 0 {
		t.Errorf("Types = %v, want the authored (empty) value untouched", loc.Types)
	}
	names := commandNamesOf(loc.Commands)
	if !slicesContain(names, "dev") || !slicesContain(names, "lint") {
		t.Errorf("commands = %v, want the package.json scripts", names)
	}
}

// Authored commands still come first, detected ones after.
func TestAutodetect_KeepsAuthoredCommands(t *testing.T) {
	cfg := loadAutodetectFixture(t, "locations:\n  - name: web\n    location: .\n    commands:\n      - name: deploy\n        command: ./deploy.sh\n", map[string]string{
		"package.json": packageJSON,
	})
	names := commandNamesOf(cfg.Locations[0].Commands)
	if len(names) == 0 || names[0] != "deploy" || !slicesContain(names, "dev") {
		t.Errorf("commands = %v, want deploy first, then detected", names)
	}
}

// `type: none` opts out: only the authored commands, whatever files are there.
func TestAutodetect_TypeNoneOptsOut(t *testing.T) {
	cfg := loadAutodetectFixture(t, "locations:\n  - name: web\n    location: .\n    type: none\n    commands:\n      - name: deploy\n        command: ./deploy.sh\n", map[string]string{
		"package.json": packageJSON,
	})
	loc := cfg.Locations[0]
	if got := commandNamesOf(loc.Commands); !reflect.DeepEqual(got, []string{"deploy"}) {
		t.Errorf("commands = %v, want only [deploy]", got)
	}
	if len(loc.DetectedTypes) != 0 {
		t.Errorf("DetectedTypes = %v, want none", loc.DetectedTypes)
	}
}

// An explicit type is the whole list: other types' files in the folder are ignored.
func TestAutodetect_ExplicitTypeIsNotExtended(t *testing.T) {
	cfg := loadAutodetectFixture(t, "locations:\n  - name: web\n    location: .\n    type: go\n", map[string]string{
		"package.json": packageJSON,
		"go.mod":       "module example.com/x\n",
	})
	for _, cmd := range cfg.Locations[0].Commands {
		if cmd.Type != "go" {
			t.Errorf("command %q from type %q, want only go", cmd.Name, cmd.Type)
		}
	}
}

// Each folder of a type-less glob detects its own types.
func TestAutodetect_GlobChildrenDetectPerFolder(t *testing.T) {
	cfg := loadAutodetectFixture(t, "locations:\n  - location: packages/*\n", map[string]string{
		"packages/web/package.json": packageJSON,
		"packages/api/go.mod":       "module example.com/api\n",
	})
	got := map[string][]string{}
	for _, loc := range cfg.Locations {
		got[filepath.Base(loc.Location)] = loc.DetectedTypes
	}
	want := map[string][]string{"web": {"npm"}, "api": {"go"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("detected = %v, want %v", got, want)
	}
}

// Shell-backed detected types defer like declared ones, so detection never
// blocks the palette opening.
func TestAutodetect_ShellTypesStayDeferred(t *testing.T) {
	configPath := makeProject(t, "\"printf 'test\\\\n'\"", "locations:\n  - name: infra\n    location: \".\"\n")
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !reflect.DeepEqual(cfg.Locations[0].PendingTypes, []string{"make"}) {
		t.Errorf("PendingTypes = %v, want [make]", cfg.Locations[0].PendingTypes)
	}
}

// Two detected types label rows with their type, like two declared ones.
func TestAutodetect_MultipleDetectedTypesMarkRowsMultiType(t *testing.T) {
	cfg := loadAutodetectFixture(t, "locations:\n  - name: app\n    location: .\n", map[string]string{
		"package.json": packageJSON,
		"go.mod":       "module example.com/x\n",
	})
	for _, row := range cfg.Rows(false) {
		if !row.IsTool && !row.MultiType {
			t.Errorf("row %q not MultiType, want it with two detected types", row.Name)
		}
	}
}

// Detected types are runtime only: they never reach a rewritten .pltrc.
func TestAutodetect_DetectedTypesAreNotSerialized(t *testing.T) {
	cfg := loadAutodetectFixture(t, "locations:\n  - name: web\n    location: .\n", map[string]string{
		"package.json": packageJSON,
	})
	loc := cfg.Locations[0]
	loc.Commands = nil
	out, err := yaml.Marshal(loc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "type") || strings.Contains(string(out), "npm") {
		t.Errorf("marshaled location = %q, want no type", out)
	}
}

func slicesContain(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
