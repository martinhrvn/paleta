package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/martinhrvn/paleta/internal/config"
	"gopkg.in/yaml.v3"
)

// NameFix records a single rewrite performed by FixConfigFile.
type NameFix struct {
	Scope  string // "location", "command", or "key" (a deprecated key renamed)
	Before string
	After  string
}

// deprecatedKeyRenames maps each deprecated location key to its current spelling.
// Applied to locations and to each folder entry under `overrides:`.
var deprecatedKeyRenames = []struct{ before, after string }{
	{"include", "include_commands"},
	{"exclude", "exclude_commands"},
}

// SanitizeName replaces every character outside the alias-safe charset with '_'
// so the name can be used in an @project:command reference. Commands allow
// [A-Za-z0-9._:-]; locations allow [A-Za-z0-9._/-]. Note this does not enforce
// the command leading-character rule (must be alphanumeric); a name whose first
// char was disallowed becomes a leading '_' and is still reported by lint.
func SanitizeName(name string, isCommand bool) string {
	var b strings.Builder
	for _, r := range name {
		if isAlnum(r) || r == '.' || r == '_' || r == '-' ||
			(isCommand && r == ':') || (!isCommand && r == '/') {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('_')
	}
	return b.String()
}

func isAlnum(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

// FixConfigFile repairs the .pltrc at path: out-of-charset location and command
// names have their offending characters replaced with '_', and deprecated keys are
// renamed to their current spellings. It edits the parsed YAML node tree in place
// and re-marshals, so comments and structure survive. When nothing needs fixing the
// file is left untouched. Returns the applied fixes.
func FixConfigFile(path string) ([]NameFix, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, invalidConfig(path, err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, invalidConfig(path, err)
	}

	fixes := sanitizeConfigNode(&root)
	if len(fixes) == 0 {
		return nil, nil
	}

	out, err := yaml.Marshal(&root)
	if err != nil {
		return nil, fmt.Errorf("failed to render config: %w", err)
	}

	perm := os.FileMode(0644)
	if info, statErr := os.Stat(path); statErr == nil {
		perm = info.Mode().Perm()
	}
	if err := os.WriteFile(path, out, perm); err != nil {
		return nil, fmt.Errorf("failed to write config file: %w", err)
	}
	return fixes, nil
}

// sanitizeConfigNode walks locations[].name, locations[].commands[].name and each
// location's deprecated keys in the parsed YAML tree, fixing them in place and
// collecting the changes.
func sanitizeConfigNode(root *yaml.Node) []NameFix {
	doc := root
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		doc = doc.Content[0]
	}
	locs := mapValue(doc, "locations")
	if locs == nil || locs.Kind != yaml.SequenceNode {
		return nil
	}

	var fixes []NameFix
	for _, loc := range locs.Content {
		if before, after, ok := fixNameNode(mapValue(loc, "name"), false); ok {
			fixes = append(fixes, NameFix{Scope: "location", Before: before, After: after})
		}
		fixes = append(fixes, renameDeprecatedKeys(loc)...)
		// Each folder entry under `overrides:` takes the same filter keys.
		if overrides := mapValue(loc, "overrides"); overrides != nil && overrides.Kind == yaml.MappingNode {
			for i := 1; i < len(overrides.Content); i += 2 {
				fixes = append(fixes, renameDeprecatedKeys(overrides.Content[i])...)
			}
		}
		cmds := mapValue(loc, "commands")
		if cmds == nil || cmds.Kind != yaml.SequenceNode {
			continue
		}
		for _, cmd := range cmds.Content {
			if before, after, ok := fixNameNode(mapValue(cmd, "name"), true); ok {
				fixes = append(fixes, NameFix{Scope: "command", Before: before, After: after})
			}
		}
	}
	return fixes
}

// fixNameNode sanitizes a scalar name node in place, reporting whether it
// changed. A nil or non-scalar node is a no-op.
func fixNameNode(node *yaml.Node, isCommand bool) (before, after string, changed bool) {
	if node == nil || node.Kind != yaml.ScalarNode {
		return "", "", false
	}
	fixed := SanitizeName(node.Value, isCommand)
	if fixed == node.Value {
		return "", "", false
	}
	before = node.Value
	node.Value = fixed
	node.Tag = "!!str"
	node.Style = 0 // let the emitter pick quoting for the new value
	return before, fixed, true
}

// renameDeprecatedKeys rewrites the deprecated filter keys of one mapping node (a
// location or an override entry) to their current spellings, reporting each
// rename.
func renameDeprecatedKeys(m *yaml.Node) []NameFix {
	var fixes []NameFix
	for _, r := range deprecatedKeyRenames {
		if renameKeyNode(m, r.before, r.after) {
			fixes = append(fixes, NameFix{Scope: "key", Before: r.before, After: r.after})
		}
	}
	return fixes
}

// renameKeyNode renames a mapping key in place, reporting whether it changed. The
// key node is mutated rather than replaced so its comments survive. A mapping that
// already carries the new key is left alone: merging two authored keys is the
// user's call, so lint keeps warning instead.
func renameKeyNode(m *yaml.Node, old, new string) bool {
	if m == nil || m.Kind != yaml.MappingNode {
		return false
	}
	if mapValue(m, new) != nil {
		return false
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == old {
			m.Content[i].Value = new
			return true
		}
	}
	return false
}

// mapValue returns the value node for key in a mapping node, or nil.
func mapValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// FormatLintReport renders config warnings for `plt lint`: out-of-charset names,
// unresolved aliases, unknown tools, deprecated keys, and keys with no effect.
// Each kind's guidance is printed only when that kind occurred — the charset
// guidance in particular must not follow a warning `--fix` can't repair that way.
// An empty slice yields a clean-result message.
func FormatLintReport(warnings []config.Warning) string {
	if len(warnings) == 0 {
		return "No issues found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d issue(s) found:\n", len(warnings))

	nameIssues, aliasIssues, toolIssues := 0, 0, 0
	deprecatedIssues, ignoredIssues, parserIssues := 0, 0, 0
	for _, w := range warnings {
		switch w.Kind {
		case "alias":
			aliasIssues++
			fmt.Fprintf(&b, "  unresolved alias — %s\n", w.Reason)
		case "tool":
			toolIssues++
			fmt.Fprintf(&b, "  unknown tool  %q — %s\n", w.Context, w.Reason)
		case "deprecated":
			deprecatedIssues++
			fmt.Fprintf(&b, "  deprecated key in %q — %s\n", w.Context, w.Reason)
		case "ignored":
			ignoredIssues++
			fmt.Fprintf(&b, "  ignored %-6s in %q — %s\n", w.Name, w.Context, w.Reason)
		case "parser":
			parserIssues++
			fmt.Fprintf(&b, "  parser failed in %q — %s\n", w.Context, w.Reason)
		default:
			nameIssues++
			fmt.Fprintf(&b, "  %-9s %q — %s\n", w.Scope, w.Context, w.Reason)
		}
	}

	if nameIssues > 0 {
		b.WriteString("\nNames used in @project:command aliases must use only letters, digits and . _ -\n")
		b.WriteString("(commands also allow ':', locations also allow '/').\n")
		b.WriteString("Run 'plt lint --fix' to replace offending characters with '_'.")
	}
	if aliasIssues > 0 {
		if nameIssues > 0 {
			b.WriteString("\n")
		}
		b.WriteString("\nUnresolved aliases must be fixed by editing .pltrc — the referenced\n")
		b.WriteString("project or command may be renamed, missing, or ambiguous.")
	}
	if toolIssues > 0 {
		if nameIssues > 0 || aliasIssues > 0 {
			b.WriteString("\n")
		}
		b.WriteString("\nEnabled tools must be built in or defined in the global config\n")
		b.WriteString("(~/.config/paleta/config.yaml) under 'tools:'.")
	}
	if deprecatedIssues > 0 {
		if nameIssues > 0 || aliasIssues > 0 || toolIssues > 0 {
			b.WriteString("\n")
		}
		b.WriteString("\nDeprecated keys still work, but should be renamed.\n")
		b.WriteString("Run 'plt lint --fix' to rename them in place (comments are preserved).")
	}
	if ignoredIssues > 0 {
		if nameIssues > 0 || aliasIssues > 0 || toolIssues > 0 || deprecatedIssues > 0 {
			b.WriteString("\n")
		}
		b.WriteString("\n'exclude_locations:' and 'overrides:' only apply to a glob location, and\n")
		b.WriteString("an override key must match a folder the glob expands to.")
	}
	if parserIssues > 0 {
		if nameIssues > 0 || aliasIssues > 0 || toolIssues > 0 || deprecatedIssues > 0 || ignoredIssues > 0 {
			b.WriteString("\n")
		}
		b.WriteString("\nA type whose parser failed still contributes its base commands. Check that\n")
		b.WriteString("the tool runs in that directory (e.g. ./gradlew, mvn, make).")
	}
	return b.String()
}
