package commands

import (
	"errors"
	"io/fs"
	"sort"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/ui"
)

// FocusEntries reads the authored .pltrc and returns one entry per location for
// the focus picker, carrying each location's current focused state (membership
// in the top-level focus list).
func FocusEntries(configPath string) ([]ui.FocusEntry, error) {
	authored, err := config.LoadAuthored(configPath)
	if err != nil || authored == nil {
		return nil, err
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
// the map are left unchanged. Only the top-level focus list is edited (added
// when true, removed when false); the rest of the file is left as written.
func SetFocused(configPath string, focused map[string]bool) error {
	file, err := config.OpenFile(configPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	authored, err := file.Authored()
	if err != nil {
		return err
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

	if err := file.SetFocused(list); err != nil {
		return err
	}
	return file.Save()
}
