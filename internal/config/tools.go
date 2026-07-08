package config

import (
	_ "embed"
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
)

// builtinToolsYAML holds the default tool definitions shipped with paleta. A
// project references any of them by name; a same-named tool in the global config
// overrides the built-in.
//
//go:embed builtin_tools.yaml
var builtinToolsYAML []byte

// ToolsField is the authored value of a `tools:` key. The same key carries two
// shapes: a YAML sequence of names (in a project's .pltrc — the set of tools to
// enable) or a mapping of name -> definition (in the global config — where tools
// are defined). Mirrors the scalar-or-list pattern used by Types and commandField.
type ToolsField struct {
	Enabled []string                  // from a sequence: tool names to enable
	Defs    map[string]ToolDefinition // from a mapping: tool definitions
}

// UnmarshalYAML accepts either a sequence of names or a mapping of definitions.
func (t *ToolsField) UnmarshalYAML(value *yaml.Node) error {
	var names []string
	if err := value.Decode(&names); err == nil {
		t.Enabled = names
		return nil
	}

	var defs map[string]ToolDefinition
	if err := value.Decode(&defs); err != nil {
		return err
	}
	t.Defs = defs
	return nil
}

// MarshalYAML emits the sequence form when tools are enabled by name and the
// mapping form when definitions are present, so a Config round-trips to the same
// shape it was authored in (and never leaks the internal struct fields).
func (t ToolsField) MarshalYAML() (any, error) {
	if len(t.Enabled) > 0 {
		return t.Enabled, nil
	}
	if len(t.Defs) > 0 {
		return t.Defs, nil
	}
	return nil, nil
}

// ToolDefinition describes how an enabled tool runs. It has two shapes: a single
// `command` (one selector row) or a list of named `commands` (one row each). Both
// reuse existing types so a tool command behaves like any other command.
type ToolDefinition struct {
	Command  commandField      `yaml:"command"`
	Commands []Command         `yaml:"commands"`
	Env      map[string]string `yaml:"env,omitempty"`
}

// ResolvedTool is one selector row produced from an enabled tool. It is a leaf
// value (no ui/commands types) so both packages can render it without an import
// cycle. It carries everything the SelectionResult pipeline needs.
type ResolvedTool struct {
	Tool      string            // tool name, e.g. "docker"
	Display   string            // full row label, e.g. "docker: up" or "lazygit"
	Command   string            // the shell command to run
	Env       map[string]string // resolved environment variables
	Directory string            // working directory (the user's cwd)
}

var (
	builtinToolsOnce sync.Once
	builtinToolsDefs map[string]ToolDefinition
)

// builtinTools parses the embedded default tool definitions once.
func builtinTools() map[string]ToolDefinition {
	builtinToolsOnce.Do(func() {
		var wrapper struct {
			Tools ToolsField `yaml:"tools"`
		}
		if err := yaml.Unmarshal(builtinToolsYAML, &wrapper); err != nil {
			// The file is embedded and covered by tests; a parse failure means a
			// developer broke it. Fail loud rather than silently shipping no tools.
			panic(fmt.Sprintf("paleta: invalid builtin_tools.yaml: %v", err))
		}
		builtinToolsDefs = wrapper.Tools.Defs
	})
	return builtinToolsDefs
}

// AttachTools resolves the config's enabled tools into cfg.ResolvedTools, ready
// for the list surfaces to render at the end of the command list. The registry is
// the built-in defaults overlaid with any global-config definitions (cfg.ToolDefs),
// so a global tool overrides a built-in of the same name. workdir is the directory
// each tool runs in (the user's current directory). Unknown or empty tools are
// recorded as non-fatal warnings and skipped, so a bad name never blocks the rest.
//
// It is deliberately separate from LoadConfig: during load the process cwd is the
// config directory, not the user's, so the caller resolves tools once cwd is known.
func AttachTools(cfg *Config, workdir string) {
	cfg.ResolvedTools = nil
	if len(cfg.Tools.Enabled) == 0 {
		return
	}

	registry := make(map[string]ToolDefinition, len(builtinTools())+len(cfg.ToolDefs))
	for name, def := range builtinTools() {
		registry[name] = def
	}
	for name, def := range cfg.ToolDefs {
		registry[name] = def
	}

	for _, name := range cfg.Tools.Enabled {
		def, ok := registry[name]
		if !ok {
			cfg.Warnings = append(cfg.Warnings, Warning{
				Kind:    "tool",
				Scope:   "tool",
				Context: name,
				Name:    name,
				Reason:  "unknown tool (not built in and not defined in global config)",
			})
			continue
		}
		cfg.ResolvedTools = append(cfg.ResolvedTools, resolveTool(name, def, workdir)...)
	}
}

// resolveTool turns a single tool definition into its selector rows. A tool with
// a `commands` list yields one row per command ("tool: label"); otherwise its
// single `command` yields one row labeled with the tool name. A definition with
// neither runs nothing (the caller has already ensured the tool exists).
func resolveTool(name string, def ToolDefinition, workdir string) []ResolvedTool {
	toolEnv := Location{Env: def.Env}

	if len(def.Commands) > 0 {
		rows := make([]ResolvedTool, 0, len(def.Commands))
		for _, cmd := range def.Commands {
			label := cmd.Name
			if label == "" {
				label = cmd.Command
			}
			rows = append(rows, ResolvedTool{
				Tool:      name,
				Display:   name + ": " + label,
				Command:   cmd.Command,
				Env:       EffectiveEnv(toolEnv, cmd),
				Directory: workdir,
			})
		}
		return rows
	}

	if def.Command.joined != "" {
		return []ResolvedTool{{
			Tool:      name,
			Display:   name,
			Command:   def.Command.joined,
			Env:       EffectiveEnv(toolEnv, Command{}),
			Directory: workdir,
		}}
	}

	return nil
}
