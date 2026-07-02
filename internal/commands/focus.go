package commands

import (
	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/ui"
)

// locationFocusKey derives the stable identity used to persist a location's
// focus state. It matches on the authored name when present, otherwise the
// authored path (root normalizes to "."), so the same key is produced when the
// picker lists entries and when SetFocused writes them back.
func locationFocusKey(loc config.Location) string {
	if loc.Name != "" {
		return loc.Name
	}
	if loc.Location == "" || loc.Location == "." {
		return "."
	}
	return loc.Location
}

// FocusEntries reads the authored .pltrc and returns one entry per location for
// the focus picker, carrying each location's current focused state.
func FocusEntries(configPath string) ([]ui.FocusEntry, error) {
	authored, err := LoadAuthoredConfig(configPath)
	if err != nil {
		return nil, err
	}
	if authored == nil {
		return nil, nil
	}

	entries := make([]ui.FocusEntry, 0, len(authored.Locations))
	for _, loc := range authored.Locations {
		key := locationFocusKey(loc)
		label := key
		if label == "." {
			label = "(root)"
		}
		entries = append(entries, ui.FocusEntry{
			Key:     key,
			Label:   label,
			Focused: loc.Focused,
		})
	}
	return entries, nil
}

// SetFocused persists the focus set to the authored .pltrc. focused maps each
// location key (see locationFocusKey) to its desired state; keys absent from the
// map are left unchanged. The authored config is round-tripped through
// GenerateConfig so hand-written locations, commands, and frecency settings are
// preserved to the same extent as `plt init`.
func SetFocused(configPath string, focused map[string]bool) error {
	authored, err := LoadAuthoredConfig(configPath)
	if err != nil {
		return err
	}
	if authored == nil {
		return nil
	}

	for i := range authored.Locations {
		key := locationFocusKey(authored.Locations[i])
		if state, ok := focused[key]; ok {
			authored.Locations[i].Focused = state
		}
	}

	content := GenerateConfig(authored.Locations, authored)
	return WriteConfig(configPath, content)
}
