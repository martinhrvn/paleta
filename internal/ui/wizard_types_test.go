package ui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/martinhrvn/paleta/internal/config"
)

// multiTypeItems: one single-type location and one that matched two types, plus a
// configured location that has since grown a second type.
func multiTypeItems() []WizardItem {
	return []WizardItem{
		{
			Location:    config.Location{Name: "web", Location: "packages/web", Types: config.Types{"npm"}},
			TypeOptions: []string{"npm"},
			Detected:    true,
		},
		{
			Location:    config.Location{Name: "api", Location: "services/api", Types: config.Types{"pnpm", "docker"}},
			TypeOptions: []string{"pnpm", "docker"},
			Detected:    true,
		},
		{
			Location:    config.Location{Name: "infra", Location: "infra", Types: config.Types{"npm"}},
			TypeOptions: []string{"npm", "docker"},
			Detected:    true,
			Configured:  true,
		},
	}
}

// rowKeys renders the model's visible rows as "path" / "path>type" strings.
func rowKeys(m WizardModel) []string {
	out := make([]string, 0, len(m.rows))
	for _, r := range m.rows {
		key := m.items[r.item].Location.Location
		if r.typ != "" {
			key += ">" + r.typ
		}
		out = append(out, key)
	}
	return out
}

// TestWizard_SingleTypeHasNoChildRows: a location with one type stays a single
// compact row — unticking its only type would just mean unticking the location.
func TestWizard_SingleTypeHasNoChildRows(t *testing.T) {
	m := NewWizardModel([]WizardItem{multiTypeItems()[0]})
	if got := rowKeys(m); !reflect.DeepEqual(got, []string{"packages/web"}) {
		t.Errorf("rows = %v, want just the location row", got)
	}
}

func TestWizard_MultiTypeExpandsToChildRows(t *testing.T) {
	m := NewWizardModel(multiTypeItems())
	want := []string{
		"packages/web",
		"services/api", "services/api>pnpm", "services/api>docker",
		"infra", "infra>npm", "infra>docker",
	}
	if got := rowKeys(m); !reflect.DeepEqual(got, want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

// TestWizard_ConfiguredNewTypeStartsUnticked: a type the location didn't declare
// before is opt-in — ticking it changes how that location's commands are labelled.
func TestWizard_ConfiguredNewTypeStartsUnticked(t *testing.T) {
	m := NewWizardModel(multiTypeItems())

	if !m.typeSelected(2, "npm") {
		t.Errorf("authored type npm should start ticked")
	}
	if m.typeSelected(2, "docker") {
		t.Errorf("newly detected type docker should start unticked on a configured location")
	}
	// A brand-new location takes all its detected types.
	if !m.typeSelected(1, "pnpm") || !m.typeSelected(1, "docker") {
		t.Errorf("a newly detected location should start with every type ticked")
	}

	row := m.formatRow(6) // infra>docker
	if !strings.Contains(row, "new") {
		t.Errorf("row = %q, want the added type marked new", row)
	}
}

func TestWizard_SelectedLocationsUsesTickedTypes(t *testing.T) {
	m := NewWizardModel(multiTypeItems())
	m.toggle(3) // untick services/api>docker
	m.confirmed = true

	locs := m.SelectedLocations()
	var api *config.Location
	for i := range locs {
		if locs[i].Location == "services/api" {
			api = &locs[i]
		}
	}
	if api == nil {
		t.Fatalf("services/api missing from %+v", locs)
	}
	if !reflect.DeepEqual(api.Types, config.Types{"pnpm"}) {
		t.Errorf("types = %v, want only the ticked one", api.Types)
	}
}

// TestWizard_UntickingLastTypeUnticksLocation: a location with no types left has
// nothing to contribute, so it drops out of the selection.
func TestWizard_UntickingLastTypeUnticksLocation(t *testing.T) {
	m := NewWizardModel(multiTypeItems())
	m.toggle(2) // services/api>pnpm
	m.toggle(3) // services/api>docker

	if m.selected[1] {
		t.Errorf("location should be unticked once its last type is")
	}
	m.confirmed = true
	for _, l := range m.SelectedLocations() {
		if l.Location == "services/api" {
			t.Errorf("services/api should not be selected: %+v", l)
		}
	}
}

func TestWizard_TickingTypeReticksLocation(t *testing.T) {
	m := NewWizardModel(multiTypeItems())
	m.toggle(2)
	m.toggle(3)
	if m.selected[1] {
		t.Fatalf("precondition: location should be unticked")
	}

	m.toggle(3) // tick docker again
	if !m.selected[1] {
		t.Errorf("ticking a type should bring its location back")
	}
	m.confirmed = true
	for _, l := range m.SelectedLocations() {
		if l.Location == "services/api" && !reflect.DeepEqual(l.Types, config.Types{"docker"}) {
			t.Errorf("types = %v, want [docker]", l.Types)
		}
	}
}

// TestWizard_LocationToggleAppliesToItsTypes keeps the two levels consistent:
// unticking a location clears its types, and ticking it restores them all.
func TestWizard_LocationToggleAppliesToItsTypes(t *testing.T) {
	m := NewWizardModel(multiTypeItems())

	m.toggle(1) // untick services/api itself
	if m.typeSelected(1, "pnpm") || m.typeSelected(1, "docker") {
		t.Errorf("unticking a location should clear its types")
	}

	m.toggle(1)
	if !m.typeSelected(1, "pnpm") || !m.typeSelected(1, "docker") {
		t.Errorf("re-ticking a location should restore its types")
	}
}
