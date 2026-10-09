package config

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const ConfigFileName = ".pltrc"

// ErrConfigNotFound is returned (wrapped) when no local .pltrc is found in the
// working directory or its parents and no matching global project configuration
// exists. Callers can detect it with errors.Is to show a friendly hint.
var ErrConfigNotFound = errors.New("no paleta configuration found")

// LoadOptions says how Load assembles a configuration.
type LoadOptions struct {
	// Workdir is the directory the user invoked plt from. Discovery walks up from
	// it to find .pltrc (or matches a global project against it), and enabled
	// tools run in it. Empty means the process working directory.
	Workdir string
	// Defer leaves shell-backed project types unresolved in Location.PendingTypes
	// so an interactive caller can resolve them in the background (see
	// pending.go). Non-interactive callers leave it false and get a complete
	// command list.
	Defer bool
	// ConfigPath, when set, is the file to load instead of discovering one. BaseDir
	// then overrides the directory its relative paths and globs resolve against
	// (the file's own directory, or a global project's root). Together they load
	// the configuration of one checkout as it applies to another — a linked git
	// worktree of the same project; see Config.ReloadAt. BaseDir is ignored
	// without ConfigPath.
	ConfigPath string
	BaseDir    string
}

// Load is the one way to get a usable configuration. It discovers the config for
// a working directory, loads it, resolves deferred project types unless asked
// not to, and attaches the enabled tools — in that order, every time. Nothing
// about the result depends on the process working directory: paths inside the
// file resolve against the file's own directory (or a global project's root).
//
// A missing configuration is reported with ErrConfigNotFound; a broken file with
// an *InvalidConfigError naming it.
func Load(opts LoadOptions) (*Config, error) {
	if opts.Workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %w", err)
		}
		opts.Workdir = wd
	}

	var cfg *Config
	if opts.ConfigPath != "" {
		loaded, err := loadConfigWithGlobalAt(opts.ConfigPath, opts.BaseDir)
		if err != nil {
			return nil, err
		}
		loaded.Path = opts.ConfigPath
		cfg = loaded
	} else {
		projectsDir, err := globalProjectsDir()
		if err != nil {
			return nil, err
		}
		cfg, err = loadFromDiscovery(opts.Workdir, projectsDir)
		if err != nil {
			return nil, err
		}
	}

	cfg.loadOpts = opts
	if !opts.Defer {
		ResolveAllPending(cfg)
	}
	AttachTools(cfg, opts.Workdir)
	return cfg, nil
}

// Reload runs the same Load that produced c, so a config re-read after a save is
// assembled exactly like the original: same working directory, same deferral,
// tools attached.
func (c *Config) Reload() (*Config, error) {
	return Load(c.loadOpts)
}

// ReloadAt loads the configuration c describes as it applies to another checkout
// of the same project — a linked git worktree whose mirror of c's base directory
// is dir. The .pltrc at dir wins when that checkout has one (it may differ on
// that branch); otherwise c's own file is loaded with its relative paths and
// globs resolved under dir. The result remembers the move, so a later Reload
// stays in that checkout.
func (c *Config) ReloadAt(dir string) (*Config, error) {
	opts := c.loadOpts
	opts.Workdir = dir
	opts.BaseDir = dir
	opts.ConfigPath = c.Path
	if own := filepath.Join(dir, ConfigFileName); exists(own) {
		opts.ConfigPath = own
	}
	return Load(opts)
}

// globalProjectsDir is where centralized project configs live
// (~/.config/paleta/projects/).
func globalProjectsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "paleta", "projects"), nil
}

// FindConfigFile returns the config file that governs the process working
// directory; see FindConfigFileFor.
func FindConfigFile() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	return FindConfigFileFor(cwd)
}

// FindConfigFileFor returns the file Load would read for workdir: the nearest
// .pltrc, or else the global project file whose root contains workdir. It is the
// file to edit or rewrite. A missing one is reported with ErrConfigNotFound.
func FindConfigFileFor(workdir string) (string, error) {
	projectsDir, err := globalProjectsDir()
	if err != nil {
		return "", err
	}
	return findConfigFileIn(workdir, projectsDir)
}

// findConfigFileIn is FindConfigFileFor with an explicit global projects
// directory. Global files are matched on their root key alone, so one whose
// body is broken is still found — editing is how it gets fixed.
func findConfigFileIn(workdir, projectsDir string) (string, error) {
	if path, err := findConfigFileFromPath(workdir); err == nil {
		return path, nil
	}

	best, bestDepth := "", -1
	for _, path := range globalProjectFiles(projectsDir) {
		if depth, ok := rootDepth(readRootKey(path), workdir); ok && depth > bestDepth {
			best, bestDepth = path, depth
		}
	}
	if best == "" {
		return "", fmt.Errorf("%w: no %s in this directory or any parent, and no matching global project", ErrConfigNotFound, ConfigFileName)
	}
	return best, nil
}

// readRootKey returns the top-level root key of the YAML file at path, or ""
// when it has none. The rest of the file is not interpreted: when it doesn't
// parse, the root is read from its own `root:` line instead.
func readRootKey(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var head struct {
		Root string `yaml:"root"`
	}
	if err := yaml.Unmarshal(data, &head); err == nil {
		return head.Root
	}
	for _, line := range strings.Split(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "root:"); ok {
			if err := yaml.Unmarshal([]byte(value), &head.Root); err == nil {
				return head.Root
			}
		}
	}
	return ""
}

// findConfigFileFromPath searches for .pltrc starting from the given path and
// traversing up the directory tree.
func findConfigFileFromPath(startPath string) (string, error) {
	dir, ok := ascend(startPath, func(d string) bool {
		return exists(filepath.Join(d, ConfigFileName))
	})
	if !ok {
		return "", fmt.Errorf("no %s found in current directory or any parent directories", ConfigFileName)
	}
	return filepath.Join(dir, ConfigFileName), nil
}

// LoadConfigFromDiscovery is the discovery step of Load for the process working
// directory: it finds and loads the nearest .pltrc, falling back to global
// project configurations. Deferred types stay pending and no tools are attached;
// use Load for a complete configuration.
func LoadConfigFromDiscovery() (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}
	projectsDir, err := globalProjectsDir()
	if err != nil {
		return nil, err
	}
	return loadFromDiscovery(cwd, projectsDir)
}

// loadConfigFromDiscoveryWithGlobalFallback is the discovery step with an
// explicit global projects directory, for tests.
func loadConfigFromDiscoveryWithGlobalFallback(projectsDir string) (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}
	return loadFromDiscovery(cwd, projectsDir)
}

// loadFromDiscovery loads the nearest .pltrc above workdir with the global
// frecency and tool settings merged in, or — when there is none — the global
// project whose root contains workdir. It never changes the process working
// directory.
func loadFromDiscovery(workdir, projectsDir string) (*Config, error) {
	if configPath, err := findConfigFileFromPath(workdir); err == nil {
		cfg, err := LoadConfigWithGlobal(configPath)
		if err != nil {
			return nil, err
		}
		cfg.Path = configPath
		return cfg, nil
	}

	projects, err := loadGlobalProjectsFromDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load global projects: %w", err)
	}

	matched := FindMatchingProject(workdir, projects)
	if matched == nil {
		return nil, fmt.Errorf("%w: no %s in this directory or any parent, and no matching global project", ErrConfigNotFound, ConfigFileName)
	}

	// The matched project is already loaded and processed; merge in the global
	// frecency settings and tool definitions.
	globalConfig, _ := LoadGlobalConfig()
	matched.Frecency = MergeFrecencyConfig(globalConfig.Frecency, matched.Frecency)
	matched.ToolDefs = globalConfig.Tools.Defs

	return matched, nil
}

// LoadGlobalProjects loads all project configurations from ~/.config/paleta/projects/
func LoadGlobalProjects() ([]*Config, error) {
	projectsDir, err := globalProjectsDir()
	if err != nil {
		return nil, err
	}
	return loadGlobalProjectsFromDir(projectsDir)
}

// loadGlobalProjectsFromDir loads all project configurations from the specified directory
func loadGlobalProjectsFromDir(projectsDir string) ([]*Config, error) {
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return []*Config{}, nil
	}

	if _, err := os.ReadDir(projectsDir); err != nil {
		return nil, fmt.Errorf("failed to read projects directory: %w", err)
	}

	var configs []*Config
	for _, path := range globalProjectFiles(projectsDir) {
		config, err := LoadConfig(path)
		if err != nil {
			// Skip files that fail to load
			continue
		}
		config.Path = path
		configs = append(configs, config)
	}

	return configs, nil
}

// globalProjectFiles lists the .yaml/.yml files in projectsDir. A missing or
// unreadable directory has none.
func globalProjectFiles(projectsDir string) []string {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".yaml" && filepath.Ext(name) != ".yml" {
			continue
		}
		paths = append(paths, filepath.Join(projectsDir, name))
	}
	return paths
}

// FindMatchingProject finds a project configuration whose root is a parent of
// (or equal to) cwd. When several match, the one with the deepest root wins.
func FindMatchingProject(cwd string, projects []*Config) *Config {
	var bestMatch *Config
	var bestMatchDepth int

	for _, project := range projects {
		depth, ok := rootDepth(project.Root, cwd)
		if !ok {
			continue
		}
		if bestMatch == nil || depth > bestMatchDepth {
			bestMatch = project
			bestMatchDepth = depth
		}
	}

	return bestMatch
}

// rootDepth reports whether root is cwd or one of its parents and, if so, how
// many directory levels root has: the deeper root is the more specific match.
func rootDepth(root, cwd string) (int, bool) {
	if root == "" {
		return 0, false
	}
	rel, err := filepath.Rel(root, cwd)
	if err != nil {
		return 0, false
	}
	// A relative path starting with ".." means cwd is not under root.
	if len(rel) >= 2 && rel[0] == '.' && rel[1] == '.' {
		return 0, false
	}

	depth := 0
	for p := filepath.Clean(root); p != "/" && p != "."; p = filepath.Dir(p) {
		depth++
	}
	return depth, true
}

// GlobalProjectFile names the global project file for the project at root (an
// absolute path): the existing file whose root it is, so a rerun edits that one,
// otherwise ~/.config/paleta/projects/<dir name>.yaml.
func GlobalProjectFile(root string) (string, error) {
	projectsDir, err := globalProjectsDir()
	if err != nil {
		return "", err
	}
	return globalProjectFileIn(projectsDir, root), nil
}

// globalProjectFileIn is GlobalProjectFile with an explicit projects directory.
// When <dir name>.yaml already belongs to another root, a short hash of root
// tells the two apart.
func globalProjectFileIn(projectsDir, root string) string {
	for _, path := range globalProjectFiles(projectsDir) {
		if readRootKey(path) == root {
			return path
		}
	}
	name := filepath.Base(root)
	path := filepath.Join(projectsDir, name+".yaml")
	if _, err := os.Stat(path); err == nil {
		sum := sha256.Sum256([]byte(root))
		path = filepath.Join(projectsDir, fmt.Sprintf("%s-%x.yaml", name, sum[:3]))
	}
	return path
}
