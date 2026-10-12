package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocationFor_LongestPrefix(t *testing.T) {
	cfg := &Config{Locations: []Location{
		{Name: "root", Location: "/p"},
		{Name: "internal", Location: "/p/internal"},
		{Name: "ui", Location: "/p/internal/ui"},
		{Name: "uix", Location: "/p/internal/uix"},
	}}
	cases := map[string]string{
		"/p/internal/ui/sub": "ui",
		"/p/internal/ui":     "ui",
		"/p/internal/uix":    "uix",
		"/p/internal/x":      "internal",
		"/p/cmd":             "root",
		"/p":                 "root",
		"/p/internal/ui/":    "ui",
	}
	for dir, want := range cases {
		loc, ok := cfg.LocationFor(dir)
		if !ok || loc.DisplayName() != want {
			t.Errorf("LocationFor(%q) = %v, %v; want %q", dir, loc, ok, want)
		}
	}
	if _, ok := cfg.LocationFor("/elsewhere"); ok {
		t.Error("a directory outside every location should not match")
	}
	if _, ok := cfg.LocationFor("/pp"); ok {
		t.Error("prefix must be on a path boundary (/pp is not under /p)")
	}
}

func TestLocationFor_ResolvesSymlinks(t *testing.T) {
	real := t.TempDir()
	if err := os.Mkdir(filepath.Join(real, "infra"), 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Locations: []Location{{Name: "infra", Location: filepath.Join(real, "infra")}}}
	loc, ok := cfg.LocationFor(filepath.Join(link, "infra"))
	if !ok || loc.DisplayName() != "infra" {
		t.Fatalf("symlinked dir should match its real location, got %v %v", loc, ok)
	}
}
