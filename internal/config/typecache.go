package config

// Cached results for deferred project types.
//
// A deferred type (see pending.go) finds its commands by running a shell command,
// and `./gradlew tasks --all` can take many seconds. Its last result is kept on
// disk, one file per (directory, type definition), so the next launch can show it
// straight away: the selector renders the cached commands and refreshes them in
// the background, and `plt list` trusts them as long as the type's detect files
// (build.gradle, Makefile, ...) are unchanged.
//
// The cache holds the type's raw name→command map, before authored commands and
// include/exclude filters are applied, so editing .pltrc never needs a refresh.
// Every failure here is a miss: the cache only ever saves time, never breaks a
// load.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/martinhrvn/paleta/internal/parsers"
)

const typeCacheVersion = 1

type typeCacheEntry struct {
	Version     int               `json:"version"`
	Dir         string            `json:"dir"`
	Type        string            `json:"type"`
	Fingerprint string            `json:"fingerprint"`
	SavedAt     time.Time         `json:"saved_at"`
	Commands    map[string]string `json:"commands"`
}

// typeCacheDir is where cache entries live, resolved per call so a test that
// points HOME elsewhere gets its own cache.
func typeCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return filepath.Join(home, ".paleta", "cache", "types")
}

// typeCachePath names the entry for a type in a directory. The type's whole
// definition is part of the key, so editing it in parsers.yaml (a new
// parser_command, different base commands) never reuses the old result.
func typeCachePath(typ *parsers.Type, dir string) string {
	definition, _ := json.Marshal(typ.Config())
	sum := sha256.Sum256([]byte(dir + "\x00" + typ.Name() + "\x00" + string(definition)))
	return filepath.Join(typeCacheDir(), hex.EncodeToString(sum[:])[:16]+".json")
}

// detectFingerprint summarizes the type's detect files in dir (name, size, mtime),
// glob patterns expanded. A change to any of them marks a cached result stale.
func detectFingerprint(typ *parsers.Type, dir string) string {
	var parts []string
	for _, pattern := range typ.DetectFiles() {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s:%d:%d", filepath.Base(match), info.Size(), info.ModTime().UnixNano()))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

// loadTypeCache returns the last stored commands for the type in dir. fresh
// reports whether the detect files are unchanged since they were stored; a stale
// entry is still returned so an interactive caller can show it while refreshing.
func loadTypeCache(typ *parsers.Type, dir string) (commands map[string]string, fresh bool, ok bool) {
	data, err := os.ReadFile(typeCachePath(typ, dir))
	if err != nil {
		return nil, false, false
	}
	var entry typeCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil ||
		entry.Version != typeCacheVersion || entry.Dir != dir || entry.Type != typ.Name() {
		return nil, false, false
	}
	return entry.Commands, entry.Fingerprint == detectFingerprint(typ, dir), true
}

// storeTypeCache records the type's commands for dir. The write goes through a
// temp file and a rename, so a concurrent reader sees the old entry or the new
// one, never half of one.
func storeTypeCache(typ *parsers.Type, dir string, commands map[string]string) {
	data, err := json.Marshal(typeCacheEntry{
		Version:     typeCacheVersion,
		Dir:         dir,
		Type:        typ.Name(),
		Fingerprint: detectFingerprint(typ, dir),
		SavedAt:     time.Now(),
		Commands:    commands,
	})
	if err != nil {
		return
	}

	path := typeCachePath(typ, dir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return
	}
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil || os.Rename(tmp.Name(), path) != nil {
		os.Remove(tmp.Name())
	}
}
