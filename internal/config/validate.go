package config

import "strings"

// Warning describes a non-fatal config issue to surface to the user (the selector
// banner, `plt lint`). Two kinds exist:
//   - Kind "name": a location/command name outside the alias-safe charset, so it
//     can't be referenced with an @project:command token.
//   - Kind "alias": a command whose @project:command reference could not be
//     resolved (recorded on Command.Error by expandCommandAliases).
type Warning struct {
	Kind    string // "name" or "alias"
	Scope   string // "location" or "command"
	Context string // human label, e.g. "root: test ui"
	Name    string // the offending name (name issues only)
	Reason  string // human explanation / underlying error message
}

// collectConfigWarnings flags every location/command whose (non-empty) name
// falls outside the alias-safe charset (setting a per-entry NameError), and
// gathers those together with any unresolved-alias errors (already recorded on
// Command.Error by expandCommandAliases) into cfg.Warnings. Names are the only
// thing charset-checked — a location *path* may legitimately contain '*' (a
// glob) and is never flagged. Empty names are skipped.
func collectConfigWarnings(cfg *Config) {
	cfg.Warnings = nil
	for i := range cfg.Locations {
		loc := &cfg.Locations[i]
		locLabel := loc.Name
		if locLabel == "" {
			locLabel = loc.Location
		}

		if loc.Name != "" && !refProjectRe.MatchString(loc.Name) {
			reason := nameReason(loc.Name)
			loc.NameError = reason
			cfg.Warnings = append(cfg.Warnings, Warning{
				Kind:    "name",
				Scope:   "location",
				Context: locLabel,
				Name:    loc.Name,
				Reason:  reason,
			})
		}

		for j := range loc.Commands {
			cmd := &loc.Commands[j]
			cmdLabel := locLabel
			if cmd.Name != "" {
				cmdLabel = locLabel + ": " + cmd.Name
			}

			if cmd.Name != "" && !refCommandRe.MatchString(cmd.Name) {
				reason := nameReason(cmd.Name)
				cmd.NameError = reason
				cfg.Warnings = append(cfg.Warnings, Warning{
					Kind:    "name",
					Scope:   "command",
					Context: cmdLabel,
					Name:    cmd.Name,
					Reason:  reason,
				})
			}

			// Unresolved @project:command reference recorded during alias
			// expansion. The error message already carries command/project context.
			if cmd.Error != "" {
				cfg.Warnings = append(cfg.Warnings, Warning{
					Kind:    "alias",
					Scope:   "command",
					Context: cmdLabel,
					Reason:  cmd.Error,
				})
			}
		}
	}
}

// nameReason describes, for a human, why a name is not alias-safe, based on its
// first offending character.
func nameReason(name string) string {
	switch {
	case strings.Contains(name, " "):
		return "contains a space"
	case strings.Contains(name, "*"):
		return "contains '*'"
	default:
		return "has characters not allowed in aliases"
	}
}
