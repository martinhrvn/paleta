package parsers

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CommandParser executes a shell command to parse project commands
type CommandParser struct{}

func (c *CommandParser) ParseCommands(directory string, config ParserConfig) ([]string, error) {
	if config.ParserCommand == "" {
		return []string{}, nil
	}

	// Execute the parser command in the project directory
	cmd := exec.Command("sh", "-c", config.ParserCommand)
	cmd.Dir = directory

	// Set up environment
	cmd.Env = os.Environ()

	// Execute the command
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute parser command '%s': %w", config.ParserCommand, err)
	}

	// Parse the output - each line is a command
	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return []string{}, nil
	}

	lines := strings.Split(outputStr, "\n")
	var commands []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			commands = append(commands, line)
		}
	}

	return commands, nil
}
