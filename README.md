# paleta — a command palette for your monorepo

[![CI](https://github.com/martinhrvn/paleta/actions/workflows/ci.yml/badge.svg)](https://github.com/martinhrvn/paleta/actions/workflows/ci.yml)
[![Release](https://github.com/martinhrvn/paleta/actions/workflows/release.yml/badge.svg)](https://github.com/martinhrvn/paleta/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A fast, lightweight CLI tool for managing and executing commands across multiple projects in monorepos using fuzzy search.

![plt demo: fuzzy-searching and running commands across a monorepo](docs/demo/demo.gif)

<sub>Recorded with [VHS](https://github.com/charmbracelet/vhs) from [`docs/demo/demo.tape`](docs/demo/demo.tape); re-render with `./docs/demo/render.sh`.</sub>

## Features

- **Interactive Command Selection**: Use fuzzy search (via fzf or built-in TUI) to quickly find and run commands
- **Zsh Keyboard Shortcut**: Press Ctrl+P from anywhere to launch plt (auto-configured during installation)
- **tmux / zellij Integration**: When running inside tmux or zellij, press **Ctrl+O** in the selector to run the command(s) in a new window/tab instead of the current shell
- **Global Project Configuration**: Centralized config management in `~/.config/paleta/projects/` with automatic project detection
- **Monorepo Support**: Manage commands across multiple projects in a single configuration
- **Project Type Detection**: Automatically discovers commands from package.json, go.mod, and other project files
- **Tools**: Surface project-adjacent programs (e.g. `lazygit`, `docker`) at the end of the list, defined via a built-in registry or global config
- **Glob Pattern Support**: Define locations using glob patterns to match multiple directories
- **Enhanced Filtering**: Filter commands by location with an enhanced TUI mode
- **Flexible Configuration**: YAML-based configuration with support for custom parsers
- **Zero Dependencies Runtime**: Self-contained binary with optional fzf integration

## Installation

> plt is two parts: the `plt-bin` binary (runs the TUI) and a `plt` shell
> wrapper that performs the actual `cd` + command execution. Every package below
> installs both, plus the `plt-core.sh` helper and shell completions.
> Runtime dependencies: **`jq`** and **`bash`**.

### Arch Linux (AUR)

```bash
yay -S plt-bin        # or: paru -S plt-bin
```

### Homebrew (macOS / Linux)

```bash
brew install martinhrvn/tap/plt
```

### Debian / Ubuntu (.deb) and Fedora / RHEL (.rpm)

Download the package for your architecture from the
[latest release](https://github.com/martinhrvn/paleta/releases/latest):

```bash
# Debian/Ubuntu
sudo dpkg -i paleta_*_linux_amd64.deb

# Fedora/RHEL
sudo rpm -i paleta_*_linux_amd64.rpm
```

### Pre-built binaries (tarball)

Download and extract the archive for your platform from the
[releases page](https://github.com/martinhrvn/paleta/releases/latest), then put
the extracted directory on your `PATH` (the `plt` wrapper finds its siblings):

```bash
tar -xzf paleta_*_linux_amd64.tar.gz
sudo cp -r paleta_* /opt/paleta
sudo ln -s /opt/paleta/plt /usr/local/bin/plt
```

### Using `go install`

Installs only the binary (`plt-bin`); use the wrapper from a release archive
or the repo for the full experience:

```bash
go install github.com/martinhrvn/paleta/cmd/plt@latest
```

### Using Nix

```bash
nix build           # build
nix profile install # install
nix run             # or run directly
```

### From source (install script)

```bash
git clone https://github.com/martinhrvn/paleta
cd paleta
./install.sh
```

The installer builds and installs the `plt-bin` binary plus the `plt`
wrapper to `~/.local/bin`, sets up bash/zsh completion, and configures the
zsh Ctrl+P keyboard shortcut.

### Manual build

```bash
go build -o plt-bin ./cmd/plt
cp plt-bin ~/.local/bin/
cp packaging/plt ~/.local/bin/plt
cp plt-core.sh ~/.local/bin/      # the wrapper sources this beside itself
chmod +x ~/.local/bin/plt
```

## Quick Start

1. Initialize a new configuration file in your project root:

```bash
plt init
```

This creates a `.pltrc` file with helpful examples and documentation.

2. Edit the `.pltrc` file to configure your project locations:

```yaml
locations:
  - name: "frontend"
    location: "packages/frontend"
    type: "npm"
    commands:
      - "npm run dev"
      - "npm run build"

  - name: "backend"
    location: "packages/backend"
    type: "npm"
    commands:
      - "npm run start"
      - "npm test"

  - location: "scripts"
    commands:
      - "./deploy.sh"
      - "./backup.sh"
```

3. Run plt:

```bash
# Interactive selection (default)
plt

# List all available commands
plt list

# Enhanced selection with location filtering
plt select --enhanced
```

### Appearance

The interactive palette uses a Catppuccin Mocha theme with 24-bit truecolor,
bold accents, a highlighted selection bar, and live fuzzy-match highlighting.
Use a terminal with truecolor support for the intended look.

Location and command rows are prefixed with [Nerd Font](https://www.nerdfonts.com/)
glyphs. If your terminal font is not a patched Nerd Font (you'll see missing
glyphs / tofu boxes), set `PLT_NO_ICONS=1` to fall back to plain ASCII:

```bash
export PLT_NO_ICONS=1
```

## Configuration

plt reads a `.pltrc` YAML file. It looks in the current directory first, then in each parent directory. For a repo where you'd rather not commit your personal config, `plt init --global` keeps it in `~/.config/paleta/projects/` instead (see [global project configuration](docs/pltrc.md#global-project-configuration)). A minimal config:

```yaml
locations:
  - location: "packages/*"   # one location per folder, commands detected from its files
  - name: "scripts"
    location: "scripts"
    commands:
      - name: "deploy"
        command: "./deploy.sh"
```

**The full format is documented in [docs/pltrc.md](docs/pltrc.md)**, including:

- every key
- project types
- glob locations and overrides
- `@project:command` references
- env vars
- filtering
- tools
- custom types
- global configs
- examples

## Usage

### Commands

```bash
# Initialize a new .pltrc configuration file
plt init

# Overwrite existing .pltrc file
plt init --force

# Interactive command selection and execution
plt
plt select

# Enhanced selection with location filtering
plt select --enhanced

# List all available commands
plt list

# List in fzf format
plt list --format=fzf

# Show command usage history (runs, recency, frecency score)
plt stats
plt stats --by=count      # or --by=recent
plt stats --limit=10

# Show help
plt help
```

The interactive selector's preview pane also shows per-command stats — run count,
last/first use, and frecency score — for any command you've run before.

### Selector Keyboard Shortcuts

Inside the interactive selector (`plt` / `plt select`):

| Key | Action |
| --- | --- |
| `Type` | Fuzzy-filter commands |
| `↑` / `↓` (or `Ctrl+K` / `Ctrl+J`) | Move the cursor |
| `Enter` | Run the selection (or queue) in the current shell |
| `Tab` | Add/remove the command under the cursor to the run queue |
| `Ctrl+O` | Run the selection in a new tmux window / zellij tab — **shown only when running inside tmux or zellij** |
| `Ctrl+E` | Edit the command before running |
| `Ctrl+Q` | Open the queue editor |
| `Ctrl+F` | Toggle frecency sorting |
| `Ctrl+P` | Pick which locations are focused |
| `Ctrl+W` | Switch to another git worktree: the same commands, re-rooted in that checkout — **shown only when the repository has more than one worktree** |
| `Ctrl+N` | Add projects (init wizard) |
| `Esc` / `Ctrl+C` | Clear the search, then cancel |

#### Run in a tmux window / zellij tab (Ctrl+O)

paleta auto-detects whether it is running inside tmux (`$TMUX`) or zellij
(`$ZELLIJ`). When it is, the selector's help line gains a `^O` hint and pressing
**Ctrl+O** launches the selected command — or the whole multi-select queue — in a
new tmux window (or new zellij pane, since zellij's CLI launches commands into
panes) rather than the current shell. The new window/tab starts in your current
directory and drops into an interactive shell once the command finishes, so its
output stays on screen. Set `PLT_PANE_SHELL` to change the shell it lands in.

When plt is not inside a multiplexer the shortcut is hidden and does nothing, so
the binding is only ever offered where it can work.

> **Note:** `Ctrl+O` is used instead of tmux's default prefix (`Ctrl+B`), which
> tmux intercepts before the selector can see it.

### Enhanced Mode

The enhanced mode (`--enhanced` flag) provides:
- Location filtering with multi-select
- Real-time command filtering
- Preview window showing command details
- Keyboard shortcuts for quick navigation

### Shell Integration

#### Wrapper Script

The plt wrapper script automatically:
- Changes to the correct directory
- Executes the selected command
- Handles errors gracefully
- Provides colored output

#### Zsh Keyboard Shortcut (Ctrl+P)

The installer automatically sets up zsh integration that allows you to launch plt from anywhere with a keyboard shortcut.

**Default Setup (Automatic):**

After running `./install.sh`, the integration is automatically configured in your `~/.zshrc`. Simply restart your shell or run:

```bash
source ~/.zshrc
```

Now press **Ctrl+P** to launch plt's interactive selector from anywhere!

**Customization:**

You can customize the behavior by setting environment variables in your `~/.zshrc`:

```bash
# Use a different keybinding (e.g., Ctrl+G instead of Ctrl+P)
export PLT_KEYBIND='^G'

# Use enhanced mode by default
export PLT_MODE='--enhanced'

# Custom binary location (if needed)
export PLT_BINARY="$HOME/custom/path/plt-bin"
```

**Manual Setup:**

If you need to set up the integration manually:

```bash
# Download the integration file
mkdir -p ~/.local/share/paleta
cp plt-integration.zsh ~/.local/share/paleta/

# Add to your .zshrc
echo '[ -f "$HOME/.local/share/paleta/plt-integration.zsh" ] && source "$HOME/.local/share/paleta/plt-integration.zsh"' >> ~/.zshrc

# Reload your shell
source ~/.zshrc
```

**Features:**

- Launch plt with a single keypress from anywhere
- Selected command is executed in the correct directory
- Command appears in your shell history
- Automatically records command usage for frecency sorting

## Examples

See [docs/pltrc.md#examples](docs/pltrc.md#examples).

## Dependencies

### Required

- **Go 1.24+**: For building from source
- **jq**: For JSON parsing in the shell wrapper

### Optional

- **fzf**: For enhanced fuzzy finding experience (falls back to built-in selector)
- **Nix**: For reproducible builds and development environment

## Development

### Building

```bash
# Build the binary
go build -o plt ./cmd/plt

# Run tests
go test ./...

# Build with Nix
nix build
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/config
go test ./internal/commands

# Run with coverage
go test -cover ./...
```

### Project Structure

```
.
├── cmd/
│   └── plt/           # Main application entry point
├── internal/
│   ├── commands/       # CLI: subcommand dispatch and orchestration
│   ├── config/         # .pltrc model: loading, in-place rewriting, palette rows
│   ├── ui/             # TUI: selector and init wizard
│   ├── parsers/        # project types: detection, command discovery, priority
│   ├── scan/           # `plt init` tree scan
│   ├── history/        # command history and frecency
│   └── mux/            # tmux / zellij detection
├── plt-core.sh        # wrapper logic (runs the selection the binary returns)
├── plt-integration.zsh # zsh Ctrl+P widget
├── packaging/plt      # Shell wrapper script (installed as `plt`)
├── install.sh         # Installation script
└── flake.nix          # Nix flake configuration
```

## Troubleshooting

### plt binary not found

Ensure `~/.local/bin` is in your PATH:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Add this to your `~/.bashrc` or `~/.zshrc` to make it permanent.

### jq not installed

Install jq using your package manager:

```bash
# Ubuntu/Debian
sudo apt-get install jq

# macOS
brew install jq

# Fedora/CentOS
sudo dnf install jq
```

### Config file not found

plt searches for `.pltrc` in the current directory and all parent directories. Create one in your project root.

### Commands not showing up

1. Check your `.pltrc` syntax with `plt list`
2. Verify the `location` paths exist
3. For type-based detection, ensure the config file exists (e.g., `package.json` for npm)

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

MIT License - see LICENSE file for details

## Acknowledgments

- Built with [go-fuzzyfinder](https://github.com/ktr0731/go-fuzzyfinder) for TUI selection
- Inspired by various project management tools for monorepos
