package history

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"
)

func TestProjectKeyIsShortSHA256(t *testing.T) {
	root := "/home/someone/proj/app"
	sum := sha256.Sum256([]byte(root))
	want := hex.EncodeToString(sum[:])[:16]
	if got := ProjectKey(root); got != want {
		t.Fatalf("ProjectKey(%q) = %q, want %q", root, got, want)
	}
	if ProjectKey(root) == ProjectKey(root+"/sub") {
		t.Fatal("different roots must have different keys")
	}
}

func TestDataDirAndHistoryPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got, want := DataDir(), filepath.Join(home, ".paleta"); got != want {
		t.Fatalf("DataDir() = %q, want %q", got, want)
	}
	root := "/some/project"
	want := filepath.Join(home, ".paleta", "history", ProjectKey(root)+".json")
	if got := GetHistoryPath(root); got != want {
		t.Fatalf("GetHistoryPath(%q) = %q, want %q", root, got, want)
	}
}
