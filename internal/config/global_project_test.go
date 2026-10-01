package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile writes content to path, creating parent directories.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A global project config is a real file, so a loaded one carries its path just
// like a discovered .pltrc does — that is what lets edits find their way back.
func TestLoadFromDiscoveryGlobalProjectHasPath(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "repo")
	projectsDir := filepath.Join(tmp, "projects")
	yamlPath := filepath.Join(projectsDir, "repo.yaml")
	writeFile(t, yamlPath, "root: "+root+"\nlocations:\n  - location: \".\"\n    commands: [\"build\"]\n")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFromDiscovery(filepath.Join(root, "sub"), projectsDir)
	if err != nil {
		t.Fatalf("loadFromDiscovery() error = %v", err)
	}
	if cfg.Path != yamlPath {
		t.Errorf("Path = %q, want %q", cfg.Path, yamlPath)
	}
	if cfg.BaseDir() != root {
		t.Errorf("BaseDir() = %q, want %q", cfg.BaseDir(), root)
	}
}

func TestFindConfigFileIn(t *testing.T) {
	tests := []struct {
		name    string
		local   bool              // write <tmp>/repo/.pltrc
		globals map[string]string // file name -> content; ROOT is replaced with <tmp>
		want    string            // relative to <tmp>; empty means ErrConfigNotFound
	}{
		{
			name:    "local .pltrc wins over a global match",
			local:   true,
			globals: map[string]string{"repo.yaml": "root: ROOT/repo\n"},
			want:    "repo/.pltrc",
		},
		{
			name:    "global project matching an ancestor",
			globals: map[string]string{"repo.yaml": "root: ROOT/repo\n"},
			want:    "projects/repo.yaml",
		},
		{
			name: "deepest root wins",
			globals: map[string]string{
				"outer.yaml": "root: ROOT\n",
				"inner.yml":  "root: ROOT/repo\n",
			},
			want: "projects/inner.yml",
		},
		{
			name:    "broken body with a valid root is still found",
			globals: map[string]string{"repo.yaml": "root: ROOT/repo\nlocations: [\n"},
			want:    "projects/repo.yaml",
		},
		{
			name:    "unrelated root is not a match",
			globals: map[string]string{"other.yaml": "root: ROOT/other\n"},
		},
		{
			name: "nothing at all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			workdir := filepath.Join(tmp, "repo", "sub")
			if err := os.MkdirAll(workdir, 0o755); err != nil {
				t.Fatal(err)
			}
			if tt.local {
				writeFile(t, filepath.Join(tmp, "repo", ConfigFileName), "locations: []\n")
			}
			projectsDir := filepath.Join(tmp, "projects")
			for name, content := range tt.globals {
				writeFile(t, filepath.Join(projectsDir, name), strings.ReplaceAll(content, "ROOT", tmp))
			}

			got, err := findConfigFileIn(workdir, projectsDir)
			if tt.want == "" {
				if !errors.Is(err, ErrConfigNotFound) {
					t.Fatalf("err = %v, want ErrConfigNotFound", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("findConfigFileIn() error = %v", err)
			}
			if want := filepath.Join(tmp, tt.want); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestFileSetRoot(t *testing.T) {
	t.Run("new file gets root first and saves into a missing directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "does", "not", "exist", "repo.yaml")
		f := NewFile(path)
		if err := f.SetRoot("/abs/repo"); err != nil {
			t.Fatal(err)
		}
		if err := f.SetLocations([]Location{{Location: "web", Commands: []Command{{Name: "build", Command: "build"}}}}); err != nil {
			t.Fatal(err)
		}
		if err := f.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		authored, err := LoadAuthored(path)
		if err != nil {
			t.Fatal(err)
		}
		if authored.Root != "/abs/repo" {
			t.Errorf("Root = %q, want /abs/repo", authored.Root)
		}
		data, _ := os.ReadFile(path)
		if strings.Index(string(data), "root:") > strings.Index(string(data), "locations:") {
			t.Errorf("root should come before locations:\n%s", data)
		}
	})

	t.Run("existing file keeps comments and locations", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "repo.yaml")
		writeFile(t, path, "# my notes\nroot: /old\nlocations:\n  - location: web # keep me\n")
		f, err := OpenFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.SetRoot("/new"); err != nil {
			t.Fatal(err)
		}
		if err := f.Save(); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(path)
		for _, want := range []string{"# my notes", "# keep me", "root: /new", "location: web"} {
			if !strings.Contains(string(data), want) {
				t.Errorf("missing %q in:\n%s", want, data)
			}
		}
	})
}

func TestGlobalProjectFileIn(t *testing.T) {
	dir := t.TempDir()
	write := func(name, root string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("root: "+root+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if got, want := globalProjectFileIn(dir, "/src/api"), filepath.Join(dir, "api.yaml"); got != want {
		t.Errorf("fresh: got %q, want %q", got, want)
	}

	write("mine.yml", "/src/api")
	if got, want := globalProjectFileIn(dir, "/src/api"), filepath.Join(dir, "mine.yml"); got != want {
		t.Errorf("existing file for the same root: got %q, want %q", got, want)
	}

	write("web.yaml", "/work/web")
	got := globalProjectFileIn(dir, "/src/web")
	if got == filepath.Join(dir, "web.yaml") || !strings.HasPrefix(filepath.Base(got), "web-") || filepath.Ext(got) != ".yaml" {
		t.Errorf("name taken by another root: got %q, want web-<hash>.yaml", got)
	}
	if again := globalProjectFileIn(dir, "/src/web"); again != got {
		t.Errorf("collision name is not stable: %q then %q", got, again)
	}
}
