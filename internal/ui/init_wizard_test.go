package ui

import (
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

func TestWizard_PreselectsConfiguredOnly(t *testing.T) {
	m := NewWizardModel(wizardItems())
	for i, it := range m.items {
		if it.Configured && !m.selected[i] {
			t.Errorf("configured item %d should be pre-selected", i)
		}
		if !it.Configured && m.selected[i] {
			t.Errorf("non-configured item %d should not be pre-selected", i)
		}
	}
}

func TestWizard_CursorStartsAtFirstUnselected(t *testing.T) {
	m := NewWizardModel(configuredFirstItems())
	if m.cursor != 2 {
		t.Errorf("expected cursor at first unselected item (2), got %d", m.cursor)
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
	// Not all start selected; Ctrl+A should select all.
	m.toggleAll()
	for i := range m.items {
		if !m.selected[i] {
			t.Errorf("item %d should be selected after toggleAll", i)
		}
	}
	// Ctrl+A again should deselect all.
	m.toggleAll()
	for i := range m.items {
		if m.selected[i] {
			t.Errorf("item %d should be deselected after second toggleAll", i)
		}
	}
}

func TestWizard_SelectedLocationsAfterConfirm(t *testing.T) {
	m := NewWizardModel(wizardItems())
	m.toggle(0) // add the detected-only root
	m.toggle(2) // drop the glob
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
	m := NewWizardModel(wizardItems())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	wm := updated.(WizardModel)
	if wm.confirmed {
		t.Error("expected Esc not to confirm")
	}
	if !wm.quitting {
		t.Error("expected Esc to quit")
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

func TestWizard_TabSelectsAndAdvances(t *testing.T) {
	// Cursor starts on the first unselected item (the "api" candidate at index 2).
	m := NewWizardModel(configuredFirstItems())
	if m.cursor != 2 {
		t.Fatalf("precondition: expected cursor 2, got %d", m.cursor)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(WizardModel)
	if !m.selected[2] {
		t.Error("expected Tab to select the item under the cursor")
	}
	if m.cursor != 3 {
		t.Errorf("expected Tab to advance cursor to 3, got %d", m.cursor)
	}
}

func TestWizard_SelectionPersistsAcrossFilter(t *testing.T) {
	// Select the "api" candidate (index 2), then filter it out of view.
	m := NewWizardModel(configuredFirstItems())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(WizardModel)
	m = typeRunes(m, "web")
	if len(m.filtered) != 1 {
		t.Fatalf("expected 'web' to narrow to 1 item, got %d", len(m.filtered))
	}
	m.confirmed = true
	locs := m.SelectedLocations()
	var found bool
	for _, l := range locs {
		if l.Location == "packages/api" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected filtered-out selection to persist; got %+v", locs)
	}
}
