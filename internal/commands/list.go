package commands

import (
	"fmt"

	"github.com/martinhrvn/paleta/internal/config"
)

// ListCommands renders the palette as "location:command" lines, tools last.
func ListCommands(cfg *config.Config) []string {
	var lines []string
	for _, row := range cfg.Rows(false) {
		switch {
		case len(row.Pending) > 0:
			continue
		case row.IsTool:
			lines = append(lines, row.Label())
		default:
			entry := row.DisplayName + ":" + row.CommandLabel()
			// Surface an unresolved reference as an error entry rather than
			// silently listing (or expanding) a command that won't run.
			if row.Error != "" {
				entry += "  ⚠ error: " + row.Error
			}
			lines = append(lines, entry)
		}
	}
	return lines
}

// FormatForFzf renders the palette as "[location] command" lines; tools are
// grouped under their tool name like a location.
func FormatForFzf(cfg *config.Config) []string {
	var lines []string
	for _, row := range cfg.Rows(false) {
		if len(row.Pending) > 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("[%s] %s", row.DisplayName, row.CommandLabel()))
	}
	return lines
}
