# paleta Project Structure

How the paleta codebase (the `plt` command) is laid out, what each package owns,
and the rules that keep the boundaries where they are.

## Directory layout

```
.
├── cmd/plt/main.go            # entry point: os.Exit(commands.Run(...)) and nothing else
├── internal/
│   ├── commands/              # the CLI: subcommand dispatch, flags, output, and the
│   │                          # orchestration of everything below
│   ├── config/                # the .pltrc model: load pipeline, in-place rewriting,
│   │                          # palette rows, project-root resolution
│   ├── ui/                    # bubbletea views: the selector and the init wizard
│   ├── parsers/               # project types: detection, command discovery, priority
│   ├── scan/                  # `plt init` tree scan (git-aware) over the parsers registry
│   ├── history/               # per-project command history and frecency scoring
│   └── mux/                   # tmux / zellij detection
├── plt-core.sh                # bash wrapper logic: runs the selection the binary returns
├── plt-integration.zsh        # zsh Ctrl+P widget doing the same inside the line editor
├── packaging/plt              # the installed `plt` wrapper (sources plt-core.sh)
├── completion.bash, completion.zsh
├── install.sh, flake.nix, .goreleaser.yaml
└── test-example/              # a sample repository with a .pltrc
```

## Dependency graph

```
cmd/plt ─► commands ─► ui ─► mux
              │         │
              │         └─► config, history   (types and pure helpers only)
              ├─► scan ─► parsers
              ├─► history
              └─► config ─► parsers
```

There are no cycles and no callbacks from a lower package into a higher one.
`ui` reaches the outside world only through `ui.Backend`, a value `commands`
fills in (history, reload, deferred-type resolution, focus and queue saves).

## Packages

### `internal/commands` — the CLI
`Run(version, args, stdout, stderr) int` is the whole program: one `case` per
subcommand, each with its own `flag.FlagSet`. Everything a subcommand needs is
called from here: `config.Load`, the selector (`RunSelector`), the init wizard
(`RunInitWizard`, `Bootstrap`), history recording, lint. Output goes to the
writers it is given, so the entire command surface is testable without a
process. Adding a subcommand means a case in `Run`, a line in `usage`, and the
shell-side entries listed in the memory note on new subcommands.

### `internal/config` — the `.pltrc` model
- **Loading**: `Load(LoadOptions{Workdir, Defer})` is the one way to get a
  usable configuration. It discovers the file (or a global project), runs the
  `loadStages` pipeline, resolves deferred types unless asked not to, and
  attaches tools. Nothing depends on the process working directory: relative
  paths and globs resolve against the config's own directory (`baseDir`).
  `Config.Reload()` repeats the same load. `LoadConfigFromDiscovery` is the
  discovery step alone, used by tests.
- **Rewriting**: `File` (`OpenFile`, `NewFile`) edits a `.pltrc` in place over
  its YAML node tree — `SetFocused`, `SetLocations`, `AppendCommand`,
  `AddLocation`, `Fix`, `Save`. Every write path (`plt init` merge, focus save,
  queue save, `plt lint --fix`) goes through it, so comments, key order and
  unmanaged keys survive. `LoadAuthored` reads the file as written.
- **Rows**: `Config.Rows(focusedOnly)` is the palette: each location's commands,
  a placeholder per location still resolving, tools last. `Location.DisplayName`
  is the one display-name rule; it is also the location half of a history key.
- **Roots**: `FindProjectRoot` (nearest `.pltrc` or `.git`, symlinks resolved)
  identifies a project for history and for where a bootstrap writes.
- Also here: glob expansion and overrides, `@project[type]:command` aliases,
  env merging, focus keys, frecency settings, warnings (`collectConfigWarnings`
  rebuilds `Warnings` from `loadWarnings` plus the name/alias checks, so order
  never loses one), tools, and the wizard's glob-collapse planner.

### `internal/ui` — the terminal views
- `selector.go`: the palette `Model`. It is in exactly one `selectorMode` at a
  time (`modeNormal`, `modeEdit`, `modeFocusPick`, `modeQueueEdit`,
  `modeQueueSave`); `handler()` is the single dispatch point for keys,
  non-key messages and the view. `listHeight` is the one chrome computation the
  viewport and renderer share. The loaded rows (`commands`) are never reordered;
  `filteredCommands` is derived, with match positions cached per row.
- `queue_editor.go`, `focus_picker.go`: sub-modes. `init_wizard.go`: a separate
  program the selector hands off to on Ctrl+N. `list.go`: cursor and viewport
  arithmetic shared by every list. `theme.go`: palette and styles.
- `backend.go`: `Backend`, the selector's only seam to the rest of the program.

### `internal/parsers` — project types
`ParsersFile` is the YAML (embedded defaults plus `~/.paleta/parsers.yaml`).
`Registry` (`DefaultRegistry`, `NewRegistry`) holds every `Type` in priority
order: `Lookup` for a declared type, `DetectTypes(dir)` for the types a folder
has. Both the config loader and the wizard scan go through it, so they agree.
`priority:` in the YAML orders types found in one folder; a type without one
sorts last. Parsers with a `parser_command` are deferred by the loader.

### `internal/scan`
Finds candidate directories for `plt init` from a git-aware file listing, then
asks the registry which types each one has.

### `internal/history`
Per-project history under `~/.paleta/history/<hash>.json`, keyed by
`displayName:command`, with frecency scoring. It knows nothing about roots or
configs; callers pass the project root.

## Rules that keep it this way
1. Only `commands` performs I/O orchestration; `ui` asks its `Backend`.
2. `.pltrc` is written only through `config.File`.
3. A location's display name comes only from `Location.DisplayName`.
4. Project types and their ranking come only from `parsers.Registry`.
5. A new selector sub-mode is a `selectorMode` value plus one `handler()` case.
6. `config.Load` is the only load entry point in production code.

## Build and test
```bash
go build ./... && go vet ./... && go test ./...
bash test_shell_fixes.sh          # wrapper behaviour
go build -o /tmp/plt ./cmd/plt && (cd test-example && /tmp/plt list)
```
