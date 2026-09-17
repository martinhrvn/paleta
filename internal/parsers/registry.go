package parsers

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Type is a project type as the parser configuration defines it: which files
// mark a directory as this type, how to list its commands, and where it ranks
// among other types found in the same folder.
type Type struct {
	name   string
	config ParserConfig
}

// Name returns the type's name (e.g. "npm", "go").
func (t *Type) Name() string { return t.name }

// DetectFiles returns the file names (or glob patterns) that mark a directory as
// this type.
func (t *Type) DetectFiles() []string { return t.config.DetectFiles }

// CanHandleDirectory reports whether any of the type's detect files is present
// in the directory. Detect files may be glob patterns (e.g.
// "docker-compose.*.yml"), so matching is glob-aware.
func (t *Type) CanHandleDirectory(directory string) bool {
	for _, detectFile := range t.config.DetectFiles {
		if DetectFilePresent(directory, detectFile) {
			return true
		}
	}
	return false
}

// Commands returns the type's commands for a directory, keyed by name. A parser
// failure returns the base commands alongside the error so the caller can
// degrade to them.
func (t *Type) Commands(directory string) (map[string]string, error) {
	return ParseAndFormatCommands(directory, t.config)
}

// DefersLoading reports whether this type's command list comes from running a
// shell command (`./gradlew tasks --all`, a `make -qp` pipeline) rather than from
// reading a file. Callers resolve these in the background instead of blocking a
// config load on them.
func (t *Type) DefersLoading() bool {
	return t.config.ParserCommand != ""
}

// Registry holds every configured project type, in priority order. It is the
// one source for "which types exist" and "how do they rank": the config loader
// resolves declared types through it, and the wizard scan detects and orders a
// folder's types through it, so the two never disagree.
type Registry struct {
	byName  map[string]*Type
	ordered []*Type
}

// NewRegistry builds a registry from a parser configuration.
func NewRegistry(file *ParsersFile) *Registry {
	reg := &Registry{byName: make(map[string]*Type, len(file.Parsers))}
	for name, cfg := range file.Parsers {
		typ := &Type{name: name, config: cfg}
		reg.byName[name] = typ
		reg.ordered = append(reg.ordered, typ)
	}
	sort.SliceStable(reg.ordered, func(i, j int) bool {
		pi, pj := reg.ordered[i].config.Priority, reg.ordered[j].config.Priority
		switch {
		case pi == pj:
			return reg.ordered[i].name < reg.ordered[j].name
		case pi == 0:
			return false // unranked types come last
		case pj == 0:
			return true
		default:
			return pi < pj
		}
	})
	return reg
}

// Lookup returns the type registered under name.
func (r *Registry) Lookup(name string) (*Type, bool) {
	typ, ok := r.byName[name]
	return typ, ok
}

// Types returns every type in priority order (primary first).
func (r *Registry) Types() []*Type {
	return append([]*Type(nil), r.ordered...)
}

// DetectTypes lists the types whose detect files are present in dir, in
// priority order. The JavaScript package managers all detect package.json, so
// they collapse to the one the lockfile names (see JSPackageManager).
func (r *Registry) DetectTypes(dir string) []string {
	var out []string
	js := false
	for _, typ := range r.ordered {
		if !typ.CanHandleDirectory(dir) {
			continue
		}
		if jsPackageManagers[typ.name] {
			if !js {
				js = true
				out = append(out, JSPackageManager(dir))
			}
			continue
		}
		out = append(out, typ.name)
	}
	return out
}

// jsPackageManagers are the types that all detect package.json and are told
// apart by lockfile instead.
var jsPackageManagers = map[string]bool{"npm": true, "yarn": true, "pnpm": true}

// JSPackageManager picks npm, yarn or pnpm for a package.json folder from the
// lockfile present.
func JSPackageManager(dir string) string {
	switch {
	case fileExists(filepath.Join(dir, "pnpm-lock.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(dir, "yarn.lock")):
		return "yarn"
	default:
		return "npm"
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var (
	defaultRegistryMu sync.Mutex
	defaultRegistry   *Registry
)

// DefaultRegistry returns the process-wide registry built from the parser
// configuration (the embedded defaults plus ~/.paleta/parsers.yaml). It is
// built on first use and reused afterwards.
func DefaultRegistry() (*Registry, error) {
	defaultRegistryMu.Lock()
	defer defaultRegistryMu.Unlock()
	if defaultRegistry != nil {
		return defaultRegistry, nil
	}
	file, err := LoadParsersConfig()
	if err != nil {
		return nil, err
	}
	defaultRegistry = NewRegistry(file)
	return defaultRegistry, nil
}

// ReloadRegistry discards the process-wide registry so the next DefaultRegistry
// re-reads the parser configuration. Tests call it after writing an override.
func ReloadRegistry() {
	defaultRegistryMu.Lock()
	defer defaultRegistryMu.Unlock()
	defaultRegistry = nil
}
