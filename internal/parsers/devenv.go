package parsers

import (
	"encoding/json"
	"fmt"
	"strings"
)

// devenvBuiltinTaskPrefix marks devenv's own lifecycle tasks (devenv:enterShell,
// devenv:files, ...), which are plumbing rather than something to run by hand.
const devenvBuiltinTaskPrefix = "devenv:"

// DevenvParser lists what a devenv.nix defines by asking devenv itself, so
// imports and modules are resolved exactly as devenv sees them:
//
//	up:<process>  → devenv up <process>
//	task:<task>   → devenv tasks run <task>
//	script:<name> → devenv shell <name>
//
// Scripts are prefixed so a `test` script doesn't shadow the `devenv test` base
// command. Evaluating devenv.nix is slow, so the type defers (see shellBacked).
type DevenvParser struct {
	NullParser
}

func (d *DevenvParser) shellBacked() {}

// BuildCommands evaluates the project's processes, scripts and tasks.
func (d *DevenvParser) BuildCommands(directory string, config ParserConfig) (map[string]string, error) {
	evalOut, err := runParserProcess(directory, "devenv", "eval", "processes", "scripts")
	if err != nil {
		return nil, err
	}
	var eval struct {
		Processes map[string]json.RawMessage `json:"processes"`
		Scripts   map[string]json.RawMessage `json:"scripts"`
	}
	if err := json.Unmarshal(evalOut, &eval); err != nil {
		return nil, fmt.Errorf("parsing devenv eval output: %w", err)
	}

	tasksOut, err := runParserProcess(directory, "devenv", "tasks", "list", "--json")
	if err != nil {
		return nil, err
	}
	var tasks []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(tasksOut, &tasks); err != nil {
		return nil, fmt.Errorf("parsing devenv tasks output: %w", err)
	}

	commands := make(map[string]string)
	for name := range eval.Processes {
		commands["up:"+name] = "devenv up " + name
	}
	for name := range eval.Scripts {
		commands["script:"+name] = "devenv shell " + name
	}
	for _, task := range tasks {
		if task.Name == "" || strings.HasPrefix(task.Name, devenvBuiltinTaskPrefix) {
			continue
		}
		commands["task:"+task.Name] = "devenv tasks run " + task.Name
	}
	return commands, nil
}
