package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhrvn/paleta/internal/config"
)

// siblingItems: three npm packages under packages/ plus one unrelated location,
// the shape that collapses to `packages/*`.
func siblingItems() []WizardItem {
	items := []WizardItem{{
		Location:    config.Location{Name: "infra", Location: "infra", Types: config.Types{"make"}},
		TypeOptions: []string{"make"},
		Detected:    true,
	}}
	for _, name := range []string{"a", "b", "c"} {
		items = append(items, WizardItem{
			Location:    config.Location{Name: name, Location: "packages/" + name, Types: config.Types{"npm"}},
			TypeOptions: []string{"npm"},
			Detected:    true,
		})
	}
	return items
}

func confirmed(m WizardModel) WizardModel {
	m.confirmed = true
	return m
}

// locationPaths renders a selection as its written `location:` values.
func locationPaths(locs []config.Location) []string {
	out := make([]string, 0, len(locs))
	for _, l := range locs {
		out = append(out, l.Location)
	}
	return out
}

// TestWizard_GlobOutputIsDefault: three siblings are written as one glob without
// the user doing anything, since that's the config they'd have written by hand.
func TestWizard_GlobOutputIsDefault(t *testing.T) {
	m := confirmed(NewWizardModel(siblingItems()))

	got := locationPaths(m.SelectedLocations())
	want := []string{"infra", "packages/*"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("locations = %v, want %v", got, want)
	}
}

// TestWizard_GlobToggleChangesOutput: ^g writes exactly what was ticked instead,
// for anyone who'd rather see the folders spelled out.
func TestWizard_GlobToggleChangesOutput(t *testing.T) {
	m := NewWizardModel(siblingItems())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = confirmed(updated.(WizardModel))

	got := locationPaths(m.SelectedLocations())
	want := []string{"infra", "packages/a", "packages/b", "packages/c"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("locations = %v, want the explicit folders %v", got, want)
	}

	// And back again.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = confirmed(updated.(WizardModel))
	if got := locationPaths(m.SelectedLocations()); strings.Join(got, ",") != "infra,packages/*" {
		t.Errorf("locations after toggling back = %v, want the glob", got)
	}
}

// TestWizard_RowShowsGlobAnnotation: the rows a collapse would absorb say so, so
// the written form is visible before Enter rather than a surprise in .pltrc.
func TestWizard_RowShowsGlobAnnotation(t *testing.T) {
	m := NewWizardModel(siblingItems())

	if row := m.formatRow(0); strings.Contains(row, "packages/*") {
		t.Errorf("infra row = %q, want no glob annotation", row)
	}
	for pos := 1; pos <= 3; pos++ {
		if row := m.formatRow(pos); !strings.Contains(row, "packages/*") {
			t.Errorf("row %d = %q, want it annotated with the glob it folds into", pos, row)
		}
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = updated.(WizardModel)
	if row := m.formatRow(1); strings.Contains(row, "packages/*") {
		t.Errorf("row = %q, want no annotation once glob output is off", row)
	}
}

// TestWizard_UntickedSiblingBecomesExclusion: unticking one of four packages
// still writes a glob, with the odd one out named in exclude_locations.
func TestWizard_UntickedSiblingBecomesExclusion(t *testing.T) {
	items := append(siblingItems(), WizardItem{
		Location:    config.Location{Name: "legacy", Location: "packages/legacy", Types: config.Types{"npm"}},
		TypeOptions: []string{"npm"},
		Detected:    true,
	})
	m := NewWizardModel(items)
	m.toggle(4) // untick packages/legacy
	m = confirmed(m)

	locs := m.SelectedLocations()
	if got := locationPaths(locs); strings.Join(got, ",") != "infra,packages/*" {
		t.Fatalf("locations = %v, want the glob", got)
	}
	if got := locs[1].ExcludeLocations; len(got) != 1 || got[0] != "legacy" {
		t.Errorf("exclude_locations = %v, want [legacy]", got)
	}
}

// TestWizard_HelpLineShowsGlobState: the toggle is only useful if it's on the
// help line, and it has to say which way it's currently set.
func TestWizard_HelpLineShowsGlobState(t *testing.T) {
	m := NewWizardModel(siblingItems())
	if help := m.helpLine(); !strings.Contains(help, "^g") || !strings.Contains(help, "globs on") {
		t.Errorf("help = %q, want a ^g entry reading 'globs on'", help)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	if help := updated.(WizardModel).helpLine(); !strings.Contains(help, "globs off") {
		t.Errorf("help = %q, want it to read 'globs off'", help)
	}
}
