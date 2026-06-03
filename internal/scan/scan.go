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
var typePriority = []string{"go", "rust", "npm", "yarn", "pnpm", "python", "maven", "gradle", "make", "docker"}

// Scan walks root and returns one Candidate per directory that contains a
// recognized project file. Results are sorted by RelPath with the root first.
func Scan(root string) ([]Candidate, error) {
	typeMap, err := detectTypeMap()
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
		if _, ok := typeMap[base]; !ok {
			continue
		}
		dir := filepath.Dir(rel)
		byDir[dir] = append(byDir[dir], base)
	}

	candidates := make([]Candidate, 0, len(byDir))
	for dir, detectFiles := range byDir {
		detectFile, typ := chooseType(filepath.Join(root, dir), detectFiles, typeMap)
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
func chooseType(absDir string, detectFiles []string, typeMap map[string]string) (string, string) {
	sort.Strings(detectFiles)

	bestFile, bestType := "", ""
	bestRank := len(typePriority) + 1

	for _, file := range detectFiles {
		typ := typeMap[file]
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

// detectTypeMap builds a detect-file -> paleta-type map from the parser
// configuration. When several parsers claim the same file (e.g. package.json),
// the higher-priority type wins so the map is deterministic.
func detectTypeMap() (map[string]string, error) {
	cfg, err := parsers.LoadParsersConfig()
	if err != nil {
		return nil, err
	}

	// Sort parser names for deterministic iteration.
	names := make([]string, 0, len(cfg.Parsers))
	for name := range cfg.Parsers {
		names = append(names, name)
	}
	sort.Strings(names)

	m := make(map[string]string)
	for _, name := range names {
		for _, file := range cfg.Parsers[name].DetectFiles {
			existing, ok := m[file]
			if !ok || typeRank(name) < typeRank(existing) {
				m[file] = name
			}
		}
	}
	return m, nil
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
