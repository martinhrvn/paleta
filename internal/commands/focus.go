package commands

import (
	"sort"

	"github.com/martinhrvn/paleta/internal/ui"
)

// FocusEntries reads the authored .pltrc and returns one entry per location for
// the focus picker, carrying each location's current focused state (membership
// in the top-level focus list).
func FocusEntries(configPath string) ([]ui.FocusEntry, error) {
	authored, err := LoadAuthoredConfig(configPath)
	if err != nil {
		return nil, err
	}
	if authored == nil {
		return nil, nil
	}

	focused := make(map[string]bool, len(authored.Focused))
	for _, key := range authored.Focused {
		focused[key] = true
	}

	entries := make([]ui.FocusEntry, 0, len(authored.Locations))
	for _, loc := range authored.Locations {
		key := loc.FocusKey()
		label := key
		if label == "." {
			label = "(root)"
		}
		entries = append(entries, ui.FocusEntry{
			Key:     key,
			Label:   label,
			Focused: focused[key],
		})
	}
	return entries, nil
}

// SetFocused persists the focus set to the authored .pltrc. focused maps each
// location key (see Location.FocusKey) to its desired state; keys absent from
// the map are left unchanged. The change is applied to the top-level focus list
// (added when true, removed when false) and the authored config is round-tripped
// through GenerateConfig so hand-written locations, commands, and frecency
// settings are preserved to the same extent as `plt init`.
func SetFocused(configPath string, focused map[string]bool) error {
	authored, err := LoadAuthoredConfig(configPath)
	if err != nil {
		return err
	}
	if authored == nil {
		return nil
	}

	set := make(map[string]bool, len(authored.Focused))
	for _, key := range authored.Focused {
		set[key] = true
	}
	for key, state := range focused {
		if state {
			set[key] = true
		} else {
			delete(set, key)
		}
	}

	list := make([]string, 0, len(set))
	for key := range set {
		list = append(list, key)
	}
	sort.Strings(list)
	authored.Focused = list

	content := GenerateConfig(authored.Locations, authored)
	return WriteConfig(configPath, content)
}
