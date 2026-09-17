package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/martinhrvn/paleta/internal/parsers"
)

// Candidate is a directory detected as a project, with its inferred paleta types.
// A directory can match several types at once (e.g. an npm package that also has
// a Dockerfile); Types is ordered by detection priority, primary type first.
type Candidate struct {
	RelPath    string   // path relative to the scan root ("." for the root itself)
	Types      []string // detected paleta types, primary first: e.g. [npm, docker]
	DetectFile string   // the file that triggered detection of the primary type
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

// Scan walks root and returns one Candidate per directory that contains a
// recognized project file. Results are sorted by RelPath with the root first.
func Scan(root string) ([]Candidate, error) {
	reg, err := parsers.DefaultRegistry()
	if err != nil {
		return nil, err
	}
	matcher := newDetectMatcher(reg)

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

	// The registry decides which types a folder has and how they rank, exactly
	// as the config loader does for a declared type.
	candidates := make([]Candidate, 0, len(byDir))
	for dir, detectFiles := range byDir {
		types := reg.DetectTypes(filepath.Join(root, dir))
		if len(types) == 0 {
			continue
		}
		primary, _ := reg.Lookup(types[0])
		candidates = append(candidates, Candidate{
			RelPath:    dir,
			Types:      types,
			DetectFile: primaryDetectFile(primary, detectFiles),
		})
	}

	sortCandidates(candidates)
	return candidates, nil
}

// primaryDetectFile names the file, among those found in the folder, that
// marks it as the primary type: the type's first detect file present.
func primaryDetectFile(typ *parsers.Type, files []string) string {
	if typ == nil {
		return ""
	}
	sort.Strings(files)
	for _, pattern := range typ.DetectFiles() {
		for _, file := range files {
			if file == pattern {
				return file
			}
			if ok, _ := filepath.Match(pattern, file); ok && hasGlobMeta(pattern) {
				return file
			}
		}
	}
	return ""
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

// detectMatcher recognizes a filename (basename) as some type's detect file, so
// the scan knows which directories to look at. Literal detect files are matched
// exactly; glob detect patterns with filepath.Match. Which types a directory
// then has is the registry's call (Registry.DetectTypes).
type detectMatcher struct {
	literals map[string]string // basename -> the highest-priority type claiming it
	globs    []globRule
}

// match reports whether base is a detect file, and the type that claims it.
func (m detectMatcher) match(base string) (string, bool) {
	if typ, ok := m.literals[base]; ok {
		return typ, true
	}
	for _, g := range m.globs {
		if ok, _ := filepath.Match(g.pattern, base); ok {
			return g.typ, true
		}
	}
	return "", false
}

// hasGlobMeta reports whether a detect-file entry is a glob pattern rather than a
// literal filename.
func hasGlobMeta(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// buildDetectMatcher constructs a detectMatcher from the process-wide registry.
func buildDetectMatcher() (detectMatcher, error) {
	reg, err := parsers.DefaultRegistry()
	if err != nil {
		return detectMatcher{}, err
	}
	return newDetectMatcher(reg), nil
}

// newDetectMatcher collects every type's detect files. Types come in priority
// order, so when several claim the same literal file (package.json) the first,
// highest-priority one is recorded.
func newDetectMatcher(reg *parsers.Registry) detectMatcher {
	matcher := detectMatcher{literals: make(map[string]string)}
	for _, typ := range reg.Types() {
		for _, file := range typ.DetectFiles() {
			if hasGlobMeta(file) {
				matcher.globs = append(matcher.globs, globRule{pattern: file, typ: typ.Name()})
				continue
			}
			if _, taken := matcher.literals[file]; !taken {
				matcher.literals[file] = typ.Name()
			}
		}
	}
	return matcher
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
