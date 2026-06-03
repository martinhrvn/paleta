# CLAUDE.md - gopm Development Checklist

## Project Overview
**gopm** (Go Project Manager) - A utility for quickly running commands in monorepos using fzf selection.

## Core Requirements

### Configuration File (.gopmrc)
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

### Command Selection
- [x] Integration with fzf for fuzzy selection
- [x] Display format: `[location-or-name] command`
- [x] Support for keyboard interrupts (Ctrl+C)
- [x] Show helpful error messages
- [ ] The cli interface should have following commands
  - [x] gopm list - output all available location:command pairs
  - [x] gopm list --format=fzf - format for fzf selection
  - [ ] gopm get --location=X --command=Y - get execution details as JSON
  - [x] gopm help - show usage and available commands
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
- [ ] ability to focus specific locations
  - either using .gopmrc.local.yaml (something like `only: location/foo` or command line argument
  - keyboard shortcut to focus/unfocus while searching
- [ ] aliases
- [ ] automatically detect type of a location based on presence of package.json/go.mod/etc.
- [x] history of executed commands
   - [x] store in a file (per project in ~/.gopm/history/)
   - [x] default sort by frecency of use (50/50 balance)
   - [x] allow to change sorting by keyboard shortcut (Ctrl+F in TUI mode)
   - [x] global config support (~/.config/gopm/config.yaml)
   - [x] per-project config override via .gopmrc
### Advanced Features
- [ ] Support for pre/post command hooks
- [ ] Support for command templates/variables
- [ ] Support for running commands in parallel
- [ ] Support for command groups/categories
- [ ] Integration with different package managers based on `type`
- [ ] Watch mode for repeated command execution
- [x] Shell completion (bash/zsh)
- [x] Zsh keyboard shortcut integration (Ctrl+P to launch gopm)

## Implementation Details

### Language & Dependencies
- [x] Written in Go
- [x] Minimal external dependencies (gopkg.in/yaml.v3, bubbletea, lipgloss, bubbles)
- [x] Use standard library where possible
- [ ] Ensure cross-platform compatibility (Linux/Mac/Windows)

### Distribution Strategy
- **Two-component approach**: Go binary (`gopm-bin`) + shell wrapper (`gopm`)
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
- [x] Enhanced shell wrapper script (packaging/gopm)
- [x] Installation script (install.sh)
- [x] Shell completion (bash/zsh)
- [x] Zsh keyboard shortcut integration (Ctrl+P, with multi-select support)
- [x] Auto-detection of binary location
- [x] Colored output and better UX
- [x] Nix flake with complete package and development environment
- [x] Cross-platform support (Linux/macOS)
- [ ] Usage documentation
- [ ] Example .gopmrc files
- [x] GitHub releases with binaries (GoReleaser + Actions on `v*` tags)
- [x] Homebrew formula (GoReleaser brews -> martinhrvn/homebrew-tap)
- [x] AUR package (GoReleaser-generated PKGBUILD for gopm-bin)
- [x] deb/rpm packages (GoReleaser nfpm)
- [x] CI pipeline (build/test/lint on push & PR)
- [x] Version injected via -ldflags (`gopm version`)

## Testing Checklist

### Unit Tests
- [x] Config file parsing tests
- [x] Glob pattern expansion tests
- [x] Project type parsing tests (npm, yarn, pnpm)
- [x] Error handling tests

### Integration Tests
- [x] Test with real monorepo structure
- [ ] Test with various .gopmrc formats
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
- [ ] Example .gopmrc for npm/yarn/pnpm monorepo
- [ ] Example .gopmrc for Go workspace
- [ ] Example .gopmrc for mixed-language monorepo
- [ ] Advanced configuration examples

### Parser Configuration System
- [x] Support ~/.gopm/parsers.yaml configuration file
- [x] Built-in parsers (package_json_scripts, go_standard)
- [x] Custom command-based parsers
- [x] Command templates with {key} substitution
- [x] Base commands always available
- [x] Multiple detection files per parser
- [x] Default embedded parser configurations
- [x] Parser command execution with proper error handling

### Global Project Configuration
- [x] Support for centralized project configs in `~/.config/gopm/projects/`
- [x] Per-project YAML files with root directory specification
- [x] Automatic matching based on current directory vs project root
- [x] Precedence: local .gopmrc takes priority over global configs
- [x] Global frecency settings still apply from `~/.config/gopm/config.yaml`
- [x] Closest match preferred when multiple projects match

## Release Checklist
- [x] Release automation (GoReleaser + GitHub Actions on `v*` tags)
- [x] Changelog generation (GoReleaser, conventional-commit filters)
- [x] Binary releases for major platforms (linux/darwin x amd64/arm64)
- [ ] Create `martinhrvn/homebrew-tap` repo + `HOMEBREW_TAP_TOKEN` secret (before first stable release)
- [ ] First publish of AUR `gopm-bin` PKGBUILD (manual, from release artifact)
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
- **History Tracking**: Per-project command execution history stored in `~/.gopm/history/`
- **Frecency Algorithm**: 50/50 balanced scoring (configurable)
  - Frequency score: `log(count + 1) × 100`
  - Recency score: `100 / (1 + days_since_access)`
- **Configuration**:
  - Global config: `~/.config/gopm/config.yaml`
  - Local override: `.gopmrc` in project root
  - Default: enabled with 50/50 balance
- **UI Integration**:
  - Standard fzf: Commands sorted by frecency (controlled by config)
  - Enhanced TUI: Ctrl+F to toggle frecency on/off
  - Visual indicator when frecency is enabled
- **Implementation**:
  - `internal/history/`: Core tracking, storage, and scoring
  - File locking for concurrent access safety
  - Automatic pruning (keeps last 1000 entries)
  - Project detection via .git or .gopmrc
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
  - `gopm select --tui`: Run the new TUI directly
  - `gopm tui`: Shell command for the new TUI
  - Ctrl+G (zsh): Keyboard shortcut for gpm TUI mode
- **Implementation**:
  - `internal/ui/fzf_tui_selector.go`: New TUI component
  - `internal/ui/fzf_tui_selector_test.go`: Unit tests (TDD)
  - Updated `gopm-core.sh` with `gpm_run` function
  - Updated `gopm-integration.zsh` with `__gpm_tui_select_widget`