package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/history"
	"github.com/martinhrvn/paleta/internal/ui"
)

// SelectionResult is what the selector hands back for each chosen row. It is an
// alias for ui.SelectionResult so the two packages share one definition and no
// conversion is needed between them.
type SelectionResult = ui.SelectionResult

// RunSelector runs the interactive selector with multi-select support. The
// config's Path (the discovered .pltrc; empty for a global-fallback config)
// enables focus persistence and in-app project adding. When the user requests
// adding projects (Ctrl+N), the init wizard runs and the selector re-enters with
// the reloaded config.
func RunSelector(cfg *config.Config) ([]SelectionResult, error) {
	for {
		model := ui.NewModel(cfg, selectorBackend(cfg))
		results, reinit, err := model.Run()
		if reinit {
			if _, ierr := RunInitWizard(filepath.Dir(cfg.Path), false); ierr != nil {
				return nil, ierr
			}
			if reloaded, rerr := cfg.Reload(); rerr == nil {
				cfg = reloaded
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		// ui.SelectionResult and commands.SelectionResult are the same type, so
		// the model's results can be returned directly with no conversion.
		return results, nil
	}
}

// selectorBackend wires the selector to the rest of the program: the project's
// history (weighted per the config's frecency settings), config reloads, deferred
// type resolution, and — when there is a writable local .pltrc — focus and queue
// persistence. A global-fallback config has no Path, so those stay disabled.
func selectorBackend(cfg *config.Config) ui.Backend {
	be := ui.Backend{
		History:        projectHistory(cfg),
		Reload:         cfg.Reload,
		ResolvePending: config.ResolvePendingTypes,
	}
	configPath := cfg.Path
	if configPath == "" {
		return be
	}
	be.ListFocus = func() ([]ui.FocusEntry, error) { return FocusEntries(configPath) }
	be.SaveFocus = func(focused map[string]bool) error { return SetFocused(configPath, focused) }
	be.SaveQueue = func(displayName, directory, name string, parts []string) error {
		return AddCommandToLocation(configPath, displayName, directory, name, parts)
	}
	// The root location is the one at the config's own directory; cross-folder
	// queue saves land there. Match the absolute form the loaded config uses.
	if abs, err := filepath.Abs(filepath.Dir(configPath)); err == nil {
		be.RootDir = abs
	}
	return be
}

// projectHistory loads the working directory's project history, scored with the
// config's frecency weights. History is optional: nil when it can't be read.
func projectHistory(cfg *config.Config) *history.History {
	hist, err := history.LoadOrCreateHistory(config.FindProjectRoot("."))
	if err != nil {
		return nil
	}
	hist.SetWeights(history.NewWeights(cfg.Frecency.FrequencyWeight, cfg.Frecency.RecencyWeight))
	return hist
}

// AddCommandToLocation appends a command built from the given parts (one segment
// per queued command) to the location identified by displayName in the authored
// .pltrc, editing the file in place. A single part saves as a scalar; several
// save as a YAML list (joined with " && " for execution). directory is the
// resolved location path, used to match an authored entry or — when no entry
// matches (e.g. the display name came from a glob) — to create a new location.
func AddCommandToLocation(configPath, displayName, directory, name string, parts []string) error {
	file, err := config.OpenFile(configPath)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("no config file at %s", configPath)
	}
	if err != nil {
		return err
	}
	authored, err := file.Authored()
	if err != nil {
		return err
	}

	newCmd := config.NewCommand(name, parts)
	if idx := findAuthoredLocation(authored, displayName, directory); idx >= 0 {
		err = file.AppendCommand(idx, newCmd)
	} else {
		err = file.AddLocation(config.Location{
			Name:     displayName,
			Location: directory,
			Commands: []config.Command{newCmd},
		})
	}
	if err != nil {
		return err
	}
	return file.Save()
}

// findAuthoredLocation returns the index of the authored location matching the
// display name, falling back to a directory match; -1 when none match.
func findAuthoredLocation(cfg *config.Config, displayName, directory string) int {
	for i, loc := range cfg.Locations {
		if loc.DisplayName() == displayName || (directory != "" && loc.Location == directory) {
			return i
		}
	}
	return -1
}

// selectionJSON is the wire format `plt select` prints for the shell wrapper.
// The action field is present only when it carries meaning (edit / pane); the
// default execute action stays absent for backward compatibility.
type selectionJSON struct {
	Directory   string            `json:"directory"`
	Command     string            `json:"command"`
	DisplayName string            `json:"display_name"`
	Action      string            `json:"action,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

func toSelectionJSON(r SelectionResult) selectionJSON {
	j := selectionJSON{
		Directory:   r.Directory,
		Command:     r.Command,
		DisplayName: r.DisplayName,
		Env:         r.Env,
	}
	if r.Action == "edit" || r.Action == "pane" {
		j.Action = r.Action
	}
	return j
}

// MarshalSelection renders selection results as JSON: a single result as an
// object (what the shell wrapper has always read), several as an array.
func MarshalSelection(results []SelectionResult) ([]byte, error) {
	if len(results) == 1 {
		return json.Marshal(toSelectionJSON(results[0]))
	}
	arr := make([]selectionJSON, len(results))
	for i, r := range results {
		arr[i] = toSelectionJSON(r)
	}
	return json.Marshal(arr)
}
