package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// aliasTokenRe matches a command reference: `@project`, an optional `[type]`,
// then `:command`. The leading group anchors `@` to a token boundary (start of
// string or a whitespace/shell-operator char) so it never matches things like
// `git@github.com:user/repo` or `npm i @scope/pkg`. The project charset allows
// `/` so a clashing folder name can be disambiguated by a path tail
// (`@packages/search:build`). The command charset includes `:` so npm-style names
// such as `test:watch` resolve (the type is only ever inside `[...]`, so the
// first `:` after the project/type starts the command).
var aliasTokenRe = regexp.MustCompile(`(^|[\s&|;()])@([A-Za-z0-9._/-]+)(?:\[([A-Za-z0-9._-]+)\])?:([A-Za-z0-9][A-Za-z0-9._:-]*)`)

// refCommandRe matches the full command-name charset a reference token allows
// after ':'. A name outside it (e.g. one containing spaces like "test ui")
// cannot be referenced and must fall back to its raw command string.
var refCommandRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)

// refProjectRe matches the project-reference charset (before the optional
// [type]); '/' is allowed so a clashing folder name can be disambiguated by a
// path tail (`packages/search`).
var refProjectRe = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

// indexedLoc holds a location together with a snapshot of its original commands,
// so recursive expansion always reads authored source rather than partially
// rewritten output.
type indexedLoc struct {
	loc      *Location
	commands []Command
}

// aliasIndex resolves a (project, type, command) reference to a concrete command.
// A project is matched by authored name first, then by the folder's base name,
// then by a path tail (`packages/search`) for disambiguating clashing basenames.
// A reference that still matches more than one location is a hard error rather
// than a silent pick.
type aliasIndex struct {
	byName map[string][]*indexedLoc
	byBase map[string][]*indexedLoc
	all    []*indexedLoc
}

func newAliasIndex(cfg *Config) *aliasIndex {
	idx := &aliasIndex{
		byName: make(map[string][]*indexedLoc, len(cfg.Locations)),
		byBase: make(map[string][]*indexedLoc, len(cfg.Locations)),
		all:    make([]*indexedLoc, 0, len(cfg.Locations)),
	}
	for i := range cfg.Locations {
		loc := &cfg.Locations[i]
		snapshot := make([]Command, len(loc.Commands))
		copy(snapshot, loc.Commands)
		il := &indexedLoc{loc: loc, commands: snapshot}
		idx.all = append(idx.all, il)

		if loc.Name != "" {
			idx.byName[loc.Name] = append(idx.byName[loc.Name], il)
		}
		if base := filepath.Base(loc.Location); base != "" && base != "." && base != "/" {
			idx.byBase[base] = append(idx.byBase[base], il)
		}
	}
	return idx
}

// lookupProject matches a project reference by name, then folder base name, then
// path tail. Each level errors if it matches more than one location.
func (idx *aliasIndex) lookupProject(project string) (*indexedLoc, error) {
	if ils := idx.byName[project]; len(ils) == 1 {
		return ils[0], nil
	} else if len(ils) > 1 {
		return nil, fmt.Errorf("project %q is ambiguous: %d locations share that name; disambiguate with a path like @<dir>/%s:<command>", project, len(ils), project)
	}
	if ils := idx.byBase[project]; len(ils) == 1 {
		return ils[0], nil
	} else if len(ils) > 1 {
		return nil, fmt.Errorf("project %q is ambiguous: %d locations share that folder name; disambiguate with a path like @<parent>/%s:<command>", project, len(ils), project)
	}
	if ils := idx.matchByPathTail(project); len(ils) == 1 {
		return ils[0], nil
	} else if len(ils) > 1 {
		return nil, fmt.Errorf("project path %q is ambiguous: matches %d locations", project, len(ils))
	}
	return nil, fmt.Errorf("unknown project %q (reference by name or a path tail like packages/%s)", project, project)
}

// matchByPathTail returns locations whose absolute path ends with the given
// slash-separated path components (e.g. "packages/search").
func (idx *aliasIndex) matchByPathTail(project string) []*indexedLoc {
	want := splitPath(project)
	if len(want) == 0 {
		return nil
	}
	var matches []*indexedLoc
	for _, il := range idx.all {
		if hasPathSuffix(splitPath(il.loc.Location), want) {
			matches = append(matches, il)
		}
	}
	return matches
}

// splitPath splits a slash path into non-empty components.
func splitPath(p string) []string {
	var out []string
	for _, part := range strings.Split(filepath.ToSlash(p), "/") {
		if part != "" && part != "." {
			out = append(out, part)
		}
	}
	return out
}

// hasPathSuffix reports whether path ends with the components of suffix.
func hasPathSuffix(path, suffix []string) bool {
	if len(suffix) == 0 || len(suffix) > len(path) {
		return false
	}
	off := len(path) - len(suffix)
	for i := range suffix {
		if path[off+i] != suffix[i] {
			return false
		}
	}
	return true
}

// resolve finds the single command matching a reference, or returns a clear
// error (unknown/ambiguous project, unknown command, or ambiguous multi-type).
// When a bare (typeless) reference matches several commands but exactly one is
// authored (Type==""), that authored command wins — an explicitly authored
// command owns its bare name, and auto-generated duplicates are reached with
// @project[type]:command.
func (idx *aliasIndex) resolve(project, typ, command string) (*indexedLoc, *Command, error) {
	il, err := idx.lookupProject(project)
	if err != nil {
		return nil, nil, err
	}

	var matches []*Command
	for i := range il.commands {
		c := &il.commands[i]
		if c.Name != command {
			continue
		}
		if typ == "" || c.Type == typ {
			matches = append(matches, c)
		}
	}

	switch len(matches) {
	case 0:
		if typ != "" {
			return nil, nil, fmt.Errorf("project %q has no %s command %q", project, typ, command)
		}
		return nil, nil, fmt.Errorf("project %q has no command %q", project, command)
	case 1:
		return il, matches[0], nil
	default:
		// A bare reference prefers a single authored (Type=="") command over
		// auto-generated typed duplicates — an explicitly authored command owns its
		// bare name; typed duplicates are reached with @project[type]:command.
		if typ == "" {
			var authored []*Command
			for _, m := range matches {
				if m.Type == "" {
					authored = append(authored, m)
				}
			}
			if len(authored) == 1 {
				return il, authored[0], nil
			}
		}
		return nil, nil, fmt.Errorf("command %q in project %q is ambiguous; disambiguate with @%s[<type>]:%s", command, project, project, command)
	}
}

// expandString replaces every reference token in s. home is the directory of the
// command being expanded; a reference to a different project is wrapped in a
// `(cd '<dir>' && ...)` subshell so it runs in the right place without leaking
// the cd. visiting guards against reference cycles.
func (idx *aliasIndex) expandString(s, home string, visiting map[string]bool) (string, error) {
	if !strings.Contains(s, "@") {
		return s, nil
	}
	spans := aliasTokenRe.FindAllStringSubmatchIndex(s, -1)
	if len(spans) == 0 {
		return s, nil
	}

	var b strings.Builder
	last := 0
	for _, m := range spans {
		fullStart, fullEnd := m[0], m[1]
		project := s[m[4]:m[5]]
		typ := ""
		if m[6] >= 0 {
			typ = s[m[6]:m[7]]
		}
		command := s[m[8]:m[9]]

		// Everything up to the token, then the preserved boundary char (group 1).
		b.WriteString(s[last:fullStart])
		if m[3] > m[2] {
			b.WriteString(s[m[2]:m[3]])
		}

		il, cmd, err := idx.resolve(project, typ, command)
		if err != nil {
			return "", err
		}

		key := il.loc.Location + "\x00" + cmd.Name
		if visiting[key] {
			return "", fmt.Errorf("reference cycle at @%s:%s", project, command)
		}
		visiting[key] = true
		expanded, err := idx.expandString(cmd.Command, il.loc.Location, visiting)
		delete(visiting, key)
		if err != nil {
			return "", err
		}

		if il.loc.Location != home {
			expanded = "(cd '" + il.loc.Location + "' && " + expanded + ")"
		}
		b.WriteString(expanded)
		last = fullEnd
	}
	b.WriteString(s[last:])
	return b.String(), nil
}

// expandCommandAliases rewrites every command string that contains reference
// tokens into its resolved form. It runs after processProjectTypes, when every
// command has a resolved string, Name, and Type, and every location path is
// absolute. An unresolvable reference is recorded on the command (Command.Error)
// and skipped so one bad reference never blocks the rest; the first such error
// is also returned for callers that want to fail hard (LoadConfig does not).
func expandCommandAliases(cfg *Config) error {
	idx := newAliasIndex(cfg)
	var firstErr error
	for i := range cfg.Locations {
		home := cfg.Locations[i].Location
		for j := range cfg.Locations[i].Commands {
			cfg.Locations[i].Commands[j].Error = ""
			orig := cfg.Locations[i].Commands[j].Command
			if !strings.Contains(orig, "@") {
				continue
			}
			expanded, err := idx.expandString(orig, home, map[string]bool{})
			if err != nil {
				wrapped := fmt.Errorf("%s: %w", commandErrLabel(&cfg.Locations[i], cfg.Locations[i].Commands[j]), err)
				cfg.Locations[i].Commands[j].Error = wrapped.Error()
				if firstErr == nil {
					firstErr = wrapped
				}
				continue
			}
			cfg.Locations[i].Commands[j].Command = expanded
		}
	}
	return firstErr
}

// ReferenceToken returns a reference token (@project[type]:name) that resolves
// unambiguously back to the command named `name` (project type `typ`) in the
// location at absolute path `dir`, or ok=false when no such token exists: an
// unnamed command or a name outside the token charset (e.g. with spaces). When a
// name is shared by an authored (untyped) command and auto-generated typed ones,
// the authored command is referenced bare (@project:name) and each typed one via
// @project[type]:name. The token is verified with the same resolver the loader
// uses, so it can never expand to a different command than intended — callers
// fall back to the raw command when ok=false.
func (cfg *Config) ReferenceToken(dir, name, typ string) (string, bool) {
	if !refCommandRe.MatchString(name) {
		return "", false
	}
	idx := newAliasIndex(cfg)

	var target *indexedLoc
	for _, il := range idx.all {
		if il.loc.Location == dir {
			target = il
			break
		}
	}
	if target == nil {
		return "", false
	}

	// Prefer a bare reference; only reach for [type] when the bare one is
	// ambiguous. typ=="" (a manually authored command) only ever tries bare.
	types := []string{""}
	if typ != "" {
		types = append(types, typ)
	}

	for _, project := range projectRefCandidates(target.loc) {
		if !refProjectRe.MatchString(project) {
			continue
		}
		if il, err := idx.lookupProject(project); err != nil || il != target {
			continue
		}
		for _, t := range types {
			_, cmd, err := idx.resolve(project, t, name)
			if err != nil || cmd.Type != typ {
				continue
			}
			tok := "@" + project
			if t != "" {
				tok += "[" + t + "]"
			}
			return tok + ":" + name, true
		}
	}
	return "", false
}

// projectRefCandidates lists ways to refer to a location, cleanest first:
// authored name, folder base name, then growing path tails. The caller keeps the
// first candidate that resolves back to the same location.
func projectRefCandidates(loc *Location) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	add(loc.Name)
	parts := splitPath(loc.Location)
	for n := 1; n <= len(parts); n++ {
		add(strings.Join(parts[len(parts)-n:], "/"))
	}
	return out
}

// ExpandCheck reports whether the command string s expands cleanly when stored
// as a command in the location at absolute path home. It runs the same
// resolution the loader uses without mutating anything, so callers can warn
// before persisting a chain that would fail to load.
func (cfg *Config) ExpandCheck(s, home string) error {
	idx := newAliasIndex(cfg)
	_, err := idx.expandString(s, home, map[string]bool{})
	return err
}

// commandErrLabel builds a human-friendly "project/command" prefix for errors.
func commandErrLabel(loc *Location, cmd Command) string {
	proj := loc.Name
	if proj == "" {
		proj = filepath.Base(loc.Location)
	}
	name := cmd.Name
	if name == "" {
		name = cmd.Command
	}
	return fmt.Sprintf("command %q in project %q", name, proj)
}
