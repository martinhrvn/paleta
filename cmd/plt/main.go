package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/martinhrvn/paleta/internal/commands"
	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/history"
)

// version is the paleta version. It is overridden at build time via
// -ldflags "-X main.version=<tag>" (see .goreleaser.yaml / flake.nix).
var version = "dev"

// versionString renders the version banner printed by the version command.
func versionString() string {
	return fmt.Sprintf("paleta version %s", version)
}

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
	case "stats":
		handleStatsCommand()
	case "select":
		handleSelectCommand()
	case "lint":
		handleLintCommand()
	case "record":
		handleRecordCommand()
	case "help":
		showUsage()
	case "version", "--version", "-v":
		fmt.Println(versionString())
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		showUsage()
		os.Exit(1)
	}
}

func handleInitCommand() {
	// Parse flags
	force := false
	template := false
	for _, arg := range os.Args[2:] {
		switch arg {
		case "--force", "-f":
			force = true
		case "--template", "-t":
			template = true
		}
	}

	const configPath = ".pltrc"

	if template {
		// Legacy behavior: write the static starter template.
		if err := commands.CreateDefaultConfigWithForce(configPath, force); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating config: %v\n", err)
			os.Exit(1)
		}
		printInitSuccess(configPath)
		return
	}

	runInitWizard(configPath)
}

// runInitWizard scans for projects, lets the user pick which to include, and
// writes the resulting .pltrc. An existing config is loaded as the starting
// state so a repeat run shows and preserves what is already configured. The
// wizard logic is shared with the in-app "add projects" flow (Ctrl+N).
func runInitWizard(configPath string) {
	outcome, err := commands.RunInitWizard(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch outcome {
	case commands.InitWritten:
		printInitSuccess(configPath)
	case commands.InitNoProjects:
		fmt.Println("No projects detected in this directory tree.")
		fmt.Println("Run 'plt init --template' to start from a sample configuration.")
	case commands.InitCanceled:
		fmt.Println("Init canceled.")
		os.Exit(1)
	case commands.InitNothingSelected:
		fmt.Println("No locations selected; .pltrc was not written.")
	}
}

func printInitSuccess(configPath string) {
	fmt.Printf("Wrote config file: %s\n", configPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review .pltrc and tweak locations as needed")
	fmt.Println("  2. Run 'plt list' to see available commands")
	fmt.Println("  3. Run 'plt select' to interactively select and run commands")
}

func handleEditCommand() {
	err := commands.EditConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// exitConfigError reports a config-load failure and exits. When no configuration
// exists at all, it prints a friendly hint to run `plt init` instead of the raw
// error.
func exitConfigError(err error) {
	if errors.Is(err, config.ErrConfigNotFound) {
		fmt.Fprintln(os.Stderr, "No paleta configuration found here.")
		fmt.Fprintln(os.Stderr, "Run 'plt init' to scan this folder and create a .pltrc.")
	} else {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
	}
	os.Exit(1)
}

// attachTools resolves the config's enabled tools into rows the selector and
// `plt list` render at the end of the command list. It runs the tools in the
// user's current directory: discovery restores the original cwd before returning,
// so os.Getwd() here is where the user invoked plt (not the .pltrc directory).
func attachTools(cfg *config.Config) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	config.AttachTools(cfg, wd)
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
		exitConfigError(err)
	}
	attachTools(cfg)

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

// handleStatsCommand prints recorded command history as a table. It reads only
// the history store, so it works even without a loadable .pltrc.
func handleStatsCommand() {
	opts := commands.StatsOptions{By: commands.SortFrecency}
	for _, arg := range os.Args[2:] {
		switch {
		case arg == "--by=count":
			opts.By = commands.SortCount
		case arg == "--by=recent":
			opts.By = commands.SortRecent
		case arg == "--by=frecency":
			opts.By = commands.SortFrecency
		case strings.HasPrefix(arg, "--limit="):
			if n, err := strconv.Atoi(strings.TrimPrefix(arg, "--limit=")); err == nil && n > 0 {
				opts.Limit = n
			}
		default:
			fmt.Fprintf(os.Stderr, "Unknown stats option: %s\n", arg)
			fmt.Fprintln(os.Stderr, "Usage: plt stats [--by=frecency|count|recent] [--limit=N]")
			os.Exit(1)
		}
	}

	projectRoot, err := history.FindProjectRoot(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding project root: %v\n", err)
		os.Exit(1)
	}

	hist, err := history.LoadOrCreateHistory(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading history: %v\n", err)
		os.Exit(1)
	}
	hist.SetWeights(statsWeights())

	fmt.Println(commands.FormatStats(hist, opts, time.Now()))
}

// statsWeights resolves the frecency weights to score stats with. It prefers the
// effective (global + local merged) project config, falls back to the global
// config when there is no local .pltrc, and finally to the balanced default.
func statsWeights() history.FrecencyWeights {
	fr := config.DefaultFrecencyConfig()
	if cfg, err := config.LoadConfigFromDiscovery(); err == nil {
		fr = cfg.Frecency
	} else if gc, gerr := config.LoadGlobalConfig(); gerr == nil {
		fr = gc.Frecency
	}
	return history.NewWeights(fr.FrequencyWeight, fr.RecencyWeight)
}

func handleSelectCommand() {
	// Load config from discovery
	cfg, err := config.LoadConfigFromDiscovery()
	if err != nil {
		exitConfigError(err)
	}
	attachTools(cfg)

	// The local .pltrc path (if any) enables focus persistence and in-app
	// project adding. An empty path (global-fallback config) disables both.
	configPath, _ := config.FindConfigFile()

	// Run TUI selection
	results, err := commands.RunFzfTUI(cfg, configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with selection: %v\n", err)
		os.Exit(1)
	}

	// Output JSON
	outputSelectionJSON(results)
}

// handleLintCommand checks the discovered .pltrc for location/command names that
// fall outside the alias-safe charset. Without --fix it reports issues and exits
// non-zero when any exist (CI-friendly). With --fix it rewrites offending
// characters to '_' in the local .pltrc, then re-validates and reports any
// residual issues it could not fully repair.
func handleLintCommand() {
	fix := false
	for _, arg := range os.Args[2:] {
		switch arg {
		case "--fix":
			fix = true
		default:
			fmt.Fprintf(os.Stderr, "Unknown lint option: %s\n", arg)
			fmt.Fprintln(os.Stderr, "Usage: plt lint [--fix]")
			os.Exit(1)
		}
	}

	if fix {
		lintFix()
		return
	}

	cfg, err := config.LoadConfigFromDiscovery()
	if err != nil {
		exitConfigError(err)
	}
	// Resolve tools so unknown enabled tools are reported alongside name/alias issues.
	attachTools(cfg)
	fmt.Println(commands.FormatLintReport(cfg.Warnings))
	if len(cfg.Warnings) > 0 {
		os.Exit(1)
	}
}

// lintFix rewrites out-of-charset names in the local .pltrc. It requires a local
// file (a global-fallback config has no single file to rewrite).
func lintFix() {
	configPath, err := config.FindConfigFile()
	if err != nil {
		fmt.Fprintln(os.Stderr, "No local .pltrc found to fix (plt lint --fix rewrites the nearest .pltrc).")
		os.Exit(1)
	}

	fixes, err := commands.FixConfigFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fixing config: %v\n", err)
		os.Exit(1)
	}

	if len(fixes) == 0 {
		fmt.Println("No name issues found; nothing to fix.")
	} else {
		fmt.Printf("Fixed %d name(s) in %s:\n", len(fixes), configPath)
		for _, f := range fixes {
			fmt.Printf("  %-9s %q -> %q\n", f.Scope, f.Before, f.After)
		}
	}

	// Re-validate: some names (e.g. one whose leading character was disallowed)
	// can't be fully repaired by character replacement. Reload via discovery so
	// glob/relative paths resolve from the config directory.
	cfg, err := config.LoadConfigFromDiscovery()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reloading config after fix: %v\n", err)
		os.Exit(1)
	}
	if len(cfg.Warnings) > 0 {
		fmt.Println()
		fmt.Println(commands.FormatLintReport(cfg.Warnings))
		os.Exit(1)
	}
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
	// Only surface an action field when it carries meaning (edit / pane); the
	// default execute action stays absent for backward compatibility.
	if r.Action == "edit" || r.Action == "pane" {
		j.Action = r.Action
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
	// Expect arguments: plt record <display_name> <command>
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: plt record <location> <command>\n")
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
	fmt.Println("paleta - a command palette for your monorepo")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("    plt <command> [options]")
	fmt.Println()
	fmt.Println("COMMANDS:")
	fmt.Println("    init                     Interactively scan for projects and build .pltrc")
	fmt.Println("    init --template          Write a static starter .pltrc template")
	fmt.Println("    init --template --force  Overwrite existing .pltrc with the template")
	fmt.Println("    edit                     Open nearest .pltrc in $EDITOR")
	fmt.Println("    list                     List all available location:command pairs")
	fmt.Println("    list --format=fzf        List commands in fzf format")
	fmt.Println("    stats                    Show command usage history (runs, recency, frecency)")
	fmt.Println("    stats --by=count         Sort stats by run count (or --by=recent)")
	fmt.Println("    select                   Interactive TUI command selection (multi-select with Tab)")
	fmt.Println("    lint                     Check .pltrc for names that can't be used as aliases")
	fmt.Println("    lint --fix               Rewrite offending characters in names to '_'")
	fmt.Println("    version                  Show the paleta version")
	fmt.Println("    help                     Show this help message")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("    plt init")
	fmt.Println("    plt init --template")
	fmt.Println("    plt list")
	fmt.Println("    plt list --format=fzf")
	fmt.Println("    plt select")
}
