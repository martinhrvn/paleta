package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// inGlobDir creates dirs under a fresh temp directory and chdirs into it for the
// duration of the test, so relative glob patterns resolve there.
func inGlobDir(t *testing.T, dirs ...string) {
	t.Helper()
	tmpDir := t.TempDir()
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	t.Cleanup(func() { os.Chdir(oldWd) })
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
}

// locationNames returns each location's name, for concise assertions.
func locationNames(locs []Location) []string {
	names := make([]string, len(locs))
	for i, l := range locs {
		names[i] = l.Name
	}
	return names
}

// commandNames returns each command's name (falling back to its command string).
func commandNames(cmds []Command) []string {
	names := make([]string, len(cmds))
	for i, c := range cmds {
		names[i] = c.Name
		if names[i] == "" {
			names[i] = c.Command
		}
	}
	return names
}

// warningNames returns each warning's Name field, for concise assertions.
func warningNames(ws []Warning) []string {
	names := make([]string, len(ws))
	for i, w := range ws {
		names[i] = w.Name
	}
	return names
}

func TestExpandGlobPatterns_PreservesEnv(t *testing.T) {
	inGlobDir(t, "services/api", "services/web")

	locs, _, err := ExpandGlobPatterns([]Location{{
		Location: "services/*",
		Env:      map[string]string{"LOG": "info"},
	}})
	if err != nil {
		t.Fatalf("ExpandGlobPatterns() error = %v", err)
	}
	if len(locs) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(locs))
	}
	for _, loc := range locs {
		if !reflect.DeepEqual(loc.Env, map[string]string{"LOG": "info"}) {
			t.Errorf("%s Env = %v, want the parent's env", loc.Name, loc.Env)
		}
	}
}

func TestExpandGlobPatterns_EnvNotSharedAcrossChildren(t *testing.T) {
	inGlobDir(t, "services/api", "services/web")

	locs, _, err := ExpandGlobPatterns([]Location{{
		Location: "services/*",
		Env:      map[string]string{"LOG": "info"},
	}})
	if err != nil {
		t.Fatalf("ExpandGlobPatterns() error = %v", err)
	}

	locs[0].Env["LOG"] = "debug"
	if locs[1].Env["LOG"] != "info" {
		t.Errorf("sibling Env was mutated: %v — each child needs its own map", locs[1].Env)
	}
}

func TestExpandGlobPatterns_ExcludeLocations(t *testing.T) {
	tests := []struct {
		name             string
		dirs             []string
		excludeLocations []string
		wantNames        []string
	}{
		{
			name:             "exact folder name",
			dirs:             []string{"services/api", "services/legacy", "services/web"},
			excludeLocations: []string{"legacy"},
			wantNames:        []string{"api", "web"},
		},
		{
			name:             "glob pattern",
			dirs:             []string{"services/api", "services/old-deprecated", "services/web-deprecated"},
			excludeLocations: []string{"*-deprecated"},
			wantNames:        []string{"api"},
		},
		{
			name:             "several patterns",
			dirs:             []string{"services/api", "services/legacy", "services/tmp-x"},
			excludeLocations: []string{"legacy", "tmp-*"},
			wantNames:        []string{"api"},
		},
		{
			name:             "pattern matching nothing is silent",
			dirs:             []string{"services/api"},
			excludeLocations: []string{"not-here"},
			wantNames:        []string{"api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inGlobDir(t, tt.dirs...)

			locs, warnings, err := ExpandGlobPatterns([]Location{{
				Location:         "services/*",
				ExcludeLocations: tt.excludeLocations,
			}})
			if err != nil {
				t.Fatalf("ExpandGlobPatterns() error = %v", err)
			}
			if got := locationNames(locs); !reflect.DeepEqual(got, tt.wantNames) {
				t.Errorf("locations = %v, want %v", got, tt.wantNames)
			}
			if len(warnings) != 0 {
				t.Errorf("expected no warnings, got %+v", warnings)
			}
		})
	}
}

func TestExpandGlobPatterns_ExcludeLocationsOnNonGlobLocation(t *testing.T) {
	inGlobDir(t, "services/api")

	locs, warnings, err := ExpandGlobPatterns([]Location{{
		Name:             "api",
		Location:         "services/api",
		ExcludeLocations: []string{"legacy"},
	}})
	if err != nil {
		t.Fatalf("ExpandGlobPatterns() error = %v", err)
	}
	if len(locs) != 1 || locs[0].Location != "services/api" {
		t.Fatalf("location should pass through untouched, got %+v", locs)
	}
	if len(warnings) != 1 || warnings[0].Kind != "ignored" || warnings[0].Name != "exclude_locations" {
		t.Fatalf("want one 'ignored' warning for exclude_locations, got %+v", warnings)
	}
}

func TestExpandGlobPatterns_OverridesOnNonGlobLocation(t *testing.T) {
	inGlobDir(t, "services/api")

	_, warnings, err := ExpandGlobPatterns([]Location{{
		Location:  "services/api",
		Overrides: map[string]LocationOverride{"api": {Name: "renamed"}},
	}})
	if err != nil {
		t.Fatalf("ExpandGlobPatterns() error = %v", err)
	}
	if len(warnings) != 1 || warnings[0].Kind != "ignored" || warnings[0].Name != "overrides" {
		t.Fatalf("want one 'ignored' warning for overrides, got %+v", warnings)
	}
}

func TestExpandGlobPatterns_OverrideMergesPerFolder(t *testing.T) {
	parent := Location{
		Location: "services/*",
		Types:    Types{"npm"},
		Commands: []Command{
			{Name: "foo", Command: "npm run foo"},
			{Name: "bar", Command: "npm run bar"},
		},
		Include: []string{"foo*"},
		Exclude: []string{"*:watch"},
		Env:     map[string]string{"LOG": "info"},
	}

	t.Run("commands append", func(t *testing.T) {
		inGlobDir(t, "services/api", "services/frontend-next")
		loc := parent
		loc.Overrides = map[string]LocationOverride{
			"frontend-next": {Commands: []Command{{Name: "commandx", Command: "npm run commandx"}}},
		}

		locs, warnings, err := ExpandGlobPatterns([]Location{loc})
		if err != nil {
			t.Fatalf("ExpandGlobPatterns() error = %v", err)
		}
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got %+v", warnings)
		}
		if got := commandNames(locs[0].Commands); !reflect.DeepEqual(got, []string{"foo", "bar"}) {
			t.Errorf("api commands = %v, want the inherited [foo bar]", got)
		}
		if got := commandNames(locs[1].Commands); !reflect.DeepEqual(got, []string{"foo", "bar", "commandx"}) {
			t.Errorf("frontend-next commands = %v, want [foo bar commandx]", got)
		}
	})

	t.Run("same-named command replaced in place", func(t *testing.T) {
		inGlobDir(t, "services/api", "services/frontend-next")
		loc := parent
		loc.Overrides = map[string]LocationOverride{
			"frontend-next": {Commands: []Command{
				{Name: "bar", Command: "pnpm run bar"},
				{Name: "commandx", Command: "npm run commandx"},
			}},
		}

		locs, _, err := ExpandGlobPatterns([]Location{loc})
		if err != nil {
			t.Fatalf("ExpandGlobPatterns() error = %v", err)
		}
		if got := commandNames(locs[1].Commands); !reflect.DeepEqual(got, []string{"foo", "bar", "commandx"}) {
			t.Errorf("commands = %v, want bar replaced in place", got)
		}
		if got := locs[1].Commands[1].Command; got != "pnpm run bar" {
			t.Errorf("bar = %q, want the override's body", got)
		}
		if got := locs[0].Commands[1].Command; got != "npm run bar" {
			t.Errorf("sibling bar = %q, want the inherited body", got)
		}
	})

	t.Run("env merges per key", func(t *testing.T) {
		inGlobDir(t, "services/api", "services/frontend-next")
		loc := parent
		loc.Overrides = map[string]LocationOverride{
			"frontend-next": {Env: map[string]string{"PORT": "3001", "LOG": "debug"}},
		}

		locs, _, err := ExpandGlobPatterns([]Location{loc})
		if err != nil {
			t.Fatalf("ExpandGlobPatterns() error = %v", err)
		}
		want := map[string]string{"LOG": "debug", "PORT": "3001"}
		if !reflect.DeepEqual(locs[1].Env, want) {
			t.Errorf("frontend-next Env = %v, want %v", locs[1].Env, want)
		}
		if !reflect.DeepEqual(locs[0].Env, map[string]string{"LOG": "info"}) {
			t.Errorf("api Env = %v, want the parent's env untouched", locs[0].Env)
		}
	})

	t.Run("type replaced", func(t *testing.T) {
		inGlobDir(t, "services/api", "services/frontend-next")
		loc := parent
		loc.Overrides = map[string]LocationOverride{
			"frontend-next": {Types: Types{"docker"}},
		}

		locs, _, err := ExpandGlobPatterns([]Location{loc})
		if err != nil {
			t.Fatalf("ExpandGlobPatterns() error = %v", err)
		}
		if !reflect.DeepEqual(locs[1].Types, Types{"docker"}) {
			t.Errorf("Types = %v, want the override to replace, not union", locs[1].Types)
		}
		if !reflect.DeepEqual(locs[0].Types, Types{"npm"}) {
			t.Errorf("sibling Types = %v, want [npm]", locs[0].Types)
		}
	})

	t.Run("filters replaced", func(t *testing.T) {
		inGlobDir(t, "services/api", "services/frontend-next")
		loc := parent
		loc.Overrides = map[string]LocationOverride{
			"frontend-next": {Include: []string{"commandx"}, Exclude: []string{"bar"}},
		}

		locs, _, err := ExpandGlobPatterns([]Location{loc})
		if err != nil {
			t.Fatalf("ExpandGlobPatterns() error = %v", err)
		}
		if !reflect.DeepEqual(locs[1].Include, []string{"commandx"}) {
			t.Errorf("Include = %v, want the override's list", locs[1].Include)
		}
		if !reflect.DeepEqual(locs[1].Exclude, []string{"bar"}) {
			t.Errorf("Exclude = %v, want the override's list", locs[1].Exclude)
		}
		if !reflect.DeepEqual(locs[0].Include, []string{"foo*"}) {
			t.Errorf("sibling Include = %v, want the inherited list", locs[0].Include)
		}
	})

	t.Run("name replaced", func(t *testing.T) {
		inGlobDir(t, "services/api", "services/frontend-next")
		loc := parent
		loc.Overrides = map[string]LocationOverride{
			"frontend-next": {Name: "next"},
		}

		locs, _, err := ExpandGlobPatterns([]Location{loc})
		if err != nil {
			t.Fatalf("ExpandGlobPatterns() error = %v", err)
		}
		if got := locationNames(locs); !reflect.DeepEqual(got, []string{"api", "next"}) {
			t.Errorf("names = %v, want [api next]", got)
		}
	})
}

func TestExpandGlobPatterns_OverrideKeyGlobMatchesSeveralFolders(t *testing.T) {
	inGlobDir(t, "services/api", "services/mail-worker", "services/sms-worker")

	locs, warnings, err := ExpandGlobPatterns([]Location{{
		Location: "services/*",
		Overrides: map[string]LocationOverride{
			"*-worker": {Env: map[string]string{"QUEUE": "default"}},
		},
	}})
	if err != nil {
		t.Fatalf("ExpandGlobPatterns() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %+v", warnings)
	}
	if locs[0].Env != nil {
		t.Errorf("api Env = %v, want nil", locs[0].Env)
	}
	for _, i := range []int{1, 2} {
		if !reflect.DeepEqual(locs[i].Env, map[string]string{"QUEUE": "default"}) {
			t.Errorf("%s Env = %v, want the override applied", locs[i].Name, locs[i].Env)
		}
	}
}

func TestExpandGlobPatterns_OverrideKeyMatchesNoFolder(t *testing.T) {
	inGlobDir(t, "services/api")

	_, warnings, err := ExpandGlobPatterns([]Location{{
		Location: "services/*",
		Overrides: map[string]LocationOverride{
			"zeta":  {Name: "z"},
			"alpha": {Name: "a"},
		},
	}})
	if err != nil {
		t.Fatalf("ExpandGlobPatterns() error = %v", err)
	}
	if got := warningNames(warnings); !reflect.DeepEqual(got, []string{"alpha", "zeta"}) {
		t.Fatalf("warnings = %v, want one per unmatched key in sorted order", got)
	}
	for _, w := range warnings {
		if w.Kind != "ignored" {
			t.Errorf("warning %+v, want Kind 'ignored'", w)
		}
	}
}

// TestExpandGlobPatterns_OverrideForExcludedFolderIsSilent: excluding a folder and
// keeping its override entry is a normal intermediate state, not a typo.
func TestExpandGlobPatterns_OverrideForExcludedFolderIsSilent(t *testing.T) {
	inGlobDir(t, "services/api", "services/legacy")

	locs, warnings, err := ExpandGlobPatterns([]Location{{
		Location:         "services/*",
		ExcludeLocations: []string{"legacy"},
		Overrides: map[string]LocationOverride{
			"legacy": {Name: "old"},
		},
	}})
	if err != nil {
		t.Fatalf("ExpandGlobPatterns() error = %v", err)
	}
	if got := locationNames(locs); !reflect.DeepEqual(got, []string{"api"}) {
		t.Errorf("locations = %v, want [api]", got)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %+v", warnings)
	}
}

// TestExpandGlobPatterns_OverridesNotCopiedToChildren: the directives are consumed
// during expansion, so an expanded child must not carry them.
func TestExpandGlobPatterns_OverridesNotCopiedToChildren(t *testing.T) {
	inGlobDir(t, "services/api")

	locs, _, err := ExpandGlobPatterns([]Location{{
		Location:         "services/*",
		ExcludeLocations: []string{"nothing"},
		Overrides:        map[string]LocationOverride{"api": {Name: "api"}},
	}})
	if err != nil {
		t.Fatalf("ExpandGlobPatterns() error = %v", err)
	}
	if len(locs[0].Overrides) != 0 || len(locs[0].ExcludeLocations) != 0 {
		t.Errorf("child carries expansion directives: %+v", locs[0])
	}
}
