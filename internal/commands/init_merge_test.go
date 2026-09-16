package commands

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/scan"
	"github.com/martinhrvn/paleta/internal/ui"
	"gopkg.in/yaml.v3"
)

func TestLoadAuthoredConfig_Missing(t *testing.T) {
	cfg, err := LoadAuthoredConfig(filepath.Join(t.TempDir(), ".pltrc"))
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config for missing file, got %+v", cfg)
	}
}

func TestLoadAuthoredConfig_PreservesAuthored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pltrc")
	content := `frecency:
  enabled: true
  frequency_weight: 50
  recency_weight: 50
locations:
  - name: web
    location: packages/web
    type: npm
  - location: "packages/*"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAuthoredConfig(path)
	if err != nil {
		t.Fatalf("LoadAuthoredConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if len(cfg.Locations) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(cfg.Locations))
	}
	// Authored relative paths must NOT be made absolute.
	if cfg.Locations[0].Location != "packages/web" {
		t.Errorf("expected location packages/web, got %q", cfg.Locations[0].Location)
	}
	if cfg.Locations[1].Location != "packages/*" {
		t.Errorf("expected glob preserved, got %q", cfg.Locations[1].Location)
	}
	if !cfg.Frecency.Enabled {
		t.Error("expected frecency enabled to be preserved")
	}
}

func TestBuildWizardItems_ScanOnly(t *testing.T) {
	cands := []scan.Candidate{
		{RelPath: ".", Types: []string{"go"}, DetectFile: "go.mod"},
		{RelPath: "packages/web", Types: []string{"npm"}, DetectFile: "package.json"},
	}
	items := BuildWizardItems(cands, nil)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for _, it := range items {
		if !it.Detected || it.Configured {
			t.Errorf("scan-only item should be Detected and not Configured: %+v", it)
		}
	}
	if items[0].Location.Name != "root" {
		t.Errorf("expected root item name 'root', got %q", items[0].Location.Name)
	}
	if items[1].Location.Name != "web" {
		t.Errorf("expected name 'web', got %q", items[1].Location.Name)
	}
}

func TestBuildWizardItems_MergesConfigured(t *testing.T) {
	authored := &config.Config{
		Locations: []config.Location{
			{Name: "web", Location: "packages/web", Types: config.Types{"npm"}},
			{Location: "packages/*"}, // glob, not detected
		},
	}
	cands := []scan.Candidate{
		{RelPath: "packages/web", Types: []string{"npm"}, DetectFile: "package.json"},
		{RelPath: "services/api", Types: []string{"go"}, DetectFile: "go.mod"},
	}
	items := BuildWizardItems(cands, authored)

	web := findItem(items, "packages/web")
	if web == nil || !web.Detected || !web.Configured {
		t.Fatalf("packages/web should be both detected and configured: %+v", web)
	}
	if web.Location.Name != "web" {
		t.Errorf("authored name should be preserved, got %q", web.Location.Name)
	}

	glob := findItem(items, "packages/*")
	if glob == nil || glob.Detected || !glob.Configured {
		t.Fatalf("glob location should be configured-only: %+v", glob)
	}

	api := findItem(items, "services/api")
	if api == nil || !api.Detected || api.Configured {
		t.Fatalf("services/api should be detected-only: %+v", api)
	}
}

func TestGenerateConfig_RoundTrip(t *testing.T) {
	locs := []config.Location{
		{Name: "root", Location: ".", Types: config.Types{"go"}},
		{Name: "web", Location: "packages/web", Types: config.Types{"npm"}},
	}
	out := GenerateConfig(locs, nil)

	var parsed config.Config
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("generated config is not valid YAML: %v\n%s", err, out)
	}
	if len(parsed.Locations) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(parsed.Locations))
	}
	if parsed.Locations[1].Location != "packages/web" ||
		len(parsed.Locations[1].Types) != 1 || parsed.Locations[1].Types[0] != "npm" {
		t.Errorf("location not round-tripped: %+v", parsed.Locations[1])
	}
	if strings.Contains(out, "frecency:") {
		t.Error("did not expect frecency block when none preserved")
	}
}

func TestGenerateConfig_PreservesFrecency(t *testing.T) {
	preserved := &config.Config{
		Frecency: config.FrecencyConfig{Enabled: true, RecencyWeight: 50, FrequencyWeight: 50},
	}
	out := GenerateConfig([]config.Location{{Name: "x", Location: ".", Types: config.Types{"go"}}}, preserved)
	if !strings.Contains(out, "frecency:") {
		t.Errorf("expected frecency block to be preserved:\n%s", out)
	}
}

func TestGenerateConfig_IncludeExcludeRoundTrip(t *testing.T) {
	locs := []config.Location{
		{
			Name:     "web",
			Location: "packages/web",
			Types:    config.Types{"npm"},
			Include:  []string{"dev"},
			Exclude:  []string{"test:watch"},
		},
	}
	out := GenerateConfig(locs, nil)

	// Rewrites emit the current spelling.
	if !strings.Contains(out, "include_commands:") || !strings.Contains(out, "exclude_commands:") {
		t.Errorf("expected the canonical filter keys:\n%s", out)
	}
	if strings.Contains(out, "\n      include:") || strings.Contains(out, "\n      exclude:") {
		t.Errorf("expected no deprecated filter keys:\n%s", out)
	}

	var parsed config.Config
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v\n%s", err, out)
	}
	loc := parsed.Locations[0]
	if len(loc.Include) != 1 || loc.Include[0] != "dev" {
		t.Errorf("include not round-tripped: %+v", loc.Include)
	}
	if len(loc.Exclude) != 1 || loc.Exclude[0] != "test:watch" {
		t.Errorf("exclude not round-tripped: %+v", loc.Exclude)
	}
}

func TestGenerateConfig_OverridesRoundTrip(t *testing.T) {
	locs := []config.Location{
		{
			Location:         "services/*",
			Types:            config.Types{"npm"},
			ExcludeLocations: []string{"legacy"},
			Overrides: map[string]config.LocationOverride{
				"zeta":  {Name: "z"},
				"alpha": {Env: map[string]string{"PORT": "3001"}},
				"mid":   {Commands: []config.Command{{Name: "commandx", Command: "npm run commandx"}}},
			},
		},
	}
	out := GenerateConfig(locs, nil)

	var parsed config.Config
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v\n%s", err, out)
	}
	loc := parsed.Locations[0]
	if !reflect.DeepEqual(loc.ExcludeLocations, []string{"legacy"}) {
		t.Errorf("exclude_locations not round-tripped: %+v", loc.ExcludeLocations)
	}
	if len(loc.Overrides) != 3 {
		t.Fatalf("overrides not round-tripped: %+v", loc.Overrides)
	}
	if loc.Overrides["zeta"].Name != "z" {
		t.Errorf("override name lost: %+v", loc.Overrides["zeta"])
	}
	if loc.Overrides["alpha"].Env["PORT"] != "3001" {
		t.Errorf("override env lost: %+v", loc.Overrides["alpha"])
	}
	if cmds := loc.Overrides["mid"].Commands; len(cmds) != 1 || cmds[0].Name != "commandx" {
		t.Errorf("override commands lost: %+v", cmds)
	}

	// Override keys are emitted in a stable (sorted) order, so repeated rewrites
	// don't churn the file.
	alpha, mid, zeta := strings.Index(out, "alpha:"), strings.Index(out, "mid:"), strings.Index(out, "zeta:")
	if !(alpha >= 0 && alpha < mid && mid < zeta) {
		t.Errorf("override keys not emitted in sorted order (alpha=%d mid=%d zeta=%d):\n%s", alpha, mid, zeta, out)
	}
}

// TestLoadAuthoredConfig_LegacyKeysCanonicalized: reading a legacy-spelling file
// and writing it back through GenerateConfig (as `plt init`, focus and queue-save
// all do) upgrades the spelling.
func TestLoadAuthoredConfig_LegacyKeysCanonicalized(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pltrc")
	original := `locations:
    - name: web
      location: packages/web
      type: npm
      include:
        - build*
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	authored, err := LoadAuthoredConfig(path)
	if err != nil {
		t.Fatalf("LoadAuthoredConfig: %v", err)
	}
	if !reflect.DeepEqual(authored.Locations[0].Include, []string{"build*"}) {
		t.Fatalf("legacy include not decoded: %+v", authored.Locations[0])
	}

	out := GenerateConfig(authored.Locations, authored)
	if !strings.Contains(out, "include_commands:") {
		t.Errorf("rewrite did not upgrade the spelling:\n%s", out)
	}
}

func findItem(items []ui.WizardItem, location string) *ui.WizardItem {
	for i := range items {
		if items[i].Location.Location == location {
			return &items[i]
		}
	}
	return nil
}

func TestGenerateConfig_MultiAndSingleType(t *testing.T) {
	locs := []config.Location{
		{Name: "svc", Location: "svc", Types: config.Types{"npm", "docker"}},
		{Name: "api", Location: "api", Types: config.Types{"go"}},
	}
	out := GenerateConfig(locs, nil)

	// Multi-type serializes as a YAML list...
	if !strings.Contains(out, "type:") || !strings.Contains(out, "- npm") || !strings.Contains(out, "- docker") {
		t.Errorf("expected multi-type list in output:\n%s", out)
	}
	// ...single-type stays a scalar.
	if !strings.Contains(out, "type: go") {
		t.Errorf("expected single-type scalar 'type: go' in output:\n%s", out)
	}

	// Round-trips back to the same Types.
	var parsed config.Config
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v\n%s", err, out)
	}
	if len(parsed.Locations) != 2 ||
		!reflect.DeepEqual(parsed.Locations[0].Types, config.Types{"npm", "docker"}) ||
		!reflect.DeepEqual(parsed.Locations[1].Types, config.Types{"go"}) {
		t.Errorf("types not round-tripped: %+v", parsed.Locations)
	}
}
