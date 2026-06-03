# gopm - Go Project Manager

[![CI](https://github.com/martinhrvn/go-pm/actions/workflows/ci.yml/badge.svg)](https://github.com/martinhrvn/go-pm/actions/workflows/ci.yml)
[![Release](https://github.com/martinhrvn/go-pm/actions/workflows/release.yml/badge.svg)](https://github.com/martinhrvn/go-pm/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A fast, lightweight CLI tool for managing and executing commands across multiple projects in monorepos using fuzzy search.

## Features

- **Interactive Command Selection**: Use fuzzy search (via fzf or built-in TUI) to quickly find and run commands
- **Zsh Keyboard Shortcut**: Press Ctrl+P from anywhere to launch gopm (auto-configured during installation)
- **Global Project Configuration**: Centralized config management in `~/.config/gopm/projects/` with automatic project detection
- **Monorepo Support**: Manage commands across multiple projects in a single configuration
- **Project Type Detection**: Automatically discovers commands from package.json, go.mod, and other project files
- **Glob Pattern Support**: Define locations using glob patterns to match multiple directories
- **Enhanced Filtering**: Filter commands by location with an enhanced TUI mode
- **Flexible Configuration**: YAML-based configuration with support for custom parsers
- **Zero Dependencies Runtime**: Self-contained binary with optional fzf integration

## Installation

> gopm is two parts: the `gopm-bin` binary (runs the TUI) and a `gopm` shell
> wrapper that performs the actual `cd` + command execution. Every package below
> installs both, plus the `gopm-core.sh` helper and shell completions.
> Runtime dependencies: **`jq`** and **`bash`**.

### Arch Linux (AUR)

```bash
yay -S gopm-bin        # or: paru -S gopm-bin
```

### Homebrew (macOS / Linux)

```bash
brew install martinhrvn/tap/gopm
```

### Debian / Ubuntu (.deb) and Fedora / RHEL (.rpm)

Download the package for your architecture from the
[latest release](https://github.com/martinhrvn/go-pm/releases/latest):

```bash
# Debian/Ubuntu
sudo dpkg -i gopm_*_linux_amd64.deb

# Fedora/RHEL
sudo rpm -i gopm_*_linux_amd64.rpm
```

### Pre-built binaries (tarball)

Download and extract the archive for your platform from the
[releases page](https://github.com/martinhrvn/go-pm/releases/latest), then put
the extracted directory on your `PATH` (the `gopm` wrapper finds its siblings):

```bash
tar -xzf gopm_*_linux_amd64.tar.gz
sudo cp -r gopm_* /opt/gopm
sudo ln -s /opt/gopm/gopm /usr/local/bin/gopm
```

### Using `go install`

Installs only the binary (`gopm-bin`); use the wrapper from a release archive
or the repo for the full experience:

```bash
go install github.com/martinhrvn/go-pm/cmd/gopm@latest
```

### Using Nix

```bash
nix build           # build
nix profile install # install
nix run             # or run directly
```

### From source (install script)

```bash
git clone https://github.com/martinhrvn/go-pm
cd go-pm
./install.sh
```

The installer builds and installs the `gopm-bin` binary plus the `gopm`
wrapper to `~/.local/bin`, sets up bash/zsh completion, and configures the
zsh Ctrl+P keyboard shortcut.

### Manual build

```bash
go build -o gopm-bin ./cmd/gopm
cp gopm-bin ~/.local/bin/
cp packaging/gopm ~/.local/bin/gopm
cp gopm-core.sh ~/.local/bin/      # the wrapper sources this beside itself
chmod +x ~/.local/bin/gopm
```

## Quick Start

1. Initialize a new configuration file in your project root:

```bash
gopm init
```

This creates a `.gopmrc` file with helpful examples and documentation.

2. Edit the `.gopmrc` file to configure your project locations:

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

3. Run gopm:

```bash
# Interactive selection (default)
gopm

# List all available commands
gopm list

# Enhanced selection with location filtering
gopm select --enhanced
```

## Configuration

### Configuration File Format

gopm looks for a `.gopmrc` file starting from the current directory and traversing up the directory tree.

```yaml
locations:
  - name: "display-name"        # Optional: Display name in selection UI
    location: "path/to/project" # Required: Path to project directory
    type: "npm"                 # Optional: Project type (npm, yarn, pnpm, go)
    env:                        # Optional: Env vars for all commands here
      NODE_ENV: "development"
    commands:                   # Optional: Additional custom commands
      - "npm run legacy"        # String format (backward compatible)
      - name: "build"           # Object format with name
        command: "npm run build"
      - name: "test"
        command: "npm test"
        env:                    # Optional: per-command env (overrides location)
          CI: "true"
```

### Location Fields

- **name** (optional): Display name shown in the command selector
- **location** (required): Path to the project directory (supports glob patterns)
- **type** (optional): Project type for automatic command detection
- **commands** (optional): Additional commands to include (supports both string and object formats)
- **include** (optional): Whitelist patterns for filtering commands (glob patterns)
- **exclude** (optional): Blacklist patterns for filtering commands (glob patterns)
- **env** (optional): Environment variables applied to every command in the location

### Supported Project Types

- **npm/yarn/pnpm**: Automatically discovers scripts from `package.json`
- **go**: Discovers standard Go commands (planned)

### Glob Patterns

Use glob patterns to match multiple directories:

```yaml
locations:
  - location: "packages/*"      # Matches all directories in packages/
    type: "npm"

  - location: "apps/*/backend"  # Matches backend dirs in all apps
    commands:
      - "npm start"
```

### Command Formats

Commands can be specified in two formats, which can be mixed within the same location:

**String Format (Legacy):**
```yaml
commands:
  - "npm run build"
  - "npm test"
  - "./deploy.sh"
```

**Object Format (Recommended):**
```yaml
commands:
  - name: "build"
    command: "npm run build"
  - name: "test"
    command: "npm test"
  - name: "deploy"
    command: "./deploy.sh"
```

**Benefits of Named Commands:**
- **Better UI**: Command names appear clearly in the selector
- **Filtering**: Use `include` and `exclude` patterns to filter by name
- **Clarity**: Easier to understand what each command does

### Environment Variables

Set environment variables at the **location** level (applied to every command in
that location) or the **command** level (applied to just that command). When the
same key is defined at both levels, the command-level value wins.

```yaml
locations:
  - location: "api"
    env:
      BIN: "${HOME}/.local/bin"   # references the ambient environment
      PATH: "${BIN}:$PATH"        # references a sibling var + ambient
      PORT: "3000"
    commands:
      - name: "dev"
        command: "npm run dev"
        env:
          PORT: "3001"            # overrides the location-level PORT
```

**Expansion rules:**
- Values may reference the ambient process environment (`${HOME}`, `$PATH`).
- Values may reference other variables defined in the same scope (siblings),
  in any order.
- An undefined variable expands to an empty string.

Variables are applied only while the selected command runs — they do not leak
into your shell session, and in multi-select each command gets its own env.

**Mixed Format Example:**
```yaml
locations:
  - location: "frontend"
    type: "npm"
    commands:
      - name: "dev"
        command: "npm run dev -- --host 0.0.0.0"
      - "npm run build"  # Legacy format still works
```

### Command Filtering

Filter auto-discovered commands using `include` and `exclude` patterns:

```yaml
locations:
  - location: "packages/api"
    type: "npm"
    include:
      - "test*"     # Only include commands starting with "test"
      - "build"     # And the "build" command
    exclude:
      - "*:watch"   # But exclude any watch commands
```

**Pattern Matching:**
- Patterns match against command **names** (if present) or command strings
- Supports glob patterns (`*`, `?`, etc.)
- Include patterns act as a whitelist (if specified)
- Exclude patterns act as a blacklist (applied after include)

**Example:**
```yaml
locations:
  - location: "services/*"
    type: "npm"
    commands:
      - name: "local-deploy"
        command: "docker-compose up"
    include:
      - "test*"      # Include test commands from package.json
      - "build*"     # Include build commands
      - "local-*"    # Include our custom local-* commands
    exclude:
      - "*:watch"    # Exclude watch commands
      - "test:e2e"   # Exclude e2e tests
```

### Custom Parsers

Create `~/.gopm/parsers.yaml` to define custom command parsers:

```yaml
parsers:
  custom_parser:
    detect_files:
      - "custom.json"
    base_commands:
      - "custom build"
      - "custom test"
    command_parser:
      type: "command"
      command: "jq -r '.scripts | keys[]' {file}"
      template: "npm run {key}"
```

### Global Project Configuration

Instead of having `.gopmrc` files in each project, you can maintain centralized configurations in `~/.config/gopm/projects/`. This is useful for backing up your project configurations or managing them across multiple machines.

**Setup:**

Create project configuration files in `~/.config/gopm/projects/`:

```bash
mkdir -p ~/.config/gopm/projects
```

**Example Project Configuration:**

Create `~/.config/gopm/projects/my-project.yaml`:

```yaml
root: /home/user/projects/my-project
locations:
  - name: "frontend"
    location: "packages/frontend"
    type: "npm"
  - name: "backend"
    location: "packages/backend"
    type: "npm"
```

**How It Works:**

1. gopm first looks for `.gopmrc` in the current directory and parent directories
2. If a local `.gopmrc` is found, it takes priority (global frecency settings still apply)
3. If no local config is found, gopm scans `~/.config/gopm/projects/*.yaml`
4. It finds the project whose `root` matches or is a parent of the current directory
5. If multiple projects match, the closest (most specific) one is used

**Benefits:**

- **Centralized Configuration**: Keep all project configs in one place for easy backup
- **No Per-Project Files**: Useful for projects where you can't or don't want to add a `.gopmrc`
- **Multiple Machines**: Sync your `~/.config/gopm/projects/` across machines
- **Flexible Override**: Local `.gopmrc` still takes priority when needed

**Global Settings:**

You can also configure global frecency settings in `~/.config/gopm/config.yaml`:

```yaml
frecency:
  enabled: true
  recency_weight: 0.5    # 0.0 to 1.0
  frequency_weight: 0.5  # 0.0 to 1.0
```

These settings apply to all projects unless overridden in a local `.gopmrc`.

## Usage

### Commands

```bash
# Initialize a new .gopmrc configuration file
gopm init

# Overwrite existing .gopmrc file
gopm init --force

# Interactive command selection and execution
gopm
gopm select

# Enhanced selection with location filtering
gopm select --enhanced

# List all available commands
gopm list

# List in fzf format
gopm list --format=fzf

# Show help
gopm help
```

### Enhanced Mode

The enhanced mode (`--enhanced` flag) provides:
- Location filtering with multi-select
- Real-time command filtering
- Preview window showing command details
- Keyboard shortcuts for quick navigation

### Shell Integration

#### Wrapper Script

The gopm wrapper script automatically:
- Changes to the correct directory
- Executes the selected command
- Handles errors gracefully
- Provides colored output

#### Zsh Keyboard Shortcut (Ctrl+P)

The installer automatically sets up zsh integration that allows you to launch gopm from anywhere with a keyboard shortcut.

**Default Setup (Automatic):**

After running `./install.sh`, the integration is automatically configured in your `~/.zshrc`. Simply restart your shell or run:

```bash
source ~/.zshrc
```

Now press **Ctrl+P** to launch gopm's interactive selector from anywhere!

**Customization:**

You can customize the behavior by setting environment variables in your `~/.zshrc`:

```bash
# Use a different keybinding (e.g., Ctrl+G instead of Ctrl+P)
export GOPM_KEYBIND='^G'

# Use enhanced mode by default
export GOPM_MODE='--enhanced'

# Custom binary location (if needed)
export GOPM_BINARY="$HOME/custom/path/gopm-bin"
```

**Manual Setup:**

If you need to set up the integration manually:

```bash
# Download the integration file
mkdir -p ~/.local/share/gopm
cp gopm-integration.zsh ~/.local/share/gopm/

# Add to your .zshrc
echo '[ -f "$HOME/.local/share/gopm/gopm-integration.zsh" ] && source "$HOME/.local/share/gopm/gopm-integration.zsh"' >> ~/.zshrc

# Reload your shell
source ~/.zshrc
```

**Features:**

- Launch gopm with a single keypress from anywhere
- Selected command is executed in the correct directory
- Command appears in your shell history
- Automatically records command usage for frecency sorting

## Examples

### NPM Monorepo

```yaml
locations:
  - name: "web-app"
    location: "apps/web"
    type: "npm"

  - name: "mobile-app"
    location: "apps/mobile"
    type: "npm"

  - location: "packages/*"
    type: "npm"
```

### Mixed Project Types

```yaml
locations:
  - name: "frontend"
    location: "frontend"
    type: "npm"

  - name: "backend"
    location: "backend"
    type: "go"

  - name: "scripts"
    location: "scripts"
    commands:
      - name: "deploy-prod"
        command: "./deploy.sh production"
      - name: "deploy-staging"
        command: "./deploy.sh staging"
      - name: "backup"
        command: "./backup.sh"
```

### Custom Commands with Filtering

```yaml
locations:
  - name: "api"
    location: "services/api"
    type: "npm"
    commands:
      - name: "docker-up"
        command: "docker-compose up"
      - name: "docker-down"
        command: "docker-compose down"
      - name: "migrate"
        command: "make migrate"
    include:
      - "test*"      # Include test commands from package.json
      - "build"      # Include build command
      - "docker-*"   # Include our custom docker commands
      - "migrate"    # Include migrate command
    exclude:
      - "test:e2e"   # Exclude e2e tests
```

### Advanced Example with Mixed Formats

```yaml
locations:
  - name: "frontend"
    location: "apps/frontend"
    type: "npm"
    commands:
      # Named commands for better clarity
      - name: "dev-local"
        command: "npm run dev -- --host 0.0.0.0 --port 3000"
      - name: "dev-remote"
        command: "npm run dev -- --host 0.0.0.0 --port 3000 --public"
      # Legacy string format still works
      - "npm run storybook"
    include:
      - "build*"     # Include all build variants
      - "test:unit"  # Include unit tests only
      - "dev-*"      # Include our custom dev commands
    exclude:
      - "test:e2e"   # Exclude e2e tests

  - name: "backend"
    location: "apps/backend"
    type: "npm"
    commands:
      - name: "debug"
        command: "node --inspect-brk index.js"
      - name: "migrate-up"
        command: "npm run migrate:up"
      - name: "migrate-down"
        command: "npm run migrate:down"
```

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
go build -o gopm ./cmd/gopm

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
│   └── gopm/           # Main application entry point
├── internal/
│   ├── config/         # Configuration parsing and discovery
│   ├── commands/       # Command execution and formatting
│   ├── projecttypes/   # Project type parsers
│   └── ui/            # TUI components
├── examples/          # Example configurations
├── packaging/gopm    # Shell wrapper script (installed as `gopm`)
├── install.sh        # Installation script
└── flake.nix         # Nix flake configuration
```

## Troubleshooting

### gopm binary not found

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

gopm searches for `.gopmrc` in the current directory and all parent directories. Create one in your project root.

### Commands not showing up

1. Check your `.gopmrc` syntax with `gopm list`
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
