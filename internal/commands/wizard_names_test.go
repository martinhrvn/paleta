package commands

import (
	"reflect"
	"testing"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/scan"
)

// namesFromCandidates maps each newly detected path to the name the wizard would
// write it under.
func namesFromCandidates(t *testing.T, cands []scan.Candidate, authored *config.Config) map[string]string {
	t.Helper()
	items := BuildWizardItems(cands, authored)
	out := make(map[string]string, len(items))
	for _, it := range items {
		if it.Configured {
			continue
		}
		out[it.Location.Location] = it.Location.Name
	}
	return out
}

func TestAssignWizardNames_BaseWhenUnique(t *testing.T) {
	got := namesFromCandidates(t, []scan.Candidate{
		{RelPath: ".", Types: []string{"go"}},
		{RelPath: "services/api", Types: []string{"go"}},
		{RelPath: "packages/web", Types: []string{"npm"}},
	}, nil)

	want := map[string]string{
		".":            "root",
		"services/api": "api",
		"packages/web": "web",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
}

// TestAssignWizardNames_PathOnCollision: two folders sharing a base name would
// both be called "web", which makes @web:build ambiguous and merges their
// frecency history. Both fall back to their path instead.
func TestAssignWizardNames_PathOnCollision(t *testing.T) {
	got := namesFromCandidates(t, []scan.Candidate{
		{RelPath: "apps/web", Types: []string{"npm"}},
		{RelPath: "packages/web", Types: []string{"npm"}},
		{RelPath: "services/api", Types: []string{"go"}},
	}, nil)

	want := map[string]string{
		"apps/web":     "apps/web",
		"packages/web": "packages/web",
		"services/api": "api",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
}

// TestAssignWizardNames_NeverShadowsAuthoredName: an authored location keeps its
// name; a newly detected folder that would claim the same one takes its path.
func TestAssignWizardNames_NeverShadowsAuthoredName(t *testing.T) {
	authored := &config.Config{Locations: []config.Location{
		{Name: "web", Location: "apps/web", Types: config.Types{"npm"}},
	}}

	got := namesFromCandidates(t, []scan.Candidate{
		{RelPath: "apps/web", Types: []string{"npm"}},
		{RelPath: "packages/web", Types: []string{"npm"}},
	}, authored)

	want := map[string]string{"packages/web": "packages/web"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("names = %v, want %v (apps/web is already authored)", got, want)
	}
}

// TestAssignWizardNames_CollidesWithUnnamedAuthoredFolder: alias resolution
// indexes folder base names even when a location has no name, so a candidate
// whose base matches an authored folder's base still has to back off.
func TestAssignWizardNames_CollidesWithUnnamedAuthoredFolder(t *testing.T) {
	authored := &config.Config{Locations: []config.Location{
		{Location: "apps/web", Types: config.Types{"npm"}},
	}}

	got := namesFromCandidates(t, []scan.Candidate{
		{RelPath: "packages/web", Types: []string{"npm"}},
	}, authored)

	want := map[string]string{"packages/web": "packages/web"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
}

// TestBuildWizardItems_OffersNewlyDetectedTypes: a location already in the config
// that has since grown a second project type gets it as an opt-in option, with
// its authored types still the ones ticked.
func TestBuildWizardItems_OffersNewlyDetectedTypes(t *testing.T) {
	authored := &config.Config{Locations: []config.Location{
		{Name: "api", Location: "services/api", Types: config.Types{"npm"}},
	}}
	cands := []scan.Candidate{{RelPath: "services/api", Types: []string{"npm", "docker"}}}

	items := BuildWizardItems(cands, authored)
	if len(items) != 1 {
		t.Fatalf("items = %+v, want one", items)
	}
	if !reflect.DeepEqual(items[0].TypeOptions, []string{"npm", "docker"}) {
		t.Errorf("TypeOptions = %v, want the authored type plus the detected one", items[0].TypeOptions)
	}
	if !reflect.DeepEqual(items[0].Location.Types, config.Types{"npm"}) {
		t.Errorf("Types = %v, want only the authored type ticked", items[0].Location.Types)
	}
}

// TestBuildWizardItems_NewLocationOffersAllDetectedTypes: nothing is authored, so
// every detected type is both offered and ticked.
func TestBuildWizardItems_NewLocationOffersAllDetectedTypes(t *testing.T) {
	items := BuildWizardItems([]scan.Candidate{
		{RelPath: "services/api", Types: []string{"pnpm", "docker"}},
	}, nil)

	if !reflect.DeepEqual(items[0].TypeOptions, []string{"pnpm", "docker"}) {
		t.Errorf("TypeOptions = %v", items[0].TypeOptions)
	}
	if !reflect.DeepEqual(items[0].Location.Types, config.Types{"pnpm", "docker"}) {
		t.Errorf("Types = %v, want all detected types ticked", items[0].Location.Types)
	}
}

// TestBuildWizardItems_AuthoredTypeNotDetectedIsKept: a hand-written type whose
// detect file is gone (or that plt can't see) must not vanish from the config.
func TestBuildWizardItems_AuthoredTypeNotDetectedIsKept(t *testing.T) {
	authored := &config.Config{Locations: []config.Location{
		{Name: "api", Location: "services/api", Types: config.Types{"make"}},
	}}
	cands := []scan.Candidate{{RelPath: "services/api", Types: []string{"npm"}}}

	items := BuildWizardItems(cands, authored)
	if !reflect.DeepEqual(items[0].TypeOptions, []string{"make", "npm"}) {
		t.Errorf("TypeOptions = %v, want the authored type kept alongside the detected one", items[0].TypeOptions)
	}
	if !reflect.DeepEqual(items[0].Location.Types, config.Types{"make"}) {
		t.Errorf("Types = %v, want the authored type still ticked", items[0].Location.Types)
	}
}
