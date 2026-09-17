package config

import "strings"

// DisplayName is the name a location is shown and keyed by: its authored name,
// else its path. It is the location half of a history key and of every row
// label, so every surface derives it here and nowhere else.
func (l Location) DisplayName() string {
	if l.Name != "" {
		return l.Name
	}
	return l.Location
}

// Row is one entry of the palette: a location's command, a placeholder for a
// location whose project types are still resolving (Pending non-empty), or an
// enabled tool (IsTool). It carries everything a list surface needs to show
// the entry and everything the shell needs to run it.
type Row struct {
	DisplayName string // the location's DisplayName, or the tool's name
	Name        string // the command's name; "" when it has none
	Command     string // the shell command; "" for a placeholder
	Directory   string // where the command runs
	Type        string // the project type that produced the command, "" otherwise
	// MultiType is set when the location declares several types, so Type is
	// needed to tell same-named commands apart.
	MultiType bool
	Env       map[string]string
	// Error is set when the command's @project:command reference could not be
	// resolved, so the command will not run as written.
	Error string
	// Invalid is why the row can't be referenced as an alias, "" when it can:
	// the command's name problem, else its unresolved reference, else its
	// location's name problem.
	Invalid string
	IsTool  bool
	Pending []string // for a placeholder: the types still resolving
}

// Rows lists the palette entries in display order: each location's commands,
// then its placeholder if types are still resolving, and finally the enabled
// tools — which are project-global and run in the user's directory, so they
// show regardless of focus. With focusedOnly, locations outside the focus set
// are skipped; when nothing is focused the filter is a no-op rather than an
// empty list.
func (c *Config) Rows(focusedOnly bool) []Row {
	focus := focusedOnly && c.AnyFocused()

	var rows []Row
	for _, loc := range c.Locations {
		if focus && !loc.Focused {
			continue
		}
		display := loc.DisplayName()
		multi := len(loc.Types) > 1

		for _, cmd := range loc.Commands {
			invalid := cmd.NameError
			if invalid == "" {
				invalid = cmd.Error
			}
			if invalid == "" {
				invalid = loc.NameError
			}
			rows = append(rows, Row{
				DisplayName: display,
				Name:        cmd.Name,
				Command:     cmd.Command,
				Directory:   loc.Location,
				Type:        cmd.Type,
				MultiType:   multi,
				Env:         EffectiveEnv(loc, cmd),
				Error:       cmd.Error,
				Invalid:     invalid,
			})
		}

		if len(loc.PendingTypes) > 0 {
			rows = append(rows, Row{
				DisplayName: display,
				Directory:   loc.Location,
				Pending:     append([]string(nil), loc.PendingTypes...),
			})
		}
	}

	for _, tool := range c.ResolvedTools {
		name := ""
		if tool.Display != tool.Tool {
			name = strings.TrimPrefix(tool.Display, tool.Tool+": ")
		}
		rows = append(rows, Row{
			DisplayName: tool.Tool,
			Name:        name,
			Command:     tool.Command,
			Directory:   tool.Directory,
			Env:         tool.Env,
			IsTool:      true,
		})
	}
	return rows
}

// nameOrCommand is the command half of a label: the name, else the command.
func (r Row) nameOrCommand() string {
	if r.Name != "" {
		return r.Name
	}
	if r.Command != "" {
		return r.Command
	}
	return strings.Join(r.Pending, ", ")
}

// CommandLabel is the command half of a row's label, prefixed with "[type] "
// when the location declares several types so same-named commands from
// different types (npm `build` vs docker `build`) stay distinguishable.
func (r Row) CommandLabel() string {
	if r.MultiType && r.Type != "" {
		return "[" + r.Type + "] " + r.nameOrCommand()
	}
	return r.nameOrCommand()
}

// Label is the row's plain text, "display: name-or-command". A tool with a
// single command is labelled by the tool name alone.
func (r Row) Label() string {
	if r.IsTool && r.Name == "" {
		return r.DisplayName
	}
	return r.DisplayName + ": " + r.nameOrCommand()
}
