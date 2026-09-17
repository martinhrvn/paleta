package parsers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ParserCommandTimeout bounds a single parser command. These commands are
// arbitrary shell (`./gradlew tasks --all`, `mvn help:describe`, a `make -qp`
// pipeline), so without a deadline one slow or hung project could stall every
// `plt` launch. A variable rather than a constant so tests can shorten it.
var ParserCommandTimeout = 30 * time.Second

// ErrParserTimeout reports a parser command abandoned at the deadline. Callers
// degrade to the type's base commands rather than failing the config load.
var ErrParserTimeout = errors.New("parser command timed out")

// parserWaitDelay is how long to wait for a killed parser's output pipes to
// close before giving up on them: a command that backgrounds a child
// (`sleep 30 &`) leaves the pipe open after the parent dies.
const parserWaitDelay = 1 * time.Second

// CommandParser executes a shell command to parse project commands
type CommandParser struct{}

func (c *CommandParser) ParseCommands(directory string, config ParserConfig) ([]string, error) {
	if config.ParserCommand == "" {
		return []string{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), ParserCommandTimeout)
	defer cancel()

	// Execute the parser command in the project directory
	cmd := exec.CommandContext(ctx, "sh", "-c", config.ParserCommand)
	cmd.Dir = directory
	cmd.WaitDelay = parserWaitDelay

	// Set up environment
	cmd.Env = os.Environ()

	// Execute the command
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("%w after %s: '%s'", ErrParserTimeout, ParserCommandTimeout, config.ParserCommand)
		}
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
