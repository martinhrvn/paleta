package scan

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile creates a file (and parent dirs) with the given content.
func writeFile(t *testing.T, path string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create dir %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}

// findCandidate returns the candidate with the given RelPath, or nil.
func findCandidate(cands []Candidate, relPath string) *Candidate {
	for i := range cands {
		if cands[i].RelPath == relPath {
			return &cands[i]
		}
	}
	return nil
}

func TestScan_DetectsMultipleTypes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "packages", "frontend", "package.json"))
	writeFile(t, filepath.Join(root, "services", "api", "go.mod"))

	cands, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	frontend := findCandidate(cands, filepath.Join("packages", "frontend"))
	if frontend == nil {
		t.Fatal("expected a candidate for packages/frontend")
	}
	if frontend.Type != "npm" {
		t.Errorf("expected frontend type npm, got %q", frontend.Type)
	}

	api := findCandidate(cands, filepath.Join("services", "api"))
	if api == nil {
		t.Fatal("expected a candidate for services/api")
	}
	if api.Type != "go" {
		t.Errorf("expected api type go, got %q", api.Type)
	}
}

func TestScan_JSPackageManagerByLockfile(t *testing.T) {
	tests := []struct {
		name     string
		lockfile string
		wantType string
	}{
		{"bare package.json", "", "npm"},
		{"pnpm lockfile", "pnpm-lock.yaml", "pnpm"},
		{"yarn lockfile", "yarn.lock", "yarn"},
		{"npm lockfile", "package-lock.json", "npm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "app", "package.json"))
			if tt.lockfile != "" {
				writeFile(t, filepath.Join(root, "app", tt.lockfile))
			}

			cands, err := Scan(root)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}

			app := findCandidate(cands, "app")
			if app == nil {
				t.Fatal("expected a candidate for app")
			}
			if app.Type != tt.wantType {
				t.Errorf("expected type %q, got %q", tt.wantType, app.Type)
			}
		})
	}
}

func TestScan_Dockerfile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "svc", "Dockerfile"))

	cands, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	svc := findCandidate(cands, "svc")
	if svc == nil {
		t.Fatal("expected a candidate for svc")
	}
	if svc.Type != "docker" {
		t.Errorf("expected svc type docker, got %q", svc.Type)
	}
}

func TestScan_ComposeGlobOverrideFile(t *testing.T) {
	root := t.TempDir()
	// Only an env-specific override file, no plain docker-compose.yml.
	writeFile(t, filepath.Join(root, "infra", "docker-compose.prod.yml"))

	cands, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	infra := findCandidate(cands, "infra")
	if infra == nil {
		t.Fatal("expected a candidate for infra")
	}
	if infra.Type != "compose" {
		t.Errorf("expected infra type compose, got %q", infra.Type)
	}
}

func TestScan_ComposeBeatsDockerfile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "app", "Dockerfile"))
	writeFile(t, filepath.Join(root, "app", "docker-compose.yml"))

	cands, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	app := findCandidate(cands, "app")
	if app == nil {
		t.Fatal("expected a candidate for app")
	}
	if app.Type != "compose" {
		t.Errorf("expected app type compose (compose outranks docker), got %q", app.Type)
	}
}

func TestScan_SkipsIgnoredDirs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "node_modules", "foo", "package.json"))
	writeFile(t, filepath.Join(root, "vendor", "bar", "go.mod"))
	writeFile(t, filepath.Join(root, "real", "package.json"))

	cands, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if findCandidate(cands, filepath.Join("node_modules", "foo")) != nil {
		t.Error("node_modules content should be skipped")
	}
	if findCandidate(cands, filepath.Join("vendor", "bar")) != nil {
		t.Error("vendor content should be skipped")
	}
	if findCandidate(cands, "real") == nil {
		t.Error("expected real to be detected")
	}
}

func TestScan_RootProject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"))

	cands, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	rootCand := findCandidate(cands, ".")
	if rootCand == nil {
		t.Fatal("expected a candidate for root (.)")
	}
	if rootCand.Type != "go" {
		t.Errorf("expected root type go, got %q", rootCand.Type)
	}
}

func TestScan_EmptyTree(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"))

	cands, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(cands) != 0 {
		t.Errorf("expected no candidates, got %d", len(cands))
	}
}

func TestScan_SortedRootFirst(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"))
	writeFile(t, filepath.Join(root, "zzz", "go.mod"))
	writeFile(t, filepath.Join(root, "aaa", "go.mod"))

	cands, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(cands) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(cands))
	}
	want := []string{".", "aaa", "zzz"}
	for i, w := range want {
		if cands[i].RelPath != w {
			t.Errorf("candidate %d: expected %q, got %q", i, w, cands[i].RelPath)
		}
	}
}

func TestDetectTypeMap_FromParsers(t *testing.T) {
	m, err := detectTypeMap()
	if err != nil {
		t.Fatalf("detectTypeMap failed: %v", err)
	}
	if m["go.mod"] != "go" {
		t.Errorf("expected go.mod -> go, got %q", m["go.mod"])
	}
	if m["Cargo.toml"] != "rust" {
		t.Errorf("expected Cargo.toml -> rust, got %q", m["Cargo.toml"])
	}
}
