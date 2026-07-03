package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/martinhrvn/paleta/internal/config"
	"gopkg.in/yaml.v3"
)

// NameFix records a single name rewrite performed by FixConfigFile.
type NameFix struct {
	Scope  string // "location" or "command"
	Before string
	After  string
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

// FixConfigFile rewrites out-of-charset location and command names in the .pltrc
// at path, replacing offending characters with '_'. It edits the parsed YAML
// node tree in place and re-marshals, so comments and structure survive. When
// nothing needs fixing the file is left untouched. Returns the applied fixes.
func FixConfigFile(path string) ([]NameFix, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
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

// sanitizeConfigNode walks locations[].name and locations[].commands[].name in
// the parsed YAML tree, sanitizing any name in place and collecting the fixes.
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

// FormatLintReport renders config warnings for `plt lint`. It handles both name
// issues (out-of-charset names) and unresolved-alias issues, and only prints the
// charset/`--fix` guidance when there are name issues (--fix repairs names, not
// references). An empty slice yields a clean-result message.
func FormatLintReport(warnings []config.Warning) string {
	if len(warnings) == 0 {
		return "No issues found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d issue(s) found:\n", len(warnings))

	nameIssues, aliasIssues := 0, 0
	for _, w := range warnings {
		switch w.Kind {
		case "alias":
			aliasIssues++
			fmt.Fprintf(&b, "  unresolved alias — %s\n", w.Reason)
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
	return b.String()
}
