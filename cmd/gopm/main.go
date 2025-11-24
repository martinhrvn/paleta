package main

import (
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
	// Check for enhanced flag
	enhanced := false
	for _, arg := range os.Args[2:] {
		if arg == "--enhanced" || arg == "-e" {
			enhanced = true
			break
		}
	}

	// Load config from discovery
	cfg, err := config.LoadConfigFromDiscovery()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Run fzf selection (enhanced or regular)
	var result *commands.SelectionResult
	if enhanced {
		result, err = commands.RunEnhancedFzf(cfg)
	} else {
		result, err = commands.RunFzf(cfg)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with selection: %v\n", err)
		os.Exit(1)
	}

	// Output as JSON for shell script parsing
	fmt.Printf(`{"directory":"%s","command":"%s","display_name":"%s"}`, result.Directory, result.Command, result.DisplayName)
	fmt.Println()
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
	fmt.Println("    list                     List all available location:command pairs")
	fmt.Println("    list --format=fzf        List commands in fzf format")
	fmt.Println("    select                   Interactive command selection with fzf")
	fmt.Println("    select --enhanced        Enhanced TUI selection with location filtering")
	fmt.Println("    help                     Show this help message")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("    gopm init")
	fmt.Println("    gopm init --force")
	fmt.Println("    gopm list")
	fmt.Println("    gopm list --format=fzf")
	fmt.Println("    gopm select")
	fmt.Println("    gopm select --enhanced")
}
