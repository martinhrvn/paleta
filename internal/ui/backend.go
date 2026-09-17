package ui

import (
	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/history"
)

// Backend is everything the selector needs from outside the terminal: the
// project's history, how to re-read the configuration, how deferred project
// types get resolved, and the writes a user can trigger from inside it. The
// commands package fills it in against config, history and .pltrc; tests fill
// it in memory. It is the selector's only seam to the rest of the program, so
// the ui package performs no loading or writing of its own.
//
// Every function is optional. A nil History disables frecency and the preview
// statistics; a nil Reload keeps the current configuration after a save; a nil
// ResolvePending leaves placeholder rows in place; nil focus or queue functions
// disable those features (there is no writable local .pltrc).
type Backend struct {
	History        *history.History
	Reload         func() (*config.Config, error)
	ResolvePending func(config.Location) ([]config.Command, []config.Warning)

	// Focus persistence (Ctrl+P).
	ListFocus func() ([]FocusEntry, error)
	SaveFocus func(focused map[string]bool) error

	// Queue saving ("s" in the queue editor). SaveQueue receives the target
	// project's display name and directory, the new command's name, and the
	// command's parts (one per queued command; a single part saves as a scalar,
	// several as a && chain / YAML list). RootDir is the absolute directory of
	// the config's root location: a queue spanning folders is saved there so its
	// per-project parts can cd-wrap; empty falls back to the first command's
	// project.
	SaveQueue func(displayName, directory, name string, parts []string) error
	RootDir   string
}
