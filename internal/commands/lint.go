package commands

import (
	"fmt"
	"strings"

	"github.com/martinhrvn/paleta/internal/config"
)

// FixConfigFile repairs the .pltrc at path in place — out-of-charset location and
// command names have their offending characters replaced with '_', deprecated
// keys are renamed to their current spellings — and reports what changed.
// Comments and structure survive (see config.File). When nothing needs fixing
// the file is left untouched.
func FixConfigFile(path string) ([]config.NameFix, error) {
	file, err := config.OpenFile(path)
	if err != nil {
		return nil, err
	}
	fixes := file.Fix()
	if len(fixes) == 0 {
		return nil, nil
	}
	if err := file.Save(); err != nil {
		return nil, err
	}
	return fixes, nil
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
