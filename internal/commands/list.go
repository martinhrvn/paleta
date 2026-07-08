package commands

import (
	"fmt"
	"strings"

	"github.com/martinhrvn/paleta/internal/config"
)

// ListCommands returns a slice of location:command pairs
func ListCommands(cfg *config.Config) []string {
	var commands []string

	for _, location := range cfg.Locations {
		// Use name if available, otherwise use location path
		displayName := location.Name
		if displayName == "" {
			displayName = location.Location
		}

		// Add each command for this location
		for _, command := range location.Commands {
			cmdDisplay := config.CommandLabel(location, command)
			entry := fmt.Sprintf("%s:%s", displayName, cmdDisplay)
			// Surface an unresolved reference as an error entry rather than
			// silently listing (or expanding) a command that won't run.
			if command.Error != "" {
				entry += "  ⚠ error: " + command.Error
			}
			commands = append(commands, entry)
		}
	}

	// Enabled tools render at the end of the list.
	for _, tool := range cfg.ResolvedTools {
		commands = append(commands, tool.Display)
	}

	return commands
}

// FormatForFzf returns a slice of commands formatted for fzf selection
// Format: [location-or-name] command
func FormatForFzf(cfg *config.Config) []string {
	var commands []string

	for _, location := range cfg.Locations {
		// Use name if available, otherwise use location path
		displayName := location.Name
		if displayName == "" {
			displayName = location.Location
		}

		// Add each command for this location in fzf format
		for _, command := range location.Commands {
			cmdDisplay := config.CommandLabel(location, command)
			commands = append(commands, fmt.Sprintf("[%s] %s", displayName, cmdDisplay))
		}
	}

	// Enabled tools render at the end, grouped by tool name like a location.
	for _, tool := range cfg.ResolvedTools {
		label := strings.TrimPrefix(tool.Display, tool.Tool+": ")
		commands = append(commands, fmt.Sprintf("[%s] %s", tool.Tool, label))
	}

	return commands
}
