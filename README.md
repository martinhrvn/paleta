# gopm - Go Project Manager

A fast, lightweight CLI tool for managing and executing commands across multiple projects in monorepos using fuzzy search.

## Features

- **Interactive Command Selection**: Use fuzzy search (via fzf or built-in TUI) to quickly find and run commands
- **Monorepo Support**: Manage commands across multiple projects in a single configuration
- **Project Type Detection**: Automatically discovers commands from package.json, go.mod, and other project files
- **Glob Pattern Support**: Define locations using glob patterns to match multiple directories
- **Enhanced Filtering**: Filter commands by location with an enhanced TUI mode
- **Flexible Configuration**: YAML-based configuration with support for custom parsers
- **Zero Dependencies Runtime**: Self-contained binary with optional fzf integration

## Installation

### Using the Install Script

```bash
# Clone the repository
git clone https://github.com/martin/go-pm
cd go-pm

# Run the installer
./install.sh
```

The installer will:
- Build and install the `gopm-bin` binary to `~/.local/bin`
- Install the `gopm` shell wrapper script
- Set up shell completion (bash/zsh)
- Add the installation directory to your PATH

### Using Nix

```bash
# Build and install using Nix flakes
nix build
nix profile install

# Or run directly
nix run
```

### Manual Installation

```bash
# Build the binary
go build -o gopm-bin ./cmd/gopm

# Copy to a location in your PATH
cp gopm-bin ~/.local/bin/
cp gopm.sh ~/.local/bin/gopm
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
    commands:                   # Optional: Additional custom commands
      - "command1"
      - "command2"
```

### Location Fields

- **name** (optional): Display name shown in the command selector
- **location** (required): Path to the project directory (supports glob patterns)
- **type** (optional): Project type for automatic command detection
- **commands** (optional): Additional commands to include

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

The gopm wrapper script automatically:
- Changes to the correct directory
- Executes the selected command
- Handles errors gracefully
- Provides colored output

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
      - "./deploy.sh production"
      - "./deploy.sh staging"
      - "./backup.sh"
```

### Custom Commands

```yaml
locations:
  - name: "api"
    location: "services/api"
    type: "npm"
    commands:
      - "docker-compose up"
      - "docker-compose down"
      - "make migrate"
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
├── gopm.sh           # Shell wrapper script
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
