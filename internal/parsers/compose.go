package parsers

import (
	"path/filepath"
	"sort"
	"strings"
)

// composePrefix is the invocation every compose base command starts with; the
// -f flag is inserted right after it.
const composePrefix = "docker compose "

// ComposeFilesParser lists a compose project's env-specific variant files
// (docker-compose.dev.yaml, compose.prod.yml, ...) and gives each one its own
// copy of the base commands, run with `docker compose -f <file>`. The plain
// docker-compose.yml is what the base commands already target, so it gets no
// variant of its own.
type ComposeFilesParser struct {
	NullParser
}

// BuildCommands returns `<variant>:<name>` → `docker compose -f <file> <rest>`
// for every variant file and every base command that is a docker compose
// invocation. A base command that runs something else is left alone.
func (c *ComposeFilesParser) BuildCommands(directory string, config ParserConfig) (map[string]string, error) {
	names := make([]string, 0, len(config.BaseCommands))
	for name, cmd := range config.BaseCommands {
		if strings.HasPrefix(cmd, composePrefix) {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	commands := make(map[string]string)
	for _, file := range composeVariantFiles(directory, config.DetectFiles) {
		variant := composeVariantName(file)
		for _, name := range names {
			rest := strings.TrimPrefix(config.BaseCommands[name], composePrefix)
			commands[variant+":"+name] = composePrefix + "-f " + file + " " + rest
		}
	}
	return commands, nil
}

// composeVariantFiles returns the basenames in directory that match the glob
// patterns among detectFiles, sorted. Literal detect files (docker-compose.yml)
// are not variants and are skipped.
func composeVariantFiles(directory string, detectFiles []string) []string {
	var files []string
	for _, pattern := range detectFiles {
		if !strings.ContainsAny(pattern, "*?[") {
			continue
		}
		matches, err := filepath.Glob(filepath.Join(directory, pattern))
		if err != nil {
			continue
		}
		for _, m := range matches {
			files = append(files, filepath.Base(m))
		}
	}
	sort.Strings(files)
	return files
}

// composeVariantName is the part of a variant file name between the compose
// stem and the extension: docker-compose.dev.yaml → dev, compose.prod.yml →
// prod, docker-compose.dev.local.yaml → dev.local.
func composeVariantName(file string) string {
	stem := strings.TrimSuffix(file, filepath.Ext(file))
	if i := strings.Index(stem, "."); i >= 0 {
		return stem[i+1:]
	}
	return stem
}
