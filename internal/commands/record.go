package commands

import (
	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/history"
)

// RecordExecution notes that command was run under displayName in the history of
// the project containing the working directory. History is a convenience, so any
// failure (an unreadable store, say) is ignored rather than reported.
func RecordExecution(displayName, command string) {
	hist, err := history.LoadOrCreateHistory(config.FindProjectRoot("."))
	if err != nil {
		return
	}
	if err := hist.RecordExecution(displayName, command); err != nil {
		return
	}
	_ = hist.SaveToDefaultLocation()
}
