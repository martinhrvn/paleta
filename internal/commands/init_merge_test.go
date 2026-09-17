package commands

import (
	"testing"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/scan"
	"github.com/martinhrvn/paleta/internal/ui"
)

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

func findItem(items []ui.WizardItem, location string) *ui.WizardItem {
	for i := range items {
		if items[i].Location.Location == location {
			return &items[i]
		}
	}
	return nil
}
