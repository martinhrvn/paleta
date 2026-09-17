package commands

import (
	"os"
	"testing"

	"github.com/martinhrvn/paleta/internal/config"
)

// Helper to convert strings to Commands for tests
func stringsToCommands(strs []string) []config.Command {
	cmds := make([]config.Command, len(strs))
	for i, s := range strs {
		cmds[i] = config.Command{Name: "", Command: s}
	}
	return cmds
}

// writeGenerated renders locations into a fresh .pltrc at path the way the
// wizard does, and returns the written content.
func writeGenerated(t *testing.T, path string, locs []config.Location) string {
	t.Helper()
	f := config.NewFile(path)
	if err := f.SetLocations(locs); err != nil {
		t.Fatal(err)
	}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
