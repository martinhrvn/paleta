package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/martinhrvn/paleta/internal/parsers"
)

// makeTypeInTempDir points the `make` parser at parserCommand (isolating HOME, and
// so the cache directory) and returns the type plus a directory holding a Makefile.
func makeTypeInTempDir(t *testing.T, parserCommand string) (*parsers.Type, string) {
	t.Helper()
	withMakeParserCommand(t, parserCommand)
	typ, err := lookupType("make")
	if err != nil {
		t.Fatalf("lookupType: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte("build:\n"), 0644); err != nil {
		t.Fatalf("write Makefile: %v", err)
	}
	return typ, dir
}

func TestTypeCache_RoundTrip(t *testing.T) {
	typ, dir := makeTypeInTempDir(t, "\"echo test\"")
	want := map[string]string{"build": "make build", "test": "make test"}

	storeTypeCache(typ, dir, want)

	got, fresh, ok := loadTypeCache(typ, dir)
	if !ok || !fresh {
		t.Fatalf("loadTypeCache ok=%v fresh=%v, want a fresh hit", ok, fresh)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("commands = %v, want %v", got, want)
	}
}

func TestTypeCache_MissWhenEmpty(t *testing.T) {
	typ, dir := makeTypeInTempDir(t, "\"echo test\"")
	if _, _, ok := loadTypeCache(typ, dir); ok {
		t.Errorf("loadTypeCache hit on an empty cache")
	}
}

// A changed detect file makes the entry stale — still returned (so the palette can
// show it while refreshing), but not trusted by a non-interactive load.
func TestTypeCache_StaleWhenDetectFileChanges(t *testing.T) {
	typ, dir := makeTypeInTempDir(t, "\"echo test\"")
	storeTypeCache(typ, dir, map[string]string{"test": "make test"})

	makefile := filepath.Join(dir, "Makefile")
	if err := os.WriteFile(makefile, []byte("build:\ntest:\n"), 0644); err != nil {
		t.Fatalf("rewrite Makefile: %v", err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(makefile, later, later); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	got, fresh, ok := loadTypeCache(typ, dir)
	if !ok {
		t.Fatalf("loadTypeCache missed a stale entry; want it returned")
	}
	if fresh {
		t.Errorf("entry reported fresh after the Makefile changed")
	}
	if got["test"] != "make test" {
		t.Errorf("commands = %v, want the cached ones", got)
	}
}

// Editing the type's definition (e.g. its parser_command in parsers.yaml) must not
// reuse commands the old definition produced.
func TestTypeCache_MissWhenTypeDefinitionChanges(t *testing.T) {
	typ, dir := makeTypeInTempDir(t, "\"echo test\"")
	storeTypeCache(typ, dir, map[string]string{"test": "make test"})

	// Rewrite the override under the same HOME, so the cache directory is unchanged.
	override := "parsers:\n  make:\n    detect_files: [\"Makefile\"]\n    base_commands:\n      build: \"make build\"\n    parser_command: \"echo other\"\n"
	if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".paleta", "parsers.yaml"), []byte(override), 0644); err != nil {
		t.Fatalf("write parsers.yaml: %v", err)
	}
	parsers.ReloadRegistry()
	changed, err := lookupType("make")
	if err != nil {
		t.Fatalf("lookupType: %v", err)
	}
	if _, _, ok := loadTypeCache(changed, dir); ok {
		t.Errorf("loadTypeCache hit after the parser_command changed")
	}
}

func TestTypeCache_CorruptFileIsAMiss(t *testing.T) {
	typ, dir := makeTypeInTempDir(t, "\"echo test\"")
	path := typeCachePath(typ, dir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, ok := loadTypeCache(typ, dir); ok {
		t.Errorf("loadTypeCache hit on a corrupt file")
	}
}
