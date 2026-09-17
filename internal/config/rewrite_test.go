package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// openFixture writes src to a temp .pltrc and opens it for editing.
func openFixture(t *testing.T, src string) *File {
	t.Helper()
	path := filepath.Join(t.TempDir(), ConfigFileName)
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	return f
}

// render returns the file's current content and the config it parses to.
func render(t *testing.T, f *File) (string, *Config) {
	t.Helper()
	data, err := f.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	var parsed Config
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("rendered config is not valid YAML: %v\n%s", err, data)
	}
	return string(data), &parsed
}

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

func TestLoadAuthored_Missing(t *testing.T) {
	cfg, err := LoadAuthored(filepath.Join(t.TempDir(), ConfigFileName))
	if err != nil {
		t.Fatalf("expected nil error for a missing file, got %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config for a missing file, got %+v", cfg)
	}
}

// The authored form keeps relative paths, globs and the frecency block exactly
// as written: no resolution, expansion or type processing.
func TestLoadAuthored_PreservesAuthored(t *testing.T) {
	path := filepath.Join(t.TempDir(), ConfigFileName)
	src := "frecency:\n  enabled: true\n  frequency_weight: 50\n  recency_weight: 50\nlocations:\n  - name: web\n    location: packages/web\n    type: npm\n  - location: \"packages/*\"\n"
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAuthored(path)
	if err != nil {
		t.Fatalf("LoadAuthored: %v", err)
	}
	if len(cfg.Locations) != 2 || cfg.Locations[0].Location != "packages/web" || cfg.Locations[1].Location != "packages/*" {
		t.Errorf("authored locations not preserved: %+v", cfg.Locations)
	}
	if !cfg.Frecency.Enabled {
		t.Error("expected the frecency block to be preserved")
	}
}

func TestOpenFile_Errors(t *testing.T) {
	if _, err := OpenFile(filepath.Join(t.TempDir(), ConfigFileName)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: err = %v, want fs.ErrNotExist", err)
	}

	path := filepath.Join(t.TempDir(), ConfigFileName)
	if err := os.WriteFile(path, []byte("locations: [\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := OpenFile(path)
	var invalid *InvalidConfigError
	if !errors.As(err, &invalid) || invalid.Path != path {
		t.Errorf("broken file: err = %v, want *InvalidConfigError naming %s", err, path)
	}
}

// A new file carries the generated header and renders the locations it is given.
func TestNewFile_HeaderAndRoundTrip(t *testing.T) {
	f := NewFile(filepath.Join(t.TempDir(), ConfigFileName))
	if err := f.SetLocations([]Location{
		{Name: "root", Location: ".", Types: Types{"go"}},
		{Name: "web", Location: "packages/web", Types: Types{"npm"}},
	}); err != nil {
		t.Fatal(err)
	}

	out, parsed := render(t, f)
	if !strings.Contains(out, "# paleta configuration file") {
		t.Errorf("expected the generated header comment:\n%s", out)
	}
	if len(parsed.Locations) != 2 || parsed.Locations[1].Location != "packages/web" ||
		!reflect.DeepEqual(parsed.Locations[1].Types, Types{"npm"}) {
		t.Errorf("locations not round-tripped: %+v", parsed.Locations)
	}
	if !strings.Contains(out, "type: npm") {
		t.Errorf("expected a single type to stay scalar:\n%s", out)
	}
	if strings.Contains(out, "frecency:") {
		t.Errorf("did not expect a frecency block in a fresh file:\n%s", out)
	}
}

// Every serialized Location field survives a write: filters under their current
// spelling, multi-type lists, and overrides with keys in a stable order.
func TestFile_SetLocations_FieldsRoundTrip(t *testing.T) {
	f := NewFile(filepath.Join(t.TempDir(), ConfigFileName))
	if err := f.SetLocations([]Location{
		{
			Name:     "web",
			Location: "packages/web",
			Types:    Types{"npm", "docker"},
			Include:  []string{"dev"},
			Exclude:  []string{"test:watch"},
		},
		{
			Location:         "services/*",
			Types:            Types{"npm"},
			ExcludeLocations: []string{"legacy"},
			Overrides: map[string]LocationOverride{
				"zeta":  {Name: "z"},
				"alpha": {Env: map[string]string{"PORT": "3001"}},
				"mid":   {Commands: []Command{{Name: "commandx", Command: "npm run commandx"}}},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	out, parsed := render(t, f)
	if !strings.Contains(out, "include_commands:") || !strings.Contains(out, "exclude_commands:") {
		t.Errorf("expected the canonical filter keys:\n%s", out)
	}
	web := parsed.Locations[0]
	if !reflect.DeepEqual(web.Types, Types{"npm", "docker"}) || !reflect.DeepEqual(web.Include, []string{"dev"}) || !reflect.DeepEqual(web.Exclude, []string{"test:watch"}) {
		t.Errorf("web not round-tripped: %+v", web)
	}
	glob := parsed.Locations[1]
	if !reflect.DeepEqual(glob.ExcludeLocations, []string{"legacy"}) || len(glob.Overrides) != 3 ||
		glob.Overrides["zeta"].Name != "z" || glob.Overrides["alpha"].Env["PORT"] != "3001" ||
		len(glob.Overrides["mid"].Commands) != 1 {
		t.Errorf("glob location not round-tripped: %+v", glob)
	}
	alpha, mid, zeta := strings.Index(out, "alpha:"), strings.Index(out, "mid:"), strings.Index(out, "zeta:")
	if !(alpha >= 0 && alpha < mid && mid < zeta) {
		t.Errorf("override keys not emitted in sorted order:\n%s", out)
	}
}

// Rewriting the location list keeps the authored entry (and its comments) for
// every location that is still present, updates what changed, drops the ones
// left out, appends the new ones, and leaves the rest of the file alone.
func TestFile_SetLocations_KeepsAuthoredEntries(t *testing.T) {
	f := openFixture(t, `# top of file
frecency:
  enabled: true
  frequency_weight: 70
  recency_weight: 30
locations:
  # the web app
  - name: web
    location: packages/web
    type: npm
    commands:
      - name: dev # hot reload
        command: pnpm dev
  - name: api
    location: services/api
    type: go
tools:
  - lazygit
`)
	authored, err := f.Authored()
	if err != nil {
		t.Fatal(err)
	}
	web := authored.Locations[0]
	web.Types = Types{"npm", "docker"}
	svc := Location{Name: "svc", Location: "services/svc", Types: Types{"docker"}}
	if err := f.SetLocations([]Location{web, svc}); err != nil {
		t.Fatal(err)
	}

	out, parsed := render(t, f)
	for _, want := range []string{"# top of file", "# the web app", "# hot reload", "frequency_weight: 70", "- lazygit"} {
		if !strings.Contains(out, want) {
			t.Errorf("rewrite lost %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "services/api") {
		t.Errorf("dropped location still present:\n%s", out)
	}
	if len(parsed.Locations) != 2 {
		t.Fatalf("locations = %+v, want web and svc", parsed.Locations)
	}
	if !reflect.DeepEqual(parsed.Locations[0].Types, Types{"npm", "docker"}) {
		t.Errorf("web types = %v, want the updated list", parsed.Locations[0].Types)
	}
	if len(parsed.Locations[0].Commands) != 1 || parsed.Locations[0].Commands[0].Command != "pnpm dev" {
		t.Errorf("web commands = %+v, want the authored command kept", parsed.Locations[0].Commands)
	}
	if parsed.Locations[1].Name != "svc" {
		t.Errorf("second location = %+v, want svc", parsed.Locations[1])
	}
}

// A focus change touches only the top-level list.
func TestFile_SetFocused_TouchesOnlyFocusList(t *testing.T) {
	f := openFixture(t, `# keep me
root: /srv/x
locations:
  - name: a
    location: a
    commands:
      - go run . # main
  - name: b
    location: b
focused:
  - a
`)
	f.SetFocused([]string{"b"})
	out, parsed := render(t, f)
	for _, want := range []string{"# keep me", "root: /srv/x", "# main"} {
		if !strings.Contains(out, want) {
			t.Errorf("rewrite lost %q:\n%s", want, out)
		}
	}
	if !reflect.DeepEqual(parsed.Focused, []string{"b"}) {
		t.Errorf("focused = %v, want [b]", parsed.Focused)
	}

	f.SetFocused(nil)
	out, _ = render(t, f)
	if strings.Contains(out, "focused:") {
		t.Errorf("an empty focus set should remove the key:\n%s", out)
	}
}

// Appending a command creates the list when needed, writes a chain as a YAML
// list and a single command as a scalar, and keeps the neighbours' comments.
func TestFile_AppendCommand(t *testing.T) {
	f := openFixture(t, `locations:
  - name: web # frontend
    location: web
  - name: api
    location: api
    commands:
      - go run . # serve
`)
	if err := f.AppendCommand(0, NewCommand("ci", []string{"pnpm i", "pnpm test"})); err != nil {
		t.Fatal(err)
	}
	if err := f.AppendCommand(1, NewCommand("build", []string{"go build ./..."})); err != nil {
		t.Fatal(err)
	}
	if err := f.AppendCommand(5, NewCommand("x", []string{"y"})); err == nil {
		t.Error("expected an error for a location index out of range")
	}

	out, parsed := render(t, f)
	for _, want := range []string{"# frontend", "# serve", "- pnpm i", "command: go build ./..."} {
		if !strings.Contains(out, want) {
			t.Errorf("rewrite lost or misrendered %q:\n%s", want, out)
		}
	}
	if cmds := parsed.Locations[0].Commands; len(cmds) != 1 || cmds[0].Name != "ci" || cmds[0].Command != "pnpm i && pnpm test" {
		t.Errorf("web commands = %+v", cmds)
	}
	if cmds := parsed.Locations[1].Commands; len(cmds) != 2 || cmds[0].Command != "go run ." || cmds[1].Command != "go build ./..." {
		t.Errorf("api commands = %+v", cmds)
	}
}

func TestFile_AddLocation(t *testing.T) {
	f := openFixture(t, "locations:\n  - name: a # first\n    location: a\n")
	if err := f.AddLocation(Location{Name: "b", Location: "b", Commands: []Command{NewCommand("run", []string{"./b"})}}); err != nil {
		t.Fatal(err)
	}
	out, parsed := render(t, f)
	if !strings.Contains(out, "# first") {
		t.Errorf("comment lost:\n%s", out)
	}
	if len(parsed.Locations) != 2 || parsed.Locations[1].Name != "b" || len(parsed.Locations[1].Commands) != 1 {
		t.Errorf("locations = %+v", parsed.Locations)
	}
}

// Any save emits the current key spellings, comments intact, so a file authored
// with `include:` is upgraded by the first rewrite that touches it.
func TestFile_SaveUpgradesDeprecatedKeys(t *testing.T) {
	f := openFixture(t, `locations:
  - name: web
    location: packages/web
    type: npm
    # only the build scripts
    include: # whitelist
      - build*
`)
	f.SetFocused([]string{"web"})
	out, parsed := render(t, f)
	if !strings.Contains(out, "include_commands:") || strings.Contains(out, "\n    include:") {
		t.Errorf("deprecated key not upgraded:\n%s", out)
	}
	for _, want := range []string{"# only the build scripts", "# whitelist"} {
		if !strings.Contains(out, want) {
			t.Errorf("rewrite lost %q:\n%s", want, out)
		}
	}
	if !reflect.DeepEqual(parsed.Locations[0].Include, []string{"build*"}) {
		t.Errorf("include filter lost: %+v", parsed.Locations[0])
	}
}

// Fix reports each repair and leaves a clean file untouched.
func TestFile_Fix(t *testing.T) {
	f := openFixture(t, `locations:
  - name: my proj
    location: .
    include: [build*]
    commands:
      - name: test ui
        command: go test ./...
`)
	fixes := f.Fix()
	if len(fixes) != 3 {
		t.Fatalf("fixes = %+v, want the two names and the key rename", fixes)
	}
	out, parsed := render(t, f)
	if strings.Contains(out, "my proj") || strings.Contains(out, "test ui") {
		t.Errorf("names not sanitized:\n%s", out)
	}
	if parsed.Locations[0].Name != "my_proj" || parsed.Locations[0].Commands[0].Name != "test_ui" {
		t.Errorf("sanitized names not written: %+v", parsed.Locations[0])
	}

	clean := openFixture(t, "locations:\n  - name: root\n    location: .\n")
	if fixes := clean.Fix(); len(fixes) != 0 {
		t.Errorf("expected no fixes for a clean file, got %+v", fixes)
	}
}

func TestFile_Save_PreservesMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), ConfigFileName)
	if err := os.WriteFile(path, []byte("locations:\n  - location: .\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f.SetFocused([]string{"."})
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode = %v, want the original 0600", info.Mode().Perm())
	}
	cfg, err := LoadAuthored(path)
	if err != nil || !reflect.DeepEqual(cfg.Focused, []string{"."}) {
		t.Errorf("saved focus not read back: %+v, %v", cfg, err)
	}
}
