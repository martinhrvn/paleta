package config

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestLocationUnmarshal_AllFieldsRoundTrip guards against the aux-struct decoder
// silently dropping a field: every authored Location field must survive a decode.
// (The same class of bug lost Location.Env in glob expansion for a long time.)
func TestLocationUnmarshal_AllFieldsRoundTrip(t *testing.T) {
	const src = `
name: services
location: "services/*"
type: [npm, docker]
commands:
  - name: foo
    command: "npm run foo"
include_commands: ["build*"]
exclude_commands: ["*:watch"]
exclude_locations: ["legacy"]
overrides:
  frontend-next:
    commands:
      - name: commandx
        command: "npm run commandx"
    env:
      PORT: "3001"
env:
  LOG: info
`

	var loc Location
	if err := yaml.Unmarshal([]byte(src), &loc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if loc.Name != "services" {
		t.Errorf("Name = %q, want %q", loc.Name, "services")
	}
	if loc.Location != "services/*" {
		t.Errorf("Location = %q, want %q", loc.Location, "services/*")
	}
	if !reflect.DeepEqual(loc.Types, Types{"npm", "docker"}) {
		t.Errorf("Types = %v, want [npm docker]", loc.Types)
	}
	if len(loc.Commands) != 1 || loc.Commands[0].Name != "foo" || loc.Commands[0].Command != "npm run foo" {
		t.Errorf("Commands = %+v, want one foo command", loc.Commands)
	}
	if !reflect.DeepEqual(loc.Include, []string{"build*"}) {
		t.Errorf("Include = %v, want [build*]", loc.Include)
	}
	if !reflect.DeepEqual(loc.Exclude, []string{"*:watch"}) {
		t.Errorf("Exclude = %v, want [*:watch]", loc.Exclude)
	}
	if !reflect.DeepEqual(loc.ExcludeLocations, []string{"legacy"}) {
		t.Errorf("ExcludeLocations = %v, want [legacy]", loc.ExcludeLocations)
	}
	if !reflect.DeepEqual(loc.Env, map[string]string{"LOG": "info"}) {
		t.Errorf("Env = %v, want map[LOG:info]", loc.Env)
	}
	ov, ok := loc.Overrides["frontend-next"]
	if !ok {
		t.Fatalf("Overrides missing key frontend-next, got %v", loc.Overrides)
	}
	if len(ov.Commands) != 1 || ov.Commands[0].Name != "commandx" {
		t.Errorf("override Commands = %+v, want one commandx command", ov.Commands)
	}
	if !reflect.DeepEqual(ov.Env, map[string]string{"PORT": "3001"}) {
		t.Errorf("override Env = %v, want map[PORT:3001]", ov.Env)
	}
	if len(loc.LegacyKeys) != 0 {
		t.Errorf("LegacyKeys = %v, want none for a config using the new spelling", loc.LegacyKeys)
	}
}

func TestLocationUnmarshal_LegacyIncludeExcludeCanonicalized(t *testing.T) {
	const src = `
location: "packages/web"
type: npm
include: ["build*"]
exclude: ["*:watch"]
`

	var loc Location
	if err := yaml.Unmarshal([]byte(src), &loc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(loc.Include, []string{"build*"}) {
		t.Errorf("Include = %v, want [build*] from the legacy key", loc.Include)
	}
	if !reflect.DeepEqual(loc.Exclude, []string{"*:watch"}) {
		t.Errorf("Exclude = %v, want [*:watch] from the legacy key", loc.Exclude)
	}
	if !reflect.DeepEqual(loc.LegacyKeys, []string{"include", "exclude"}) {
		t.Errorf("LegacyKeys = %v, want [include exclude]", loc.LegacyKeys)
	}
}

func TestLocationUnmarshal_NewKeysPreferredOverLegacy(t *testing.T) {
	const src = `
location: "packages/web"
include: ["legacy-include"]
include_commands: ["new-include"]
exclude: ["legacy-exclude"]
exclude_commands: ["new-exclude"]
`

	var loc Location
	if err := yaml.Unmarshal([]byte(src), &loc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(loc.Include, []string{"new-include"}) {
		t.Errorf("Include = %v, want the new key to win", loc.Include)
	}
	if !reflect.DeepEqual(loc.Exclude, []string{"new-exclude"}) {
		t.Errorf("Exclude = %v, want the new key to win", loc.Exclude)
	}
	if !reflect.DeepEqual(loc.LegacyKeys, []string{"include", "exclude"}) {
		t.Errorf("LegacyKeys = %v, want the ignored legacy keys still reported", loc.LegacyKeys)
	}
}

func TestLocationOverrideUnmarshal_LegacyKeys(t *testing.T) {
	const src = `
location: "services/*"
overrides:
  api:
    include: ["build*"]
  worker:
    exclude: ["*:watch"]
`

	var loc Location
	if err := yaml.Unmarshal([]byte(src), &loc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got := loc.Overrides["api"].Include; !reflect.DeepEqual(got, []string{"build*"}) {
		t.Errorf("overrides.api include = %v, want [build*]", got)
	}
	if got := loc.Overrides["worker"].Exclude; !reflect.DeepEqual(got, []string{"*:watch"}) {
		t.Errorf("overrides.worker exclude = %v, want [*:watch]", got)
	}
	want := []string{"overrides.api.include", "overrides.worker.exclude"}
	if !reflect.DeepEqual(loc.LegacyKeys, want) {
		t.Errorf("LegacyKeys = %v, want %v (sorted by folder key)", loc.LegacyKeys, want)
	}
}
