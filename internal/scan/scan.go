// Package scan discovers sub-projects within a directory tree by looking for
// recognized "project files" (package.json, go.mod, Cargo.toml, ...). It is the
// pure-logic backend for the interactive `plt init` wizard: no UI, no I/O beyond
// reading the filesystem and (optionally) consulting git for ignore rules.
package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/martinhrvn/paleta/internal/parsers"
)

// Candidate is a directory detected as a project, with its inferred paleta type.
type Candidate struct {
	RelPath    string // path relative to the scan root ("." for the root itself)
	Type       string // detected paleta type: npm|yarn|pnpm|go|rust|...
	DetectFile string // the file that triggered detection, e.g. "package.json"
}

// ignoredDirs is the built-in skip set used when no git repository is available
// to consult .gitignore.
var ignoredDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"target":       true,
	"dist":         true,
	"build":        true,
	".venv":        true,
	"venv":         true,
	"__pycache__":  true,
	".direnv":      true,
	".devenv":      true,
	".idea":        true,
}

// typePriority ranks detected types so a directory holding several project files
// (e.g. a Go service with a Dockerfile) gets its most meaningful type.
var typePriority = []string{"go", "rust", "npm", "yarn", "pnpm", "python", "maven", "gradle", "make", "compose", "docker"}

// Scan walks root and returns one Candidate per directory that contains a
// recognized project file. Results are sorted by RelPath with the root first.
func Scan(root string) ([]Candidate, error) {
	matcher, err := buildDetectMatcher()
	if err != nil {
		return nil, err
	}

	files, err := enumerateFiles(root)
	if err != nil {
		return nil, err
	}

	// Group recognized detect files by their containing directory.
	byDir := make(map[string][]string)
	for _, rel := range files {
		base := filepath.Base(rel)
		if _, ok := matcher.match(base); !ok {
			continue
		}
		dir := filepath.Dir(rel)
		byDir[dir] = append(byDir[dir], base)
	}

	candidates := make([]Candidate, 0, len(byDir))
	for dir, detectFiles := range byDir {
		detectFile, typ := chooseType(filepath.Join(root, dir), detectFiles, matcher)
		candidates = append(candidates, Candidate{
			RelPath:    dir,
			Type:       typ,
			DetectFile: detectFile,
		})
	}

	sortCandidates(candidates)
	return candidates, nil
}

// chooseType picks the most meaningful (detectFile, type) for a directory that
// may contain several project files. package.json is refined to the concrete JS
// package manager by inspecting lockfiles in absDir.
func chooseType(absDir string, detectFiles []string, matcher detectMatcher) (string, string) {
	sort.Strings(detectFiles)

	bestFile, bestType := "", ""
	bestRank := len(typePriority) + 1

	for _, file := range detectFiles {
		typ, ok := matcher.match(file)
		if !ok {
			continue
		}
		if file == "package.json" {
			typ = detectJSPackageManager(absDir)
		}
		rank := typeRank(typ)
		if rank < bestRank {
			bestRank, bestFile, bestType = rank, file, typ
		}
	}
	return bestFile, bestType
}

// detectJSPackageManager picks npm/yarn/pnpm based on the lockfile present in dir.
func detectJSPackageManager(dir string) string {
	switch {
	case fileExists(filepath.Join(dir, "pnpm-lock.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(dir, "yarn.lock")):
		return "yarn"
	default:
		return "npm"
	}
}

// typeRank returns the priority index of a type (lower is higher priority).
func typeRank(typ string) int {
	for i, t := range typePriority {
		if t == typ {
			return i
		}
	}
	return len(typePriority)
}

// sortCandidates orders candidates by RelPath, with the root (".") first.
func sortCandidates(candidates []Candidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].RelPath == "." {
			return true
		}
		if candidates[j].RelPath == "." {
			return false
		}
		return candidates[i].RelPath < candidates[j].RelPath
	})
}

// globRule pairs a filename glob pattern (e.g. "docker-compose.*.yml") with the
// paleta type it detects.
type globRule struct {
	pattern string
	typ     string
}

// detectMatcher resolves a filename (basename) to a paleta type. Literal detect
// files are matched exactly; glob detect patterns are matched with filepath.Match.
type detectMatcher struct {
	literals map[string]string // basename -> type
	globs    []globRule
}

// match returns the highest-priority type that claims base, and whether any did.
func (m detectMatcher) match(base string) (string, bool) {
	best, bestRank := "", len(typePriority)+1
	if typ, ok := m.literals[base]; ok {
		if r := typeRank(typ); r < bestRank {
			best, bestRank = typ, r
		}
	}
	for _, g := range m.globs {
		if ok, _ := filepath.Match(g.pattern, base); ok {
			if r := typeRank(g.typ); r < bestRank {
				best, bestRank = g.typ, r
			}
		}
	}
	return best, best != ""
}

// hasGlobMeta reports whether a detect-file entry is a glob pattern rather than a
// literal filename.
func hasGlobMeta(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// buildDetectMatcher constructs a detectMatcher from the parser configuration.
// When several parsers claim the same literal file (e.g. package.json), the
// higher-priority type wins so matching is deterministic.
func buildDetectMatcher() (detectMatcher, error) {
	cfg, err := parsers.LoadParsersConfig()
	if err != nil {
		return detectMatcher{}, err
	}

	// Sort parser names for deterministic iteration.
	names := make([]string, 0, len(cfg.Parsers))
	for name := range cfg.Parsers {
		names = append(names, name)
	}
	sort.Strings(names)

	matcher := detectMatcher{literals: make(map[string]string)}
	for _, name := range names {
		for _, file := range cfg.Parsers[name].DetectFiles {
			if hasGlobMeta(file) {
				matcher.globs = append(matcher.globs, globRule{pattern: file, typ: name})
				continue
			}
			existing, ok := matcher.literals[file]
			if !ok || typeRank(name) < typeRank(existing) {
				matcher.literals[file] = name
			}
		}
	}
	return matcher, nil
}

// detectTypeMap builds a detect-file -> paleta-type map of literal detect files
// from the parser configuration. When several parsers claim the same file (e.g.
// package.json), the higher-priority type wins so the map is deterministic.
func detectTypeMap() (map[string]string, error) {
	matcher, err := buildDetectMatcher()
	if err != nil {
		return nil, err
	}
	return matcher.literals, nil
}

// enumerateFiles returns project-relevant file paths relative to root. Inside a
// git work tree it consults git so .gitignore is honored; otherwise it walks the
// tree skipping a built-in set of directories.
func enumerateFiles(root string) ([]string, error) {
	if files, ok := gitListFiles(root); ok {
		return files, nil
	}
	return walkFiles(root)
}

// gitListFiles returns non-ignored files (tracked + untracked) relative to root
// using git. The bool is false when root is not inside a git work tree or git is
// unavailable, signaling the caller to fall back to a manual walk.
func gitListFiles(root string) ([]string, bool) {
	check := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree")
	out, err := check.Output()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return nil, false
	}

	cmd := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard")
	out, err = cmd.Output()
	if err != nil {
		return nil, false
	}

	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, filepath.FromSlash(line))
		}
	}
	return files, true
}

// walkFiles walks root collecting file paths relative to root, skipping the
// built-in ignored directories.
func walkFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && ignoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// fileExists reports whether path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
