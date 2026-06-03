package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/martinhrvn/go-pm/internal/config"
)

// GetEditor returns the editor to use, checking $EDITOR and falling back to vi
func GetEditor() string {
	editor := os.Getenv("EDITOR")
	if editor != "" {
		return editor
	}
	return "vi"
}

// FindConfigForEdit finds the nearest .gopmrc file path for editing
func FindConfigForEdit() (string, error) {
	configPath, err := config.FindConfigFile()
	if err != nil {
		return "", fmt.Errorf("no .gopmrc found: %w", err)
	}
	return configPath, nil
}

// EditConfig finds the nearest .gopmrc and opens it in the user's editor
func EditConfig() error {
	configPath, err := FindConfigForEdit()
	if err != nil {
		return err
	}

	editor := GetEditor()

	cmd := exec.Command(editor, configPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
