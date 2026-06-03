package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/martinhrvn/paleta/internal/config"
)

func wizardItems() []WizardItem {
	return []WizardItem{
		{Location: config.Location{Name: "root", Location: ".", Type: "go"}, Detected: true},
		{Location: config.Location{Name: "web", Location: "packages/web", Type: "npm"}, Detected: true, Configured: true},
		{Location: config.Location{Location: "packages/*"}, Configured: true},
	}
}

func TestWizard_PreselectsDetectedAndConfigured(t *testing.T) {
	m := NewWizardModel(wizardItems())
	for i := range m.items {
		if !m.selected[i] {
			t.Errorf("item %d should be pre-selected (detected or configured)", i)
		}
	}
}

func TestWizard_Toggle(t *testing.T) {
	m := NewWizardModel(wizardItems())
	m.toggle(0)
	if m.selected[0] {
		t.Error("expected item 0 to be deselected after toggle")
	}
	m.toggle(0)
	if !m.selected[0] {
		t.Error("expected item 0 to be selected after second toggle")
	}
}

func TestWizard_ToggleAll(t *testing.T) {
	m := NewWizardModel(wizardItems())
	// All start selected; Ctrl+A should deselect all.
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
	if locs[1].Name != "web" || locs[1].Type != "npm" {
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
