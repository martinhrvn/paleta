package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const cachedProjectYAML = "locations:\n  - name: infra\n    location: \".\"\n    type: make\n    commands:\n      - name: deploy\n        command: \"./deploy.sh\"\n"

// countingProject is a make project whose parser appends a line to a counter file
// each time it runs, and fails while a `fail` marker file exists — so tests can
// tell whether a load ran the parser or trusted the cache.
type countingProject struct {
	configPath string
	dir        string
	counter    string
	failMarker string
}

func newCountingProject(t *testing.T) countingProject {
	t.Helper()
	state := t.TempDir()
	p := countingProject{
		counter:    filepath.Join(state, "runs"),
		failMarker: filepath.Join(state, "fail"),
	}
	parser := fmt.Sprintf(`"echo x >> %s; test -e %s && exit 1; printf 'test\\nlint\\n'"`, p.counter, p.failMarker)
	p.configPath = makeProject(t, parser, cachedProjectYAML)
	p.dir = filepath.Dir(p.configPath)
	return p
}

func (p countingProject) runs(t *testing.T) int {
	t.Helper()
	data, err := os.ReadFile(p.counter)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("read counter: %v", err)
	}
	return strings.Count(string(data), "x")
}

// touchMakefile changes the detect file so a cached result goes stale.
func (p countingProject) touchMakefile(t *testing.T) {
	t.Helper()
	makefile := filepath.Join(p.dir, "Makefile")
	if err := os.WriteFile(makefile, []byte("build:\ntest:\n"), 0644); err != nil {
		t.Fatalf("rewrite Makefile: %v", err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(makefile, later, later); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

// loadResolved is a non-interactive load: everything resolved up front.
func (p countingProject) loadResolved(t *testing.T) *Config {
	t.Helper()
	cfg, err := LoadConfig(p.configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	ResolveAllPending(cfg)
	return cfg
}

var resolvedInfra = []string{"deploy", "build", "lint", "test"}

// A non-interactive load trusts a cached result while the Makefile is unchanged:
// the second `plt list` doesn't run the parser at all.
func TestResolveAllPending_UsesFreshCache(t *testing.T) {
	p := newCountingProject(t)

	p.loadResolved(t)
	if got := p.runs(t); got != 1 {
		t.Fatalf("first load ran the parser %d times, want 1", got)
	}

	cfg := p.loadResolved(t)
	if got := p.runs(t); got != 1 {
		t.Errorf("second load ran the parser (%d runs), want the cached result", got)
	}
	if got := commandNamesOf(cfg.Locations[0].Commands); !reflect.DeepEqual(got, resolvedInfra) {
		t.Errorf("commands = %v, want %v", got, resolvedInfra)
	}
}

// Once the Makefile changes, the cached result is stale and the parser runs again.
func TestResolveAllPending_RerunsParserWhenStale(t *testing.T) {
	p := newCountingProject(t)
	p.loadResolved(t)

	p.touchMakefile(t)
	p.loadResolved(t)
	if got := p.runs(t); got != 2 {
		t.Errorf("parser ran %d times, want 2 after the Makefile changed", got)
	}

	// ...and the refreshed result is cached in turn.
	p.loadResolved(t)
	if got := p.runs(t); got != 2 {
		t.Errorf("parser ran %d times, want the refreshed result reused", got)
	}
}

// An interactive (deferred) load shows cached commands immediately — no
// placeholder row — but still marks the type for a background refresh.
func TestLoadConfig_DeferredTypeShowsCachedCommands(t *testing.T) {
	p := newCountingProject(t)
	p.loadResolved(t)

	cfg, err := LoadConfig(p.configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	loc := cfg.Locations[0]
	if got := commandNamesOf(loc.Commands); !reflect.DeepEqual(got, resolvedInfra) {
		t.Errorf("commands = %v, want the cached %v", got, resolvedInfra)
	}
	if !reflect.DeepEqual(loc.PendingTypes, []string{"make"}) {
		t.Errorf("PendingTypes = %v, want [make] so it still refreshes", loc.PendingTypes)
	}
	if !reflect.DeepEqual(loc.Refreshing, []string{"make"}) {
		t.Errorf("Refreshing = %v, want [make]", loc.Refreshing)
	}
	for _, row := range cfg.Rows(false) {
		if len(row.Pending) > 0 {
			t.Errorf("got placeholder row %+v, want none for a cached type", row)
		}
	}
	if got := p.runs(t); got != 1 {
		t.Errorf("deferred load ran the parser (%d runs)", got)
	}
}

// The background refresh always runs the parser — the cache may be fresh by
// fingerprint yet out of date (a new gradle plugin) — and stores what it finds.
func TestResolvePendingTypes_AlwaysRefreshes(t *testing.T) {
	p := newCountingProject(t)
	p.loadResolved(t)

	cfg, err := LoadConfig(p.configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	commands, warnings := ResolvePendingTypes(cfg.Locations[0])
	if got := p.runs(t); got != 2 {
		t.Errorf("parser ran %d times, want a refresh", got)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if got := commandNamesOf(commands); !reflect.DeepEqual(got, resolvedInfra) {
		t.Errorf("commands = %v, want %v", got, resolvedInfra)
	}
}

// A failed refresh keeps the cached commands rather than dropping to the type's
// base commands, and says so.
func TestResolvePendingTypes_FailedRefreshKeepsCache(t *testing.T) {
	p := newCountingProject(t)
	p.loadResolved(t)

	if err := os.WriteFile(p.failMarker, nil, 0644); err != nil {
		t.Fatalf("write fail marker: %v", err)
	}
	cfg, err := LoadConfig(p.configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	commands, warnings := ResolvePendingTypes(cfg.Locations[0])
	if got := commandNamesOf(commands); !reflect.DeepEqual(got, resolvedInfra) {
		t.Errorf("commands = %v, want the cached %v", got, resolvedInfra)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Reason, "cached") {
		t.Errorf("warnings = %+v, want one saying cached commands are shown", warnings)
	}

	// The failure must not overwrite the good entry.
	os.Remove(p.failMarker)
	p.loadResolved(t)
	if got := p.runs(t); got != 2 {
		t.Errorf("parser ran %d times, want the earlier good result still cached", got)
	}
}

// A failed parse is never cached: with no earlier result, the next load retries.
func TestResolveAllPending_DoesNotCacheFailure(t *testing.T) {
	p := newCountingProject(t)
	if err := os.WriteFile(p.failMarker, nil, 0644); err != nil {
		t.Fatalf("write fail marker: %v", err)
	}
	p.loadResolved(t)
	p.loadResolved(t)
	if got := p.runs(t); got != 2 {
		t.Errorf("parser ran %d times, want a retry after a failure", got)
	}
}
