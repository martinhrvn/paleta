package config

import "strings"

// Warning describes a non-fatal config issue to surface to the user (the selector
// banner, `plt lint`). Four kinds exist:
//   - Kind "name": a location/command name outside the alias-safe charset, so it
//     can't be referenced with an @project:command token.
//   - Kind "alias": a command whose @project:command reference could not be
//     resolved (recorded on Command.Error by expandCommandAliases).
//   - Kind "deprecated": a key authored with a superseded spelling (e.g.
//     `include:` for `include_commands:`), recorded at decode time.
//   - Kind "ignored": an authored key that has no effect where it was written
//     (e.g. `overrides:` on a non-glob location, or an override key matching no
//     expanded folder).
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
	cfg.Warnings = append([]Warning(nil), cfg.pendingWarnings...)
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

// collectDeprecatedKeyWarnings records one warning per deprecated key spelling
// found while decoding (see Location.UnmarshalYAML). It runs on the authored
// locations, before glob expansion, so a legacy key on a glob pattern is reported
// once rather than once per expanded folder.
func collectDeprecatedKeyWarnings(cfg *Config) {
	for i := range cfg.Locations {
		loc := &cfg.Locations[i]
		label := loc.Name
		if label == "" {
			label = loc.Location
		}
		for _, key := range loc.LegacyKeys {
			cfg.pendingWarnings = append(cfg.pendingWarnings, Warning{
				Kind:    "deprecated",
				Scope:   "location",
				Context: label,
				Name:    key,
				Reason:  deprecatedKeyReason(key),
			})
		}
	}
}

// deprecatedKeyReason names the replacement for a deprecated key. Keys inside an
// override are reported by their full path (e.g. "overrides.api.include"), so the
// last segment is what identifies them.
func deprecatedKeyReason(key string) string {
	base := key
	if idx := strings.LastIndex(key, "."); idx >= 0 {
		base = key[idx+1:]
	}
	return "\"" + base + ":\" is deprecated — use \"" + base + "_commands:\" (run 'plt lint --fix')"
}
