# paleta — Development Checklist

## Project Overview
**paleta** — a command palette for your monorepo (invoked via the `plt` command), using fuzzy selection.

## Core Requirements

### Configuration File (.pltrc)
- [x] Support YAML-like format parsing
- [x] Support for `locations` array
- [x] Support for `name` field (optional display name)
- [x] Support for `location` field (path, can include globs)
- [x] Support for `type` field (optional, for package manager context)
  - initially support `npm`, `yarn`, `pnpm`, `go`
  - this basically means in the folder it will automatically find the appropriate package manager config file
  - generate list of commands based on the type
- [x] Support for `commands` array
  - this can be specified as a list of commands, if type is specified for project it will add this as extra commands
- [x] Support glob patterns in location paths (e.g., `packages/bar/*`)
    - This should be simple eg. expand * to all directories in the path, but not recurse into subdirectories
- [x] Config file discovery (search current dir and parents)
- [x] Validate config file structure
- [x] Handle malformed config gracefully
- [x] Validate location/command names against the alias-safe charset; the selector shows a warning banner and underlines/recolors offending rows, and `plt lint` reports them (see `plt lint` under CLI commands)
- [x] Surface unresolved `@project:command` alias references (recorded on `Command.Error`) alongside name issues — flagged in the selector and reported by `plt lint` (not auto-fixable; must be edited manually)

### Command Selection
- [x] Integration with fzf for fuzzy selection
- [x] Display format: `[location-or-name] command`
- [x] Support for keyboard interrupts (Ctrl+C)
- [x] Show helpful error messages
- [ ] The cli interface should have following commands
  - [x] plt list - output all available location:command pairs
  - [x] plt list --format=fzf - format for fzf selection
  - [ ] plt get --location=X --command=Y - get execution details as JSON
  - [x] plt help - show usage and available commands
  - [x] plt lint [--fix] - check `.pltrc` for location/command names outside the alias-safe charset; `--fix` rewrites offending characters to `_` (comments/formatting preserved via yaml.Node round-trip)
  - [x] Handle command-line argument parsing

### Command Execution
The go part should handle config parsing, command selection, the execution part should be done in some shell script.
- [x] Change directory to the specified location before execution
- [x] Execute the selected command
- [x] Handle command failures appropriately
- [x] Support complex commands with arguments
- [x] Shell script integration for execution


### Project type implementation
The project types we should support intitally are:
- [x] npm - should find package.json and run npm commands
  - [x] should allow to set the package manager to use, eg. npm, yarn, pnpm
- [ ] go - should find go.mod and run go commands


## Nice-to-Have Features

### Enhanced Configuration
- [x] Support for environment variables in commands
  - [x] env definable at location and command level (command overrides location)
  - [x] ${VAR}/$VAR expansion against ambient + sibling vars (undefined → empty)
- [ ] Support for command aliases/shortcuts
- [ ] Support for default commands per location
- [ ] Support for inheriting/extending configurations
- [ ] Support JSON/TOML formats as alternatives
- [ ] Support for comments in config file

### Enhanced UI/UX
- [x] Preview window in fzf showing command details
- [x] Most recently used (MRU) commands at top (via frecency)
- [x] Command history tracking
- [ ] Dry-run mode to preview what will be executed
- [ ] Verbose mode for debugging
- [ ] Do not show locations that do not exist
- [x] ability to focus specific locations
  - [x] top-level `focused:` list of location keys in `.pltrc`; selector defaults to focused-only when the list is non-empty
  - [x] `Ctrl+T` toggles the focus filter (focused-only ↔ all) for the current session
  - [x] `Ctrl+P` picker to set/unset which locations are focused (with `Ctrl+A` toggle-all), persisted to `.pltrc`
  - [x] `Ctrl+N` adds projects on the fly via the `plt init` wizard, then re-enters the selector
- [x] command queue for multi-select (deterministic run order)
  - [x] `Tab` enqueues the command under the cursor; the queue records selection order and persists across searches (shown as position badges in the list + `N queued` in the status)
  - [x] `Ctrl+Q` opens a queue editor: `Shift+↑/↓` reorder, `x`/`Del` remove, `Enter` run, `Esc` back
  - [x] save a queue to `.pltrc` as a command (`s` in the editor): a multi-command queue is written as a YAML list, a single command as a scalar
  - [x] a `.pltrc` `command` may be authored as a YAML list; the items are joined with `&&` (more readable than an inline `a && b` string) and the list form is preserved across rewrites
  - [x] cross-project save: a queue spanning folders saves under the root location; each part is an alias (which `cd`-wraps cross-project) or, when it can't be referenced, a `cd`-wrapped raw command
- [x] command references / aliases: `@project[type]:command` tokens in a `.pltrc` command expand at load time to the referenced command's resolved string (e.g. `@web[npm]:build` → `pnpm run build`)
  - [x] `[type]` is optional; required only to disambiguate a multi-type project
  - [x] a cross-project reference wraps in `(cd '<dir>' && …)` so it runs in the right directory; same-project refs stay bare
  - [x] unresolvable references (unknown project/command, ambiguous multi-type, cycles) are recorded per-command (`Command.Error`) and shown as an error entry in `plt list` rather than failing the whole config load — one bad saved chain never blocks the rest
  - [x] "save queue to `.pltrc`" emits reference tokens (`@web:build && @web:dev`) verified against the resolver, so a token can never expand to a different command than intended; falls back to the raw string (or a `[type]`-qualified token) when a bare reference wouldn't round-trip — e.g. names with spaces, unnamed commands, or an untyped/typed name clash
- [x] automatically detect type of a location based on presence of package.json/go.mod/etc.
  - [x] interactive `plt init` wizard: scans the tree (git-aware, skips gitignored/`node_modules`/etc.), multi-select detected projects, generates `.pltrc`
  - [x] repeatable: existing `.pltrc` loaded as starting state (configured locations pre-selected & tagged, merged on save)
  - [x] static template preserved behind `plt init --template`
  - [ ] (future) drill into a selected folder to include/exclude individual detected commands
- [x] support multiple types per location (`type: [npm, docker]`); commands from all types are merged, and when a location has >1 type each command is labelled with its type in the selector (e.g. `svc: [npm] build` / `svc: [docker] build`)
  - [x] `plt init` auto-detects all matching types per folder and generates multi-type locations
  - [ ] (future) per-type toggling within a folder in the wizard
- [x] history of executed commands
   - [x] store in a file (per project in ~/.paleta/history/)
   - [x] default sort by frecency of use (50/50 balance)
   - [x] allow to change sorting by keyboard shortcut (Ctrl+F in TUI mode)
   - [x] global config support (~/.config/paleta/config.yaml)
   - [x] per-project config override via .pltrc
   - [x] surface usage stats: `plt stats` table (runs / last used / frecency, with `--by` and `--limit`, wired into the shell wrapper + completions) and run/recency/score in the `plt select` preview pane
   - [x] wire configurable `recency_weight`/`frequency_weight` (global + local, normalized) into actual scoring — previously hard-coded 50/50
   - [x] fix: history was saved under an empty project-root hash when no prior file existed, so counts never accumulated across runs
### Advanced Features
- [ ] Support for pre/post command hooks
- [ ] Support for command templates/variables
- [ ] Support for running commands in parallel
- [ ] Support for command groups/categories
- [ ] Integration with different package managers based on `type`
- [ ] Watch mode for repeated command execution
- [x] Shell completion (bash/zsh)
- [x] Zsh keyboard shortcut integration (Ctrl+P to launch plt)

## Implementation Details

### Language & Dependencies
- [x] Written in Go
- [x] Minimal external dependencies (gopkg.in/yaml.v3, bubbletea, lipgloss, bubbles)
- [x] Use standard library where possible
- [ ] Ensure cross-platform compatibility (Linux/Mac/Windows)

### Distribution Strategy
- **Two-component approach**: Go binary (`plt-bin`) + shell wrapper (`plt`)
- **Flexible installation**: Auto-detects binary location in multiple paths
- **Shell integration**: Bash/zsh completion and colored output
- **Easy installation**: Single script installation with `install.sh`
- **User-friendly**: No need to manually manage binary paths

### Code Structure
- [x] Clear separation of concerns
- [x] Organized folder structure (cmd/, internal/)
- [x] Config parsing module (internal/config)
- [x] Command execution module (internal/commands)
- [x] FZF integration module (internal/commands)
- [x] Project type system (internal/projecttypes)
- [x] Error handling throughout
- [x] Unit tests for core functionality

### Distribution
- [x] Enhanced shell wrapper script (packaging/plt)
- [x] Installation script (install.sh)
- [x] Shell completion (bash/zsh)
- [x] Zsh keyboard shortcut integration (Ctrl+P, with multi-select support)
- [x] Auto-detection of binary location
- [x] Colored output and better UX
- [x] Nix flake with complete package and development environment
- [x] Cross-platform support (Linux/macOS)
- [ ] Usage documentation
- [ ] Example .pltrc files
- [x] GitHub releases with binaries (GoReleaser + Actions on `v*` tags)
- [x] Homebrew formula (GoReleaser brews -> martinhrvn/homebrew-tap)
- [x] AUR package (GoReleaser-generated PKGBUILD for plt-bin)
- [x] deb/rpm packages (GoReleaser nfpm)
- [x] CI pipeline (build/test/lint on push & PR)
- [x] Version injected via -ldflags (`plt version`)

## Testing Checklist

### Unit Tests
- [x] Config file parsing tests
- [x] Glob pattern expansion tests
- [x] Project type parsing tests (npm, yarn, pnpm)
- [x] Error handling tests

### Integration Tests
- [x] Test with real monorepo structure
- [ ] Test with various .pltrc formats
- [ ] Test with missing fzf
- [x] Test with malformed configs
- [ ] Test with non-existent locations

### Edge Cases
- [x] Empty config file
- [ ] No commands defined
- [x] Invalid glob patterns
- [ ] Commands with special characters
- [ ] Very long command lists
- [ ] Nested monorepo structures

## Documentation

### README.md
- [ ] Clear installation instructions
- [ ] Usage examples
- [ ] Configuration format documentation
- [ ] Troubleshooting section
- [ ] Contributing guidelines

### Examples
- [ ] Example .pltrc for npm/yarn/pnpm monorepo
- [ ] Example .pltrc for Go workspace
- [ ] Example .pltrc for mixed-language monorepo
- [ ] Advanced configuration examples

### Parser Configuration System
- [x] Support ~/.paleta/parsers.yaml configuration file
- [x] Built-in parsers (package_json_scripts, go_standard)
- [x] Custom command-based parsers
- [x] Command templates with {key} substitution
- [x] Base commands always available
- [x] Multiple detection files per parser
- [x] Default embedded parser configurations
- [x] Parser command execution with proper error handling

### Global Project Configuration
- [x] Support for centralized project configs in `~/.config/paleta/projects/`
- [x] Per-project YAML files with root directory specification
- [x] Automatic matching based on current directory vs project root
- [x] Precedence: local .pltrc takes priority over global configs
- [x] Global frecency settings still apply from `~/.config/paleta/config.yaml`
- [x] Closest match preferred when multiple projects match

## Release Checklist
- [x] Release automation (GoReleaser + GitHub Actions on `v*` tags)
- [x] Changelog generation (GoReleaser, conventional-commit filters)
- [x] Binary releases for major platforms (linux/darwin x amd64/arm64)
- [ ] Create `martinhrvn/homebrew-tap` repo + `HOMEBREW_TAP_TOKEN` secret (before first stable release)
- [ ] First publish of AUR `plt-bin` PKGBUILD (manual, from release artifact)
- [ ] Tag and push `v0.1.0`

## Current Status
- [x] Initial concept defined
- [x] Basic implementation started
- [x] Core features working
- [x] Project type system implemented (npm, yarn, pnpm)
- [x] Shell script integration complete
- [x] Testing complete
- [x] Parser configuration system implemented
- [ ] Documentation complete
- [ ] First release

## Notes for Development
- Start with MVP: config parsing, fzf selection, command execution
- Iterate based on real usage patterns
- Keep the tool fast and responsive
- Maintain backwards compatibility with config format

## Recent Updates

### Frecency Sorting (Completed)
Added intelligent command ranking based on frequency and recency:
- **History Tracking**: Per-project command execution history stored in `~/.paleta/history/`
- **Frecency Algorithm**: 50/50 balanced scoring (configurable)
  - Frequency score: `log(count + 1) × 100`
  - Recency score: `100 / (1 + days_since_access)`
- **Configuration**:
  - Global config: `~/.config/paleta/config.yaml`
  - Local override: `.pltrc` in project root
  - Default: enabled with 50/50 balance
- **UI Integration**:
  - Standard fzf: Commands sorted by frecency (controlled by config)
  - Enhanced TUI: Ctrl+F to toggle frecency on/off
  - Visual indicator when frecency is enabled
- **Implementation**:
  - `internal/history/`: Core tracking, storage, and scoring
  - File locking for concurrent access safety
  - Automatic pruning (keeps last 1000 entries)
  - Project detection via .git or .pltrc
- **Shell Integration**: Automatic recording after command selection

### fzf-style TUI with Multi-Select (Completed)
Added a new fzf-like TUI interface with improved UX:
- **New Layout**: Single-column command list (70%) + preview panel (30%)
  - Removed the locations sidebar for cleaner interface
  - Preview shows: Location, Path, Command, Type, and Frecency score
- **Multi-Select Support**:
  - Tab: Toggle selection on current item
  - Ctrl+A: Select/deselect all
  - Enter: Confirm selection(s)
  - Multiple selections are joined with `&&` for sequential execution
- **Keyboard Navigation**:
  - Ctrl+j / Down arrow: Move cursor down
  - Ctrl+k / Up arrow: Move cursor up
  - Type to filter (input has focus by default)
  - Esc/Ctrl+C: Cancel
- **Integration**:
  - `plt select --tui`: Run the new TUI directly
  - `plt tui`: Shell command for the new TUI
  - Ctrl+G (zsh): Keyboard shortcut for gpm TUI mode
- **Implementation**:
  - `internal/ui/fzf_tui_selector.go`: New TUI component
  - `internal/ui/fzf_tui_selector_test.go`: Unit tests (TDD)
  - Updated `plt-core.sh` with `gpm_run` function
  - Updated `plt-integration.zsh` with `__gpm_tui_select_widget`

### Palette Theme Refresh — Catppuccin Mocha (Completed)
Modernized the `plt select` TUI look. Truecolor already worked (forced via
`lipgloss.SetColorProfile(termenv.TrueColor)` + rendering to `/dev/tty`); the
flat look came from muted 256-palette colors with no effects.
- **Catppuccin Mocha** truecolor theme (single palette block in
  `internal/ui/fzf_tui_selector.go`, shared by the wizard/queue/focus views)
- Bold accents + faint secondary text
- Lavender accent bar on the selected row (surface-filled, gap-free)
- Live fuzzy-match highlighting via `fuzzySubsequenceIndices` + `highlightMatches`
- Nerd Font glyphs (folder/terminal/`❯`) with `PLT_NO_ICONS=1` ASCII fallback
- Tests: `fuzzySubsequenceIndices`, `highlightMatches`, theme sanity (TDD)