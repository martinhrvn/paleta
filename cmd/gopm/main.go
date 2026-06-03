package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/martin/go-pm/internal/commands"
	"github.com/martin/go-pm/internal/config"
	"github.com/martin/go-pm/internal/history"
)

func main() {
	if len(os.Args) < 2 {
		showUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		handleInitCommand()
	case "edit":
		handleEditCommand()
	case "list":
		handleListCommand()
	case "select":
		handleSelectCommand()
	case "record":
		handleRecordCommand()
	case "help":
		showUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		showUsage()
		os.Exit(1)
	}
}

func handleInitCommand() {
	// Check for force flag
	force := false
	for _, arg := range os.Args[2:] {
		if arg == "--force" || arg == "-f" {
			force = true
			break
		}
	}

	// Default config path
	configPath := ".gopmrc"

	// Create the config file
	err := commands.CreateDefaultConfigWithForce(configPath, force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created default config file: %s\n", configPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit .gopmrc to configure your project locations")
	fmt.Println("  2. Run 'gopm list' to see available commands")
	fmt.Println("  3. Run 'gopm select' to interactively select and run commands")
}

func handleEditCommand() {
	err := commands.EditConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handleListCommand() {
	// Check for format flag
	format := "default"
	if len(os.Args) > 2 && os.Args[2] == "--format=fzf" {
		format = "fzf"
	}

	// Load config from discovery
	cfg, err := config.LoadConfigFromDiscovery()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Generate command list
	var cmdList []string
	if format == "fzf" {
		cmdList = commands.FormatForFzf(cfg)
	} else {
		cmdList = commands.ListCommands(cfg)
	}

	// Output commands
	for _, cmd := range cmdList {
		fmt.Println(cmd)
	}
}

func handleSelectCommand() {
	// Load config from discovery
	cfg, err := config.LoadConfigFromDiscovery()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Run TUI selection
	results, err := commands.RunFzfTUI(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with selection: %v\n", err)
		os.Exit(1)
	}

	// Output JSON
	outputSelectionJSON(results)
}

// selectionJSON is the wire format emitted for a selection. The action field
// is only present when it carries meaning (currently just "edit").
type selectionJSON struct {
	Directory   string            `json:"directory"`
	Command     string            `json:"command"`
	DisplayName string            `json:"display_name"`
	Action      string            `json:"action,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

func toSelectionJSON(r commands.SelectionResult) selectionJSON {
	j := selectionJSON{
		Directory:   r.Directory,
		Command:     r.Command,
		DisplayName: r.DisplayName,
		Env:         r.Env,
	}
	// Preserve historical behavior: only "edit" surfaces an action field.
	if r.Action == "edit" {
		j.Action = "edit"
	}
	return j
}

// marshalSelection renders selection results as JSON. A single result is
// rendered as an object (for backward compatibility with the shell wrapper);
// multiple results are rendered as an array.
func marshalSelection(results []commands.SelectionResult) ([]byte, error) {
	if len(results) == 1 {
		return json.Marshal(toSelectionJSON(results[0]))
	}

	arr := make([]selectionJSON, len(results))
	for i, r := range results {
		arr[i] = toSelectionJSON(r)
	}
	return json.Marshal(arr)
}

func outputSelectionJSON(results []commands.SelectionResult) {
	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "No selection made")
		os.Exit(1)
	}

	data, err := marshalSelection(results)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding selection: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(data))
}

func handleRecordCommand() {
	// Expect arguments: gopm record <display_name> <command>
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: gopm record <location> <command>\n")
		os.Exit(1)
	}

	displayName := os.Args[2]
	command := os.Args[3]

	// Find project root
	projectRoot, err := history.FindProjectRoot(".")
	if err != nil {
		// Silently fail - history is optional
		return
	}

	// Load or create history
	hist, err := history.LoadOrCreateHistory(projectRoot)
	if err != nil {
		// Silently fail - history is optional
		return
	}

	// Record execution
	err = hist.RecordExecution(displayName, command)
	if err != nil {
		// Silently fail
		return
	}

	// Save history
	_ = hist.SaveToDefaultLocation()
}

func showUsage() {
	fmt.Println("gopm - Go Project Manager")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("    gopm <command> [options]")
	fmt.Println()
	fmt.Println("COMMANDS:")
	fmt.Println("    init                     Create a default .gopmrc configuration file")
	fmt.Println("    init --force             Overwrite existing .gopmrc file")
	fmt.Println("    edit                     Open nearest .gopmrc in $EDITOR")
	fmt.Println("    list                     List all available location:command pairs")
	fmt.Println("    list --format=fzf        List commands in fzf format")
	fmt.Println("    select                   Interactive TUI command selection (multi-select with Tab)")
	fmt.Println("    help                     Show this help message")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("    gopm init")
	fmt.Println("    gopm init --force")
	fmt.Println("    gopm list")
	fmt.Println("    gopm list --format=fzf")
	fmt.Println("    gopm select")
}
