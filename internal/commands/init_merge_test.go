package commands

import (
	"os"
	"path/filepath"
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
		{RelPath: ".", Type: "go", DetectFile: "go.mod"},
		{RelPath: "packages/web", Type: "npm", DetectFile: "package.json"},
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
			{Name: "web", Location: "packages/web", Type: "npm"},
			{Location: "packages/*"}, // glob, not detected
		},
	}
	cands := []scan.Candidate{
		{RelPath: "packages/web", Type: "npm", DetectFile: "package.json"},
		{RelPath: "services/api", Type: "go", DetectFile: "go.mod"},
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
		{Name: "root", Location: ".", Type: "go"},
		{Name: "web", Location: "packages/web", Type: "npm"},
	}
	out := GenerateConfig(locs, nil)

	var parsed config.Config
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("generated config is not valid YAML: %v\n%s", err, out)
	}
	if len(parsed.Locations) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(parsed.Locations))
	}
	if parsed.Locations[1].Location != "packages/web" || parsed.Locations[1].Type != "npm" {
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
	out := GenerateConfig([]config.Location{{Name: "x", Location: "x", Type: "go"}}, preserved)
	if !strings.Contains(out, "frecency:") {
		t.Errorf("expected frecency block to be preserved:\n%s", out)
	}
}

func TestGenerateConfig_IncludeExcludeRoundTrip(t *testing.T) {
	locs := []config.Location{
		{
			Name:     "web",
			Location: "packages/web",
			Type:     "npm",
			Include:  []string{"npm run dev"},
			Exclude:  []string{"npm run test:watch"},
		},
	}
	out := GenerateConfig(locs, nil)

	var parsed config.Config
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v\n%s", err, out)
	}
	loc := parsed.Locations[0]
	if len(loc.Include) != 1 || loc.Include[0] != "npm run dev" {
		t.Errorf("include not round-tripped: %+v", loc.Include)
	}
	if len(loc.Exclude) != 1 || loc.Exclude[0] != "npm run test:watch" {
		t.Errorf("exclude not round-tripped: %+v", loc.Exclude)
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
