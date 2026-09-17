package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/martinhrvn/paleta/internal/parsers"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Root      string         `yaml:"root,omitempty"`
	Locations []Location     `yaml:"locations"`
	Frecency  FrecencyConfig `yaml:"frecency,omitempty"`
	// Focused is the authored "focus" set: a list of location focus keys (see
	// Location.FocusKey). When non-empty, the selector defaults to showing only
	// the matching locations. resolveFocus turns this into per-location runtime
	// flags at load; entries matching no location are ignored.
	Focused []string `yaml:"focused,omitempty"`
	// Tools is the authored `tools:` value: a list of tool names to enable (in a
	// project's .pltrc) or a map of tool definitions (in the global config). See
	// ToolsField and tools.go.
	Tools ToolsField `yaml:"tools,omitempty"`
	// ToolDefs holds tool definitions carried over from the global config so the
	// enabled tools in a local config can resolve against them. Populated during
	// discovery/merge, never read from or written to this config's own YAML.
	ToolDefs map[string]ToolDefinition `yaml:"-"`
	// ResolvedTools holds the tool rows to render at the end of the command list,
	// produced by AttachTools once the user's working directory is known. Never
	// serialized.
	ResolvedTools []ResolvedTool `yaml:"-"`
	// Warnings collects non-fatal config issues found during load: names outside
	// the alias-safe charset and unresolved @project:command references. Callers
	// surface them (the selector banner, `plt lint`) rather than failing the load.
	// Never serialized.
	Warnings []Warning `yaml:"-"`
	// Path is the local .pltrc this config was loaded from, when discovery found
	// one. Empty for a global project (which has no single file to rewrite) and
	// for a config loaded directly by path. Never serialized.
	Path string `yaml:"-"`
	// loadOpts remembers how Load assembled this config so Reload can do it again.
	loadOpts LoadOptions
	// baseDir is the directory relative location paths and glob patterns resolve
	// against: the config file's own directory, or Root for a global project.
	baseDir string
	// loadWarnings holds the notices the load stages produce before the final
	// warning pass (deprecated keys, glob-expansion notices, parser degradation,
	// unknown tools). collectConfigWarnings rebuilds Warnings from them plus the
	// name/alias checks, so a rebuild never loses them.
	loadWarnings []Warning
}

// FrecencyConfig configures frecency sorting behavior
type FrecencyConfig struct {
	Enabled         bool    `yaml:"enabled"`
	RecencyWeight   float64 `yaml:"recency_weight"`
	FrequencyWeight float64 `yaml:"frequency_weight"`
}

// DefaultFrecencyConfig returns the default frecency configuration
func DefaultFrecencyConfig() FrecencyConfig {
	return FrecencyConfig{
		Enabled:         true,
		RecencyWeight:   0.5,
		FrequencyWeight: 0.5,
	}
}

type Command struct {
	Name    string            `yaml:"name,omitempty"`
	Command string            `yaml:"command"`
	Env     map[string]string `yaml:"env,omitempty"`
	// Type is the project type that produced this command (e.g. "npm",
	// "compose"). It is set internally by processProjectTypes and is never read
	// from or written to YAML. Empty for manually authored commands.
	Type string `yaml:"-"`
	// Error is set by expandCommandAliases when this command's references can't
	// be resolved (e.g. a referenced command was renamed or an ambiguous saved
	// chain). The command keeps its authored text so one bad reference never
	// blocks loading the rest; callers surface it rather than run it. Never
	// serialized.
	Error string `yaml:"-"`
	// NameError is set by validateNames when this command's Name falls outside the
	// alias-safe charset (e.g. contains a space or '*'), so it can never be used in
	// an @project:command reference. Non-fatal; surfaced by the selector and
	// `plt lint`. Never serialized.
	NameError string `yaml:"-"`
	// parts holds the authored list form when `command` was a YAML sequence, so
	// the command re-marshals as a list. Empty when authored as a scalar. Never
	// serialized directly; consulted by MarshalYAML.
	parts []string `yaml:"-"`
}

// NewCommand builds a Command from one or more shell segments. Multiple non-empty
// segments are joined with " && " for execution and retained so the command
// re-marshals as a YAML list.
func NewCommand(name string, parts []string) Command {
	cleaned := cleanCommandParts(parts)
	return Command{
		Name:    name,
		Command: strings.Join(cleaned, " && "),
		parts:   cleaned,
	}
}

// cleanCommandParts drops empty and whitespace-only segments while preserving
// order and content (commands are arbitrary shell, so no trimming or dedup).
func cleanCommandParts(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// Types is a location's set of project types. In .pltrc the `type` key accepts
// either a single value (`type: npm`) or a list (`type: [npm, docker]`); both
// decode into this slice. A single-element slice re-marshals as a scalar so
// existing configs round-trip unchanged.
type Types []string

// UnmarshalYAML accepts either a scalar string or a sequence of strings.
// Whitespace is trimmed and empty entries are dropped.
func (t *Types) UnmarshalYAML(value *yaml.Node) error {
	var single string
	if err := value.Decode(&single); err == nil {
		*t = normalizeTypes([]string{single})
		return nil
	}

	var list []string
	if err := value.Decode(&list); err != nil {
		return err
	}
	*t = normalizeTypes(list)
	return nil
}

// MarshalYAML emits a scalar for a single type and a sequence for several, so a
// single-type location serializes as `type: npm` rather than `type: [npm]`.
func (t Types) MarshalYAML() (any, error) {
	switch len(t) {
	case 0:
		return nil, nil
	case 1:
		return t[0], nil
	default:
		return []string(t), nil
	}
}

// normalizeTypes trims whitespace, drops empty entries, and de-duplicates while
// preserving first-seen order.
func normalizeTypes(in []string) Types {
	var out Types
	seen := make(map[string]bool)
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// commandField decodes a command's body from either a scalar string or a
// sequence of strings. A sequence is joined with " && " for execution while its
// original parts are retained so the command can be re-marshaled as a list.
type commandField struct {
	joined string
	parts  []string
}

// UnmarshalYAML accepts either a scalar string or a sequence of strings.
func (f *commandField) UnmarshalYAML(value *yaml.Node) error {
	var single string
	if err := value.Decode(&single); err == nil {
		f.joined = single
		return nil
	}

	var list []string
	if err := value.Decode(&list); err != nil {
		return err
	}
	f.parts = cleanCommandParts(list)
	f.joined = strings.Join(f.parts, " && ")
	return nil
}

// UnmarshalYAML implements custom YAML unmarshaling for Command. It supports the
// bare-string format (old), the object format, and — within the object — a
// `command` field that is either a scalar or a list of segments joined with
// " && ".
func (c *Command) UnmarshalYAML(value *yaml.Node) error {
	// Try to unmarshal as a string (old format)
	var cmdString string
	if err := value.Decode(&cmdString); err == nil {
		c.Command = cmdString
		c.Name = "" // No name for old format
		return nil
	}

	// Object format. Decode the command field loosely so it accepts either a
	// scalar or a sequence.
	var obj struct {
		Name    string            `yaml:"name,omitempty"`
		Command commandField      `yaml:"command"`
		Env     map[string]string `yaml:"env,omitempty"`
	}
	if err := value.Decode(&obj); err != nil {
		return err
	}
	c.Name = obj.Name
	c.Env = obj.Env
	c.Command = obj.Command.joined
	c.parts = obj.Command.parts
	return nil
}

// MarshalYAML renders a Command as an object, emitting `command` as a YAML
// sequence when it was authored as a multi-item list so it round-trips; single
// item and scalar-authored commands stay scalar. Type and Error are never
// serialized.
func (c Command) MarshalYAML() (any, error) {
	out := struct {
		Name    string            `yaml:"name,omitempty"`
		Command any               `yaml:"command"`
		Env     map[string]string `yaml:"env,omitempty"`
	}{
		Name: c.Name,
		Env:  c.Env,
	}
	if len(c.parts) > 1 {
		out.Command = c.parts
	} else {
		out.Command = c.Command
	}
	return out, nil
}

type Location struct {
	Name     string    `yaml:"name,omitempty"`
	Location string    `yaml:"location,omitempty"`
	Types    Types     `yaml:"type,omitempty"`
	Commands []Command `yaml:"commands,omitempty"`
	// Include and Exclude filter this location's commands (both authored and
	// type-discovered) by name. Authored as `include_commands:`/`exclude_commands:`;
	// the pre-rename `include:`/`exclude:` spellings still decode (see
	// Location.UnmarshalYAML) and are reported as deprecated.
	Include []string `yaml:"include_commands,omitempty"`
	Exclude []string `yaml:"exclude_commands,omitempty"`
	// ExcludeLocations drops folders from a glob location's expansion. Patterns are
	// matched against each expanded folder's base name with filepath.Match. Only
	// meaningful on a glob location; ignored (with a warning) elsewhere.
	ExcludeLocations []string `yaml:"exclude_locations,omitempty"`
	// Overrides carries per-folder additions/overrides for a glob location, keyed by
	// a pattern matched against each expanded folder's base name. Consumed by glob
	// expansion, so it never appears on an expanded child.
	Overrides map[string]LocationOverride `yaml:"overrides,omitempty"`
	Env       map[string]string           `yaml:"env,omitempty"`
	// Focused is a runtime-only flag set by resolveFocus from the top-level
	// Config.Focused list; it drives the selector's focus filter. It is never
	// read from or written to YAML (focus lives in the top-level list).
	Focused bool `yaml:"-"`
	// NameError is set by validateNames when this location's Name falls outside the
	// alias-safe charset, so it can never be used as a project reference. Non-fatal;
	// surfaced by the selector and `plt lint`. Never serialized.
	NameError string `yaml:"-"`
	// LegacyKeys lists the deprecated key spellings this location was authored with
	// (e.g. "include", "overrides.api.exclude"), in a stable order, so
	// collectDeprecatedKeyWarnings can report them. Set at decode time; never
	// serialized.
	LegacyKeys []string `yaml:"-"`
	// PendingTypes lists this location's declared types whose commands come from
	// running a shell command (make, gradle, maven, python). They are left
	// unresolved by the load so a slow project can't stall a launch; the selector
	// resolves them in the background and `plt list` resolves them up front. See
	// pending.go. Never serialized.
	PendingTypes []string `yaml:"-"`
	// authoredCommands and cheapCommands retain the two halves of Commands as they
	// were at load — what the user wrote, and what the non-deferred types produced
	// — so resolving a pending type can rebuild the list exactly as a blocking load
	// would have produced it. Runtime only.
	authoredCommands []Command
	cheapCommands    []Command
}

// LocationOverride carries per-folder additions/overrides for a glob location.
// Commands merge by name (a same-named command replaces the inherited one, the
// rest append) and Env merges per key; every other field replaces the inherited
// value when non-empty. It is a separate type from Location on purpose: a folder
// override can't carry its own `location:` or nested `overrides:`.
type LocationOverride struct {
	Name     string            `yaml:"name,omitempty"`
	Types    Types             `yaml:"type,omitempty"`
	Commands []Command         `yaml:"commands,omitempty"`
	Include  []string          `yaml:"include_commands,omitempty"`
	Exclude  []string          `yaml:"exclude_commands,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	// legacyKeys lists the deprecated key spellings used in this override; folded
	// into the parent location's LegacyKeys at decode time.
	legacyKeys []string `yaml:"-"`
}

// locationYAML is the authored shape of a Location: every serialized field plus
// the deprecated `include:`/`exclude:` spellings, which UnmarshalYAML folds into
// the canonical fields. Adding a field to Location means adding it here too — a
// field missing from this struct decodes to nothing, silently.
type locationYAML struct {
	Name             string                      `yaml:"name,omitempty"`
	Location         string                      `yaml:"location,omitempty"`
	Types            Types                       `yaml:"type,omitempty"`
	Commands         []Command                   `yaml:"commands,omitempty"`
	IncludeCommands  []string                    `yaml:"include_commands,omitempty"`
	ExcludeCommands  []string                    `yaml:"exclude_commands,omitempty"`
	Include          []string                    `yaml:"include,omitempty"` // deprecated
	Exclude          []string                    `yaml:"exclude,omitempty"` // deprecated
	ExcludeLocations []string                    `yaml:"exclude_locations,omitempty"`
	Overrides        map[string]LocationOverride `yaml:"overrides,omitempty"`
	Env              map[string]string           `yaml:"env,omitempty"`
}

// UnmarshalYAML decodes a location, accepting the deprecated `include:`/`exclude:`
// spellings alongside the canonical `include_commands:`/`exclude_commands:`. The
// canonical key wins when both are authored; either legacy key is recorded in
// LegacyKeys so the load can warn about it. Canonicalizing at decode time means
// every rewrite path (`plt init`, focus, queue-save) emits the new spelling.
func (l *Location) UnmarshalYAML(value *yaml.Node) error {
	var raw locationYAML
	if err := value.Decode(&raw); err != nil {
		return err
	}

	var legacy []string
	include, usedLegacyInclude := resolveFilterKeys(raw.IncludeCommands, raw.Include)
	if usedLegacyInclude {
		legacy = append(legacy, "include")
	}
	exclude, usedLegacyExclude := resolveFilterKeys(raw.ExcludeCommands, raw.Exclude)
	if usedLegacyExclude {
		legacy = append(legacy, "exclude")
	}

	// Fold each override's legacy keys in, sorted by folder key so the reported
	// order never depends on map iteration.
	for _, key := range sortedOverrideKeys(raw.Overrides) {
		for _, k := range raw.Overrides[key].legacyKeys {
			legacy = append(legacy, "overrides."+key+"."+k)
		}
	}

	l.Name = raw.Name
	l.Location = raw.Location
	l.Types = raw.Types
	l.Commands = raw.Commands
	l.Include = include
	l.Exclude = exclude
	l.ExcludeLocations = raw.ExcludeLocations
	l.Overrides = raw.Overrides
	l.Env = raw.Env
	l.LegacyKeys = legacy
	return nil
}

// locationOverrideYAML is the authored shape of a LocationOverride, including the
// deprecated filter spellings. See locationYAML.
type locationOverrideYAML struct {
	Name            string            `yaml:"name,omitempty"`
	Types           Types             `yaml:"type,omitempty"`
	Commands        []Command         `yaml:"commands,omitempty"`
	IncludeCommands []string          `yaml:"include_commands,omitempty"`
	ExcludeCommands []string          `yaml:"exclude_commands,omitempty"`
	Include         []string          `yaml:"include,omitempty"` // deprecated
	Exclude         []string          `yaml:"exclude,omitempty"` // deprecated
	Env             map[string]string `yaml:"env,omitempty"`
}

// UnmarshalYAML applies the same legacy-key folding as Location.UnmarshalYAML, so
// an `include:` copy-pasted into an override isn't silently ignored.
func (o *LocationOverride) UnmarshalYAML(value *yaml.Node) error {
	var raw locationOverrideYAML
	if err := value.Decode(&raw); err != nil {
		return err
	}

	var legacy []string
	include, usedLegacyInclude := resolveFilterKeys(raw.IncludeCommands, raw.Include)
	if usedLegacyInclude {
		legacy = append(legacy, "include")
	}
	exclude, usedLegacyExclude := resolveFilterKeys(raw.ExcludeCommands, raw.Exclude)
	if usedLegacyExclude {
		legacy = append(legacy, "exclude")
	}

	o.Name = raw.Name
	o.Types = raw.Types
	o.Commands = raw.Commands
	o.Include = include
	o.Exclude = exclude
	o.Env = raw.Env
	o.legacyKeys = legacy
	return nil
}

// resolveFilterKeys picks the effective filter list from the canonical and
// deprecated spellings of a key, reporting whether the deprecated one was
// authored (whether or not it won).
func resolveFilterKeys(canonical, legacy []string) ([]string, bool) {
	if len(legacy) == 0 {
		return canonical, false
	}
	if len(canonical) > 0 {
		return canonical, true // both authored: canonical wins, legacy still reported
	}
	return legacy, true
}

// sortedOverrideKeys returns an override map's keys in lexical order, for
// deterministic iteration.
func sortedOverrideKeys(overrides map[string]LocationOverride) []string {
	if len(overrides) == 0 {
		return nil
	}
	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// FocusKey derives the stable identity used to reference a location in the
// top-level focus list. It matches on the authored name when present, otherwise
// the authored path (root normalizes to "."), so the same key is produced when
// the picker lists entries, when SetFocused writes them back, and when
// resolveFocus reads them.
func (l Location) FocusKey() string {
	if l.Name != "" {
		return l.Name
	}
	if l.Location == "" || l.Location == "." {
		return "."
	}
	return l.Location
}

// resolveFocus turns the authored Config.Focused list into per-location runtime
// Focused flags by matching each location's FocusKey. It runs before glob
// expansion so a focused pattern (e.g. "packages/*") propagates to its children.
func resolveFocus(config *Config) {
	if len(config.Focused) == 0 {
		return
	}
	set := make(map[string]bool, len(config.Focused))
	for _, key := range config.Focused {
		set[key] = true
	}
	for i := range config.Locations {
		if set[config.Locations[i].FocusKey()] {
			config.Locations[i].Focused = true
		}
	}
}

// AnyFocused reports whether any location is marked focused.
func (c *Config) AnyFocused() bool {
	for i := range c.Locations {
		if c.Locations[i].Focused {
			return true
		}
	}
	return false
}

// InvalidConfigError reports a configuration file that exists but could not be
// loaded: a YAML syntax error, an invalid glob pattern, an unknown project type.
// It carries the file's path because the user rarely named that file — discovery
// walks up from the working directory — so a bare "yaml: line 4" leaves them
// hunting for which .pltrc is broken.
type InvalidConfigError struct {
	Path string // absolute where it could be resolved
	Err  error
}

func (e *InvalidConfigError) Error() string {
	return fmt.Sprintf("%s: %v", e.Path, e.Err)
}

func (e *InvalidConfigError) Unwrap() error { return e.Err }

// LoadConfig reads, parses and processes one configuration file. Every failure
// past "the file isn't there" is an *InvalidConfigError naming the file, so
// callers can point the user at it rather than at a line number with no file.
func LoadConfig(configPath string) (*Config, error) {
	cfg, err := loadConfig(configPath)
	if err == nil {
		return cfg, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil, err // a missing file is the caller's to interpret, not a broken one
	}
	return nil, &InvalidConfigError{Path: absOrSelf(configPath), Err: err}
}

// absOrSelf renders a config path for a human: absolute, so a relative path a
// caller passed (".pltrc") still says which directory it came from.
func absOrSelf(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// loadStages is the pipeline every parsed config goes through, in order. The
// order is a contract — each stage relies on the ones before it:
//
//  1. frecency defaults
//  2. deprecated-key warnings: on the authored locations, before expansion, so
//     one authored key yields one warning
//  3. empty paths become "."
//  4. focus: keys are authored, relative paths, so resolve them before glob
//     expansion (a focused pattern propagates to its children) and before
//     absolutization
//  5. glob expansion, against the config's base directory
//  6. absolute paths, against the base directory
//  7. project types: needs absolute paths; a parser that fails or times out
//     degrades to its base commands with a warning, only an invalid type name
//     is fatal
//  8. aliases: needs Name/Type from project types and absolute paths for
//     cd-wrapping; an unresolvable reference is recorded on the command
//  9. warnings: reads Command.Error, so last; rebuilds Warnings from the
//     notices above plus the name/alias checks
//
// ResolveAllPending re-runs 8 and 9 once deferred types arrive.
var loadStages = []func(*Config) error{
	func(c *Config) error { applyDefaultFrecencyConfig(c); return nil },
	func(c *Config) error { collectDeprecatedKeyWarnings(c); return nil },
	func(c *Config) error { normalizeEmptyPaths(c); return nil },
	func(c *Config) error { resolveFocus(c); return nil },
	expandGlobLocations,
	makeLocationPathsAbsolute,
	processProjectTypesStage,
	func(c *Config) error { expandCommandAliases(c); return nil },
	func(c *Config) error { collectConfigWarnings(c); return nil },
}

func loadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading the file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}
	config.baseDir = configBaseDir(configPath, config.Root)

	for _, stage := range loadStages {
		if err := stage(&config); err != nil {
			return nil, err
		}
	}

	return &config, nil
}

// configBaseDir is the directory a config's relative paths resolve against: a
// global project's declared root, otherwise the directory the file sits in.
func configBaseDir(configPath, root string) string {
	if root != "" {
		return root
	}
	return filepath.Dir(absOrSelf(configPath))
}

// expandGlobLocations replaces glob locations with their matching folders.
func expandGlobLocations(c *Config) error {
	expanded, warnings, err := expandGlobPatterns(c.Locations, c.baseDir)
	if err != nil {
		return fmt.Errorf("failed to expand glob patterns: %w", err)
	}
	c.Locations = expanded
	c.loadWarnings = append(c.loadWarnings, warnings...)
	return nil
}

// processProjectTypesStage discovers each location's type commands.
func processProjectTypesStage(c *Config) error {
	warnings, err := processProjectTypes(c)
	if err != nil {
		return fmt.Errorf("failed to process project types: %w", err)
	}
	c.loadWarnings = append(c.loadWarnings, warnings...)
	return nil
}

// applyDefaultFrecencyConfig applies default frecency settings if not configured
func applyDefaultFrecencyConfig(config *Config) {
	// If frecency config is missing, use defaults
	if config.Frecency.RecencyWeight == 0 && config.Frecency.FrequencyWeight == 0 {
		config.Frecency = DefaultFrecencyConfig()
	}
}

// normalizeEmptyPaths normalizes empty location paths to "." (current directory)
func normalizeEmptyPaths(config *Config) {
	for i := range config.Locations {
		if config.Locations[i].Location == "" {
			config.Locations[i].Location = "."
		}
	}
}

// makeLocationPathsAbsolute resolves every relative location path against the
// config's base directory (its own directory, or Root for a global project), so
// nothing downstream depends on the process working directory. A config built in
// memory has no base directory and falls back to the working directory.
func makeLocationPathsAbsolute(config *Config) error {
	for i := range config.Locations {
		loc := config.Locations[i].Location
		if loc == "" || filepath.IsAbs(loc) {
			continue
		}
		if config.baseDir != "" {
			config.Locations[i].Location = filepath.Join(config.baseDir, loc)
			continue
		}
		absPath, err := filepath.Abs(loc)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for location %q: %w", loc, err)
		}
		config.Locations[i].Location = absPath
	}
	return nil
}

// filterCommands filters commands based on include and exclude patterns
// Include patterns act as a whitelist (if specified)
// Exclude patterns act as a blacklist (applied after include)
// Both support glob patterns using filepath.Match syntax
// Patterns match against command name (if present) or command string
func filterCommands(commands []Command, include []string, exclude []string) []Command {
	if len(commands) == 0 {
		return []Command{}
	}

	// If no filters specified, return all commands
	if len(include) == 0 && len(exclude) == 0 {
		return commands
	}

	var result []Command

	// Apply include filter (whitelist)
	if len(include) > 0 {
		for _, cmd := range commands {
			// Match against command name if present, otherwise against command string
			matchString := cmd.Name
			if matchString == "" {
				matchString = cmd.Command
			}
			for _, pattern := range include {
				matched, err := filepath.Match(pattern, matchString)
				if err == nil && matched {
					result = append(result, cmd)
					break // Command matched, no need to check other patterns
				}
			}
		}
	} else {
		// No include filter, start with all commands
		result = append(result, commands...)
	}

	// Apply exclude filter (blacklist)
	if len(exclude) > 0 {
		filtered := make([]Command, 0, len(result))
		for _, cmd := range result {
			// Match against command name if present, otherwise against command string
			matchString := cmd.Name
			if matchString == "" {
				matchString = cmd.Command
			}
			shouldExclude := false
			for _, pattern := range exclude {
				matched, err := filepath.Match(pattern, matchString)
				if err == nil && matched {
					shouldExclude = true
					break
				}
			}
			if !shouldExclude {
				filtered = append(filtered, cmd)
			}
		}
		result = filtered
	}

	return result
}

// processProjectTypes adds each location's type-discovered commands to its
// authored ones (authored first), then applies the location's command filters to
// the combined list. A location with no `type:` still gets filtered — it just has
// nothing to discover.
func processProjectTypes(config *Config) ([]Warning, error) {
	var warnings []Warning
	for i, location := range config.Locations {
		// Accumulate discovered commands across every type the location declares,
		// deferring the ones that would have to run a shell command to find out.
		var discovered []Command
		var pending []string
		for _, typeName := range location.Types {
			if defersLoading(typeName) {
				pending = append(pending, typeName)
				continue
			}
			cmds, degraded, err := commandsForType(typeName, location.Location)
			if err != nil {
				return nil, err
			}
			if degraded != nil {
				degraded.Context = location.DisplayName()
				warnings = append(warnings, *degraded)
			}
			discovered = append(discovered, cmds...)
		}

		config.Locations[i].PendingTypes = pending

		if len(discovered) == 0 && len(pending) == 0 &&
			len(location.Include) == 0 && len(location.Exclude) == 0 {
			continue
		}

		config.Locations[i].authoredCommands = location.Commands
		config.Locations[i].cheapCommands = sortDiscovered(discovered)
		config.Locations[i].Commands = composeCommands(config.Locations[i], nil)
	}

	return warnings, nil
}

// sortDiscovered puts type-discovered commands in a stable order: map iteration
// in the parsers is non-deterministic and several maps get merged, so sort by
// type, then name.
func sortDiscovered(discovered []Command) []Command {
	sort.SliceStable(discovered, func(a, b int) bool {
		if discovered[a].Type != discovered[b].Type {
			return discovered[a].Type < discovered[b].Type
		}
		return discovered[a].Name < discovered[b].Name
	})
	return discovered
}

// composeCommands rebuilds a location's command list from its authored commands
// plus everything its types discovered — the ones resolved at load and any that
// arrived later — so a deferred type produces exactly the list a blocking load
// would have.
func composeCommands(loc Location, late []Command) []Command {
	discovered := append(append([]Command{}, loc.cheapCommands...), late...)
	all := append(append([]Command{}, loc.authoredCommands...), sortDiscovered(discovered)...)
	return filterCommands(all, loc.Include, loc.Exclude)
}

// commandsForType resolves a single project type in a directory and returns its
// commands, each tagged with the type. A type whose detect file is absent
// contributes nothing (the location may declare several types, only some of which
// apply here).
//
// The second return value is a non-fatal warning: when a type's parser fails or
// times out, its base commands are still returned and the failure is reported
// rather than breaking the load. Only a genuinely invalid config — an unknown
// type name — produces an error.
func commandsForType(typeName, directory string) ([]Command, *Warning, error) {
	projectType, err := lookupType(typeName)
	if err != nil {
		return nil, nil, fmt.Errorf("location %s has invalid type: %w", directory, err)
	}

	if !projectType.CanHandleDirectory(directory) {
		return nil, nil, nil
	}

	// A parser failure (broken Makefile, missing `mvn`, a command that timed
	// out) still yields the type's base commands, so keep them and report the
	// problem rather than losing the location — or the whole load.
	commands, parseErr := projectType.Commands(directory)

	var commandList []Command
	for name, cmd := range commands {
		commandList = append(commandList, Command{
			Name:    name,
			Command: cmd,
			Type:    typeName,
		})
	}

	var warning *Warning
	if parseErr != nil {
		reason := parseErr.Error()
		if errors.Is(parseErr, parsers.ErrParserTimeout) {
			reason = fmt.Sprintf("%s parser timed out; only its base commands are available", typeName)
		}
		warning = &Warning{
			Kind:   "parser",
			Scope:  "location",
			Name:   typeName,
			Reason: reason,
		}
	}
	return commandList, warning, nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// LoadGlobalConfig loads the global configuration from ~/.config/paleta/config.yaml
func LoadGlobalConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// If home directory can't be determined, return empty config
		return &Config{
			Frecency: DefaultFrecencyConfig(),
		}, nil
	}

	globalConfigPath := filepath.Join(homeDir, ".config", "paleta", "config.yaml")

	// If global config doesn't exist, return defaults
	if !fileExists(globalConfigPath) {
		return &Config{
			Frecency: DefaultFrecencyConfig(),
		}, nil
	}

	// Load global config
	config, err := LoadConfig(globalConfigPath)
	if err != nil {
		// If there's an error loading, return defaults
		return &Config{
			Frecency: DefaultFrecencyConfig(),
		}, nil
	}

	return config, nil
}

// MergeFrecencyConfig merges frecency settings, with local config taking precedence
func MergeFrecencyConfig(global, local FrecencyConfig) FrecencyConfig {
	result := global

	// If local config has non-zero values, use them (they override global)
	// Note: We need to distinguish between "not set" and "set to false/0"
	// For simplicity, we'll check if any local config is different from zero values
	if local.RecencyWeight != 0 || local.FrequencyWeight != 0 {
		result.RecencyWeight = local.RecencyWeight
		result.FrequencyWeight = local.FrequencyWeight
	}

	// Enabled can be explicitly set to false, so we need special handling
	// If both weights are zero in local, we assume frecency config wasn't specified locally
	localConfigSpecified := local.RecencyWeight != 0 || local.FrequencyWeight != 0
	if localConfigSpecified {
		result.Enabled = local.Enabled
	}

	return result
}

// LoadConfigWithGlobal loads both global and local configs and merges them
func LoadConfigWithGlobal(localConfigPath string) (*Config, error) {
	// Load global config
	globalConfig, _ := LoadGlobalConfig()

	// Load local config
	localConfig, err := LoadConfig(localConfigPath)
	if err != nil {
		return nil, err
	}

	// Merge frecency settings (local overrides global)
	localConfig.Frecency = MergeFrecencyConfig(globalConfig.Frecency, localConfig.Frecency)

	// Carry tool definitions from the global config so the local config's enabled
	// tools can resolve against them (see AttachTools).
	localConfig.ToolDefs = globalConfig.Tools.Defs

	return localConfig, nil
}

// lookupType resolves a declared type name through the parser registry.
func lookupType(typeName string) (*parsers.Type, error) {
	reg, err := parsers.DefaultRegistry()
	if err != nil {
		return nil, fmt.Errorf("loading project types: %w", err)
	}
	typ, ok := reg.Lookup(typeName)
	if !ok {
		return nil, fmt.Errorf("unknown project type: %s", typeName)
	}
	return typ, nil
}

// defersLoading reports whether a type resolves its commands by running a shell
// command. An unknown type defers nothing — it fails the load elsewhere.
func defersLoading(typeName string) bool {
	typ, err := lookupType(typeName)
	return err == nil && typ.DefersLoading()
}
