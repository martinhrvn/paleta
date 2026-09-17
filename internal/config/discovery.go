package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

	projectsDir, err := globalProjectsDir()
	if err != nil {
		return nil, err
	}
	cfg, err := loadFromDiscovery(opts.Workdir, projectsDir)
	if err != nil {
		return nil, err
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

// globalProjectsDir is where centralized project configs live
// (~/.config/paleta/projects/).
func globalProjectsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "paleta", "projects"), nil
}

// FindConfigFile searches for .pltrc starting from the process working directory
// and traversing up the directory tree until it finds one or reaches the root.
func FindConfigFile() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	return findConfigFileFromPath(cwd)
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

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read projects directory: %w", err)
	}

	var configs []*Config
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".yaml" && filepath.Ext(name) != ".yml" {
			continue
		}

		config, err := LoadConfig(filepath.Join(projectsDir, name))
		if err != nil {
			// Skip files that fail to load
			continue
		}
		configs = append(configs, config)
	}

	return configs, nil
}

// FindMatchingProject finds a project configuration whose root is a parent of
// (or equal to) cwd. When several match, the one with the deepest root wins.
func FindMatchingProject(cwd string, projects []*Config) *Config {
	var bestMatch *Config
	var bestMatchDepth int

	for _, project := range projects {
		if project.Root == "" {
			continue
		}

		rel, err := filepath.Rel(project.Root, cwd)
		if err != nil {
			continue
		}
		// A relative path starting with ".." means cwd is not under project.Root.
		if len(rel) >= 2 && rel[0] == '.' && rel[1] == '.' {
			continue
		}

		// Depth is the number of directory levels in the root.
		depth := 0
		for p := filepath.Clean(project.Root); p != "/" && p != "."; p = filepath.Dir(p) {
			depth++
		}

		if bestMatch == nil || depth > bestMatchDepth {
			bestMatch = project
			bestMatchDepth = depth
		}
	}

	return bestMatch
}
