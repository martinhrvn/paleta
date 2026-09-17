package commands

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/martinhrvn/paleta/internal/config"
	"github.com/martinhrvn/paleta/internal/history"
)

// Run dispatches one plt invocation. args is os.Args without the program name.
// Everything scripts consume (listings, stats, the selection JSON) goes to
// stdout, messages go to stderr, and the result is the process exit code — so
// the whole command surface can be driven from a test.
func Run(version string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stdout)
		return 1
	}

	rest := args[1:]
	switch args[0] {
	case "init":
		return runInit(rest, stdout, stderr)
	case "edit":
		if err := EditConfig(); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	case "list":
		return runList(rest, stdout, stderr)
	case "stats":
		return runStats(rest, stdout, stderr)
	case "select":
		return runSelect(stdout, stderr)
	case "lint":
		return runLint(rest, stdout, stderr)
	case "record":
		return runRecord(rest, stderr)
	case "help":
		usage(stdout)
		return 0
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "paleta version %s\n", version)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown command: %s\n", args[0])
		usage(stdout)
		return 1
	}
}

// newFlagSet builds a subcommand's flag set. Parse errors are printed to stderr
// and returned rather than exiting, so Run stays in charge of the exit code.
func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("plt "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func runInit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("init", stderr)
	var force, template bool
	fs.BoolVar(&force, "force", false, "overwrite an existing .pltrc")
	fs.BoolVar(&force, "f", false, "shorthand for --force")
	fs.BoolVar(&template, "template", false, "write the static starter template instead of scanning")
	fs.BoolVar(&template, "t", false, "shorthand for --template")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if template {
		if err := CreateDefaultConfigWithForce(config.ConfigFileName, force); err != nil {
			fmt.Fprintf(stderr, "Error creating config: %v\n", err)
			return 1
		}
		printInitSuccess(stdout, config.ConfigFileName)
		return 0
	}

	// `plt init` is explicit about where it runs: it scans and writes here, unlike
	// a bare `plt`, which anchors to the repository root (Bootstrap).
	return reportInitWizard(".", force, stdout, stderr)
}

// reportInitWizard runs the init wizard at root and reports how it went.
func reportInitWizard(root string, force bool, stdout, stderr io.Writer) int {
	outcome, err := RunInitWizard(root, force)
	if err != nil {
		// A config that exists but can't be read gets the "here's the file, here's
		// the way out" treatment; anything else is a plain failure.
		var invalid *config.InvalidConfigError
		if errors.As(err, &invalid) {
			fmt.Fprint(stderr, configErrorMessage(err))
		} else {
			fmt.Fprintf(stderr, "Error: %v\n", err)
		}
		return 1
	}

	switch outcome {
	case InitWritten:
		printInitSuccess(stdout, filepath.Join(root, config.ConfigFileName))
	case InitNoProjects:
		fmt.Fprintln(stdout, "No projects detected in this directory tree.")
		fmt.Fprintln(stdout, "Run 'plt init --template' to start from a sample configuration.")
	case InitCanceled:
		fmt.Fprintln(stdout, "Init canceled.")
		return 1
	case InitNothingSelected:
		fmt.Fprintln(stdout, "No locations selected; .pltrc was not written.")
	}
	return 0
}

func printInitSuccess(stdout io.Writer, configPath string) {
	fmt.Fprintf(stdout, "Wrote config file: %s\n", configPath)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Next steps:")
	fmt.Fprintln(stdout, "  1. Review .pltrc and tweak locations as needed")
	fmt.Fprintln(stdout, "  2. Run 'plt list' to see available commands")
	fmt.Fprintln(stdout, "  3. Run 'plt select' to interactively select and run commands")
}

// configErrorMessage explains a config-load failure. The two cases worth
// spelling out are "there is no config" (say how to make one) and "this file is
// broken" (say which file, and how to get out of it) — discovery walks up from
// the working directory, so the user may never have seen the file it picked.
func configErrorMessage(err error) string {
	var invalid *config.InvalidConfigError
	switch {
	case errors.Is(err, config.ErrConfigNotFound):
		return "No paleta configuration found here.\nRun 'plt init' to scan this folder and create a .pltrc.\n"
	case errors.As(err, &invalid):
		return fmt.Sprintf("Can't load %s:\n  %v\nOpen it with 'plt edit', or run 'plt init --force' to replace it with a fresh scan.\n", invalid.Path, invalid.Err)
	default:
		return fmt.Sprintf("Error loading config: %v\n", err)
	}
}

func runList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("list", stderr)
	format := fs.String("format", "default", "output format: default or fzf")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Scripts consume this output, so the load waits for every project type
	// rather than printing a partial list.
	cfg, err := config.Load(config.LoadOptions{})
	if err != nil {
		fmt.Fprint(stderr, configErrorMessage(err))
		return 1
	}

	var rows []string
	if *format == "fzf" {
		rows = FormatForFzf(cfg)
	} else {
		rows = ListCommands(cfg)
	}
	for _, row := range rows {
		fmt.Fprintln(stdout, row)
	}
	return 0
}

// runStats prints recorded command history as a table. It reads only the
// history store, so it works even without a loadable .pltrc.
func runStats(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("stats", stderr)
	by := fs.String("by", string(SortFrecency), "ordering: frecency, count or recent")
	limit := fs.Int("limit", 0, "show at most N rows (0 = all)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opts := StatsOptions{Limit: *limit}
	switch StatsSort(*by) {
	case SortFrecency, SortCount, SortRecent:
		opts.By = StatsSort(*by)
	default:
		fmt.Fprintf(stderr, "Unknown stats ordering: %s\n", *by)
		fmt.Fprintln(stderr, "Usage: plt stats [--by=frecency|count|recent] [--limit=N]")
		return 2
	}

	hist, err := history.LoadOrCreateHistory(config.FindProjectRoot("."))
	if err != nil {
		fmt.Fprintf(stderr, "Error loading history: %v\n", err)
		return 1
	}
	hist.SetWeights(statsWeights())

	fmt.Fprintln(stdout, FormatStats(hist, opts, time.Now()))
	return 0
}

// statsWeights resolves the frecency weights to score stats with: the effective
// (global + local merged) project config, else the global config when there is
// no local .pltrc, else the balanced default. Project types stay deferred — only
// the frecency block matters here.
func statsWeights() history.FrecencyWeights {
	fr := config.DefaultFrecencyConfig()
	if cfg, err := config.Load(config.LoadOptions{Defer: true}); err == nil {
		fr = cfg.Frecency
	} else if gc, gerr := config.LoadGlobalConfig(); gerr == nil {
		fr = gc.Frecency
	}
	return history.NewWeights(fr.FrequencyWeight, fr.RecencyWeight)
}

// runSelect opens the selector and prints the selection as JSON on stdout for
// the shell wrapper; everything else goes to stderr. With no configuration at
// all it offers the init wizard first (see Bootstrap).
func runSelect(stdout, stderr io.Writer) int {
	// Deferred: shell-backed project types resolve in the background while the
	// palette is already open.
	cfg, err := config.Load(config.LoadOptions{Defer: true})
	if err != nil {
		var code int
		if cfg, code = bootstrapConfig(err, stderr); cfg == nil {
			return code
		}
	}

	results, err := RunSelector(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "Error with selection: %v\n", err)
		return 1
	}
	if len(results) == 0 {
		fmt.Fprintln(stderr, "No selection made")
		return 1
	}

	data, err := MarshalSelection(results)
	if err != nil {
		fmt.Fprintf(stderr, "Error encoding selection: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(data))
	return 0
}

// bootstrapConfig handles a failed load for `plt select`: when there is no
// configuration at all it runs the init wizard right there and loads what it
// wrote, so a first run goes from nothing to a working palette without a detour
// through `plt init`. Every other failure, and every case where the wizard
// didn't produce a config, yields a nil config and the exit code to use.
func bootstrapConfig(loadErr error, stderr io.Writer) (*config.Config, int) {
	if !errors.Is(loadErr, config.ErrConfigNotFound) {
		fmt.Fprint(stderr, configErrorMessage(loadErr))
		return nil, 1
	}

	outcome, err := Bootstrap()
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return nil, 1
	}

	switch outcome {
	case BootstrapWritten:
		cfg, err := config.Load(config.LoadOptions{Defer: true})
		if err != nil {
			fmt.Fprint(stderr, configErrorMessage(err))
			return nil, 1
		}
		fmt.Fprintf(stderr, "Wrote %s.\n", config.ConfigFileName)
		return cfg, 0
	case BootstrapCanceled:
		// Nothing to say: the user backed out of a wizard they just saw.
		return nil, 1
	case BootstrapNothingSelected:
		fmt.Fprintf(stderr, "No locations selected; %s was not written.\n", config.ConfigFileName)
		return nil, 1
	case BootstrapNoProjects:
		// We already scanned, so pointing at 'plt init' would just repeat it.
		fmt.Fprintln(stderr, "No paleta configuration found, and no projects detected in this directory tree.")
		fmt.Fprintln(stderr, "Run 'plt init --template' to start from a sample configuration.")
		return nil, 1
	}

	// Nowhere to scan ($HOME, above it, or the root): the plain hint.
	fmt.Fprint(stderr, configErrorMessage(loadErr))
	return nil, 1
}

// runLint checks the discovered .pltrc for names outside the alias-safe charset,
// unresolved aliases, deprecated keys and unknown tools. Without --fix it reports
// and exits non-zero when anything is found (CI-friendly). With --fix it rewrites
// the local .pltrc, then re-validates and reports what it could not repair.
func runLint(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("lint", stderr)
	fix := fs.Bool("fix", false, "rewrite offending names and deprecated keys in place")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *fix {
		return lintFix(stdout, stderr)
	}

	cfg, err := config.Load(config.LoadOptions{})
	if err != nil {
		fmt.Fprint(stderr, configErrorMessage(err))
		return 1
	}
	fmt.Fprintln(stdout, FormatLintReport(cfg.Warnings))
	if len(cfg.Warnings) > 0 {
		return 1
	}
	return 0
}

// lintFix repairs the local .pltrc. It requires a local file: a global-fallback
// config has no single file to rewrite.
func lintFix(stdout, stderr io.Writer) int {
	configPath, err := config.FindConfigFile()
	if err != nil {
		fmt.Fprintln(stderr, "No local .pltrc found to fix (plt lint --fix rewrites the nearest .pltrc).")
		return 1
	}

	fixes, err := FixConfigFile(configPath)
	if err != nil {
		// A file too broken to parse can't be repaired by rewriting names.
		fmt.Fprint(stderr, configErrorMessage(err))
		return 1
	}

	if len(fixes) == 0 {
		fmt.Fprintln(stdout, "No fixable issues found; nothing to fix.")
	} else {
		fmt.Fprintf(stdout, "Fixed %d item(s) in %s:\n", len(fixes), configPath)
		for _, f := range fixes {
			fmt.Fprintf(stdout, "  %-9s %q -> %q\n", f.Scope, f.Before, f.After)
		}
	}

	// Re-validate: some names (e.g. one whose leading character was disallowed)
	// can't be fully repaired by character replacement.
	cfg, err := config.Load(config.LoadOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "Error reloading config after fix: %v\n", err)
		return 1
	}
	if len(cfg.Warnings) > 0 {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, FormatLintReport(cfg.Warnings))
		return 1
	}
	return 0
}

// runRecord notes one command execution in the project's history. The shell
// wrapper calls it after every run.
func runRecord(args []string, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "Usage: plt record <location> <command>")
		return 1
	}
	RecordExecution(args[0], args[1])
	return 0
}

func usage(out io.Writer) {
	fmt.Fprintln(out, "paleta - a command palette for your monorepo")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "USAGE:")
	fmt.Fprintln(out, "    plt <command> [options]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "COMMANDS:")
	fmt.Fprintln(out, "    init                     Interactively scan for projects and build .pltrc")
	fmt.Fprintln(out, "    init --template          Write a static starter .pltrc template")
	fmt.Fprintln(out, "    init --template --force  Overwrite existing .pltrc with the template")
	fmt.Fprintln(out, "    edit                     Open nearest .pltrc in $EDITOR")
	fmt.Fprintln(out, "    list                     List all available location:command pairs")
	fmt.Fprintln(out, "    list --format=fzf        List commands in fzf format")
	fmt.Fprintln(out, "    stats                    Show command usage history (runs, recency, frecency)")
	fmt.Fprintln(out, "    stats --by=count         Sort stats by run count (or --by=recent)")
	fmt.Fprintln(out, "    select                   Interactive TUI command selection (multi-select with Tab)")
	fmt.Fprintln(out, "    lint                     Check .pltrc for names that can't be used as aliases")
	fmt.Fprintln(out, "    lint --fix               Rewrite offending characters in names to '_'")
	fmt.Fprintln(out, "    version                  Show the paleta version")
	fmt.Fprintln(out, "    help                     Show this help message")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "EXAMPLES:")
	fmt.Fprintln(out, "    plt init")
	fmt.Fprintln(out, "    plt init --template")
	fmt.Fprintln(out, "    plt list")
	fmt.Fprintln(out, "    plt list --format=fzf")
	fmt.Fprintln(out, "    plt select")
}
