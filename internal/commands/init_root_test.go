package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhrvn/paleta/internal/config"
)

// TestRunInitWizard_PathsAreRelativeToRoot: the wizard scans and writes at one
// root, wherever it is invoked from. Ctrl+N inside the selector runs from the
// user's working directory but writes the .pltrc that was discovered — possibly
// several levels up — and the locations in it have to mean the same thing as the
// ones already there.
func TestRunInitWizard_PathsAreRelativeToRoot(t *testing.T) {
	root := chdirTemp(t)
	writePackage(t, filepath.Join(root, "services", "api"), "api")
	sub := filepath.Join(root, "services", "api")
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	stubWizard(t, true)

	outcome, err := RunInitWizard(root, false)
	if err != nil {
		t.Fatalf("RunInitWizard() error = %v", err)
	}
	if outcome != InitWritten {
		t.Fatalf("outcome = %v, want InitWritten", outcome)
	}

	written, err := os.ReadFile(filepath.Join(root, config.ConfigFileName))
	if err != nil {
		t.Fatalf("no config at the root: %v", err)
	}
	if !strings.Contains(string(written), "services/api") {
		t.Errorf("config describes the working directory, not the root:\n%s", written)
	}
	if _, err := os.Stat(filepath.Join(sub, config.ConfigFileName)); !os.IsNotExist(err) {
		t.Error("wrote a second config in the working directory")
	}
}
