package config

// Deferred project types.
//
// Most project types report their commands for free: npm reads package.json, go
// returns a fixed list, docker has base commands only, compose adds a set per
// variant file from a directory listing. A few —
// make, python, gradle, maven, and any user-defined type with a `parser_command`
// — have to run a shell command in the project directory to find out, and
// `./gradlew tasks --all` can take seconds. Doing that for every location while
// loading the config means the palette can't open until the slowest project
// answers.
//
// So LoadConfig leaves those types in Location.PendingTypes and callers decide
// when to pay: the selector resolves them in the background and fills rows in as
// they arrive, while `plt list` and friends call ResolveAllPending up front so
// scripts still see a complete list.

// HasPendingTypes reports whether any location still has types left to resolve.
func (c *Config) HasPendingTypes() bool {
	for i := range c.Locations {
		if len(c.Locations[i].PendingTypes) > 0 {
			return true
		}
	}
	return false
}

// ResolvePendingTypes runs the deferred types for one location and returns the
// command list that location should now have, plus any non-fatal warnings. The
// location itself is not modified — the caller applies the result, which lets the
// selector merge results as they arrive. Returns (nil, nil) when nothing is
// pending.
//
// It reads only the location value it is given and uses absolute paths, so
// several locations can be resolved concurrently.
func ResolvePendingTypes(loc Location) ([]Command, []Warning) {
	if len(loc.PendingTypes) == 0 {
		return nil, nil
	}

	var late []Command
	var warnings []Warning
	for _, typeName := range loc.PendingTypes {
		cmds, degraded, err := commandsForType(typeName, loc.Location)
		if err != nil {
			// An invalid type name is a load-time error, and the load already
			// succeeded — report it rather than dropping it.
			warnings = append(warnings, Warning{
				Kind:    "parser",
				Scope:   "location",
				Context: loc.DisplayName(),
				Name:    typeName,
				Reason:  err.Error(),
			})
			continue
		}
		if degraded != nil {
			degraded.Context = loc.DisplayName()
			warnings = append(warnings, *degraded)
		}
		late = append(late, cmds...)
	}

	return composeCommands(loc, late), warnings
}

// ResolveAllPending resolves every location's deferred types in place, then
// re-expands command aliases and rebuilds the warning list so references to a
// late-arriving command (e.g. `@infra[make]:test`) resolve. Use it before any
// non-interactive output, where a complete list matters more than a fast start.
func ResolveAllPending(cfg *Config) {
	if !cfg.HasPendingTypes() {
		return
	}

	for i := range cfg.Locations {
		commands, warnings := ResolvePendingTypes(cfg.Locations[i])
		if commands != nil {
			cfg.Locations[i].Commands = commands
		}
		cfg.Locations[i].PendingTypes = nil
		cfg.loadWarnings = append(cfg.loadWarnings, warnings...)
	}

	// Aliases were expanded during the load, when the deferred commands didn't
	// exist yet; re-run both passes now that they do. Both are in-memory.
	expandCommandAliases(cfg)
	collectConfigWarnings(cfg)
}
