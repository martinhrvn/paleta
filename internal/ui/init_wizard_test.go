package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/martinhrvn/paleta/internal/config"
)

func wizardItems() []WizardItem {
	return []WizardItem{
		{Location: config.Location{Name: "root", Location: ".", Types: config.Types{"go"}}, Detected: true},
		{Location: config.Location{Name: "web", Location: "packages/web", Types: config.Types{"npm"}}, Detected: true, Configured: true},
		{Location: config.Location{Location: "packages/*"}, Configured: true},
	}
}

// configuredFirstItems mirrors the real BuildWizardItems ordering: already
// configured locations on top, newly detected candidates below.
func configuredFirstItems() []WizardItem {
	return []WizardItem{
		{Location: config.Location{Name: "web", Location: "packages/web", Types: config.Types{"npm"}}, Detected: true, Configured: true},
		{Location: config.Location{Location: "packages/*"}, Configured: true},
		{Location: config.Location{Name: "api", Location: "packages/api", Types: config.Types{"go"}}, Detected: true},
		{Location: config.Location{Name: "cli", Location: "packages/cli", Types: config.Types{"go"}}, Detected: true},
	}
}

// TestWizard_PreselectsEverything: the wizard opens with everything ticked, so a
// first run is one keystroke and a repeat run reads as "add the new ones". The
// interaction is unticking what you don't want.
func TestWizard_PreselectsEverything(t *testing.T) {
	m := NewWizardModel(wizardItems())
	for i, it := range m.items {
		if !m.selected[i] {
			t.Errorf("item %d (%s) should be pre-selected", i, it.Location.Location)
		}
	}
}

func TestWizard_CursorStartsAtTop(t *testing.T) {
	m := NewWizardModel(configuredFirstItems())
	if m.cursor != 0 {
		t.Errorf("expected cursor at the top, got %d", m.cursor)
	}
}

// TestWizard_RowMarksNewItems: with everything preselected, the tick no longer
// distinguishes a newly detected project from one already in the config, so the
// row has to say which is which.
func TestWizard_RowMarksNewItems(t *testing.T) {
	m := NewWizardModel(configuredFirstItems())

	configuredRow := m.formatRow(0) // packages/web, already configured
	newRow := m.formatRow(2)        // packages/api, newly detected

	if !strings.Contains(configuredRow, "configured") {
		t.Errorf("configured row = %q, want it marked", configuredRow)
	}
	if strings.Contains(configuredRow, "new") {
		t.Errorf("configured row = %q, should not be marked new", configuredRow)
	}
	if !strings.Contains(newRow, "new") {
		t.Errorf("newly detected row = %q, want it marked new", newRow)
	}
}

func TestWizard_CursorDefaultsToZeroWhenAllSelected(t *testing.T) {
	items := []WizardItem{
		{Location: config.Location{Location: "packages/web"}, Configured: true},
		{Location: config.Location{Location: "packages/*"}, Configured: true},
	}
	m := NewWizardModel(items)
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 when all items selected, got %d", m.cursor)
	}
}

func TestWizard_Toggle(t *testing.T) {
	m := NewWizardModel(wizardItems())
	// Item 1 is configured, so it starts selected.
	m.toggle(1)
	if m.selected[1] {
		t.Error("expected item 1 to be deselected after toggle")
	}
	m.toggle(1)
	if !m.selected[1] {
		t.Error("expected item 1 to be selected after second toggle")
	}
}

func TestWizard_ToggleAll(t *testing.T) {
	m := NewWizardModel(wizardItems())
	// Everything starts selected, so Ctrl+A clears.
	m.toggleAll()
	for i := range m.items {
		if m.selected[i] {
			t.Errorf("item %d should be deselected after toggleAll", i)
		}
	}
	// Ctrl+A again should select all.
	m.toggleAll()
	for i := range m.items {
		if !m.selected[i] {
			t.Errorf("item %d should be selected after second toggleAll", i)
		}
	}
}

func TestWizard_SelectedLocationsAfterConfirm(t *testing.T) {
	m := NewWizardModel(wizardItems())
	m.toggle(2) // everything starts ticked; drop the glob
	m.confirmed = true

	locs := m.SelectedLocations()
	if len(locs) != 2 {
		t.Fatalf("expected 2 selected locations, got %d", len(locs))
	}
	if locs[0].Location != "." || locs[1].Location != "packages/web" {
		t.Errorf("unexpected selection order/content: %+v", locs)
	}
	// Authored fields preserved.
	if locs[1].Name != "web" || len(locs[1].Types) != 1 || locs[1].Types[0] != "npm" {
		t.Errorf("authored fields not preserved: %+v", locs[1])
	}
}

func TestWizard_EnterConfirms(t *testing.T) {
	m := NewWizardModel(wizardItems())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := updated.(WizardModel)
	if !wm.confirmed {
		t.Error("expected Enter to confirm")
	}
	if !wm.quitting {
		t.Error("expected Enter to quit")
	}
}

func TestWizard_EscCancels(t *testing.T) {
	// With an empty search box, Esc cancels the wizard.
	m := NewWizardModel(wizardItems())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	wm := updated.(WizardModel)
	if wm.confirmed {
		t.Error("expected Esc not to confirm")
	}
	if !wm.quitting {
		t.Error("expected Esc with empty search to quit")
	}
}

func TestWizard_EscClearsSearchThenQuits(t *testing.T) {
	m := typeRunes(NewWizardModel(configuredFirstItems()), "api")
	if len(m.filtered) != 1 {
		t.Fatalf("precondition: expected filtered list, got %d", len(m.filtered))
	}

	// First Esc: text present, so clear the search instead of quitting.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(WizardModel)
	if m.quitting {
		t.Error("expected Esc with text to clear search, not quit")
	}
	if m.searchInput.Value() != "" {
		t.Errorf("expected search cleared, got %q", m.searchInput.Value())
	}
	if len(m.filtered) != len(m.items) {
		t.Errorf("expected all %d items restored, got %d", len(m.items), len(m.filtered))
	}

	// Second Esc: search empty, so cancel/quit.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(WizardModel)
	if !m.quitting {
		t.Error("expected second Esc with empty search to quit")
	}
	if m.confirmed {
		t.Error("expected Esc not to confirm")
	}
}

// typeRunes drives the model through a sequence of printable keystrokes.
func typeRunes(m WizardModel, s string) WizardModel {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return updated.(WizardModel)
}

func TestWizard_FuzzyFilter(t *testing.T) {
	m := typeRunes(NewWizardModel(configuredFirstItems()), "api")
	if len(m.filtered) != 1 {
		t.Fatalf("expected 1 match for 'api', got %d", len(m.filtered))
	}
	if got := m.items[m.filtered[0]].Location.Location; got != "packages/api" {
		t.Errorf("expected packages/api to match, got %s", got)
	}
}

func TestWizard_ClearSearch(t *testing.T) {
	m := typeRunes(NewWizardModel(configuredFirstItems()), "api")
	if len(m.filtered) != 1 {
		t.Fatalf("precondition: expected filtered list, got %d", len(m.filtered))
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = updated.(WizardModel)
	if m.searchInput.Value() != "" {
		t.Errorf("expected search cleared, got %q", m.searchInput.Value())
	}
	if len(m.filtered) != len(m.items) {
		t.Errorf("expected all %d items after clear, got %d", len(m.items), len(m.filtered))
	}
}

func TestWizard_TabTogglesAndAdvances(t *testing.T) {
	// Everything starts ticked, so Tab on the cursor row unticks it.
	m := NewWizardModel(configuredFirstItems())
	if m.cursor != 0 {
		t.Fatalf("precondition: expected cursor 0, got %d", m.cursor)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(WizardModel)
	if m.selected[0] {
		t.Error("expected Tab to untick the item under the cursor")
	}
	if m.cursor != 1 {
		t.Errorf("expected Tab to advance cursor to 1, got %d", m.cursor)
	}
}

func TestWizard_SelectionPersistsAcrossFilter(t *testing.T) {
	// Untick the "api" candidate (index 2), then filter it out of view.
	m := NewWizardModel(configuredFirstItems())
	m.toggle(2)
	m = typeRunes(m, "web")
	if len(m.filtered) != 1 {
		t.Fatalf("expected 'web' to narrow to 1 item, got %d", len(m.filtered))
	}
	m.confirmed = true
	locs := m.SelectedLocations()
	for _, l := range locs {
		if l.Location == "packages/api" {
			t.Errorf("expected the filtered-out deselection to persist; got %+v", locs)
		}
	}
}
