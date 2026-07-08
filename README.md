# paleta — a command palette for your monorepo

[![CI](https://github.com/martinhrvn/paleta/actions/workflows/ci.yml/badge.svg)](https://github.com/martinhrvn/paleta/actions/workflows/ci.yml)
[![Release](https://github.com/martinhrvn/paleta/actions/workflows/release.yml/badge.svg)](https://github.com/martinhrvn/paleta/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A fast, lightweight CLI tool for managing and executing commands across multiple projects in monorepos using fuzzy search.

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

### Configuration File Format

plt looks for a `.pltrc` file starting from the current directory and traversing up the directory tree.

```yaml
locations:
  - name: "display-name"        # Optional: Display name in selection UI
    location: "path/to/project" # Required: Path to project directory
    type: "npm"                 # Optional: Project type, or a list: [npm, docker]
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
- **type** (optional): Project type(s) for automatic command detection. Accepts a
  single value (`type: npm`) or a list (`type: [npm, docker]`). When more than one
  type is given, each command is labelled with its type in the selector, e.g.
  `svc: [npm] build` and `svc: [docker] build`, so commands sharing a name stay
  distinguishable.
- **commands** (optional): Additional commands to include (supports both string and object formats)
- **include** (optional): Whitelist patterns for filtering commands (glob patterns)
- **exclude** (optional): Blacklist patterns for filtering commands (glob patterns)
- **env** (optional): Environment variables applied to every command in the location

### Supported Project Types

- **npm/yarn/pnpm**: Automatically discovers scripts from `package.json`
- **go**: Discovers standard Go commands (planned)
- **docker**: Detects a `Dockerfile` and offers `docker build` / `docker run`
- **compose**: Detects `docker-compose.yml`, `compose.yml`, and env-specific
  overrides matching `docker-compose.*.yml`; offers `docker compose up` / `down`
  / `build` / `logs` / `ps`

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

**List Format (chained commands):**

A command's `command` field can be a list instead of an inline `a && b` string.
The items are joined with `&&`, which is easier to read for longer chains:

```yaml
commands:
  - name: "ci-and-dev"
    command:
      - pnpm install
      - pnpm dev
    # runs as: pnpm install && pnpm dev
```

A single-item list is equivalent to a scalar. Saving a multi-command queue from
the selector (`s` in the queue editor) writes this list form; the list is
preserved when the config is rewritten (e.g. re-running `plt init`).

### Command References

A command string can reference another command instead of repeating it, using
`@project[type]:command`:

- **`@project`** — the target location, by its `name` (or the folder's base name).
- **`[type]`** — *optional*; only needed to disambiguate a location that declares
  more than one type (e.g. `[npm]` vs `[docker]`).
- **`:command`** — the referenced command's name.

References expand when the config loads, so they compose naturally with `&&`:

```yaml
locations:
  - name: web
    location: packages/web
    type: pnpm
  - name: api
    location: services/api
    type: go
    commands:
      # Referencing another project runs it in that project's directory
      # (wrapped in a subshell), then continues here:
      - name: ci
        command: "@web:build && go test ./..."
        # expands to: (cd '…/packages/web' && pnpm run build) && go test ./...
```

A same-project reference stays bare (`@web:build` inside `web` → `pnpm run build`).
An unresolvable reference — unknown project/command, an ambiguous multi-type
command with no `[type]`, or a reference cycle — is a hard error at load time, so
typos surface immediately.

Glob locations (`location: "packages/*"`) expand to one location per folder, each
named after its folder, so `@web:build` targets the specific `packages/web`.

When two folders share a name (e.g. `packages/search` **and** `services/search`),
the bare name `@search:build` is reported as ambiguous. Disambiguate with a path
tail — the project slot also accepts a path:

```
@packages/search:build   # the pnpm build in packages/search
@services/search:test    # the go test in services/search
```

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
        command: "docker compose up"
    include:
      - "test*"      # Include test commands from package.json
      - "build*"     # Include build commands
      - "local-*"    # Include our custom local-* commands
    exclude:
      - "*:watch"    # Exclude watch commands
      - "test:e2e"   # Exclude e2e tests
```

### Tools

Beyond location-scoped commands, you can surface **tools** — project-adjacent
programs like `lazygit` that aren't tied to a single location. Tools appear at the
**end** of the selector list and run in your **current directory**.

Enable tools in a project by listing them in `.pltrc`:

```yaml
tools:
  - lazygit
  - docker
```

The listed names resolve against a **built-in registry** plus any definitions in
your global config, so common tools work out of the box. Tools shipped built in:

| Tool         | Rows                                   |
| ------------ | -------------------------------------- |
| `lazygit`    | `lazygit`                              |
| `lazydocker` | `lazydocker`                           |
| `docker`     | `docker: up` / `down` / `logs` / `ps`  |

Define your own (or override a built-in of the same name) in the global config
`~/.config/paleta/config.yaml`. A tool can be a single command or a set of named
commands — each named command becomes its own selectable row (`docker: up`, …):

```yaml
tools:
  # single command -> one row labeled "gitui"
  gitui:
    command: gitui
  # multiple commands -> one row each: "compose: up", "compose: logs"
  compose:
    env:
      COMPOSE_PROJECT_NAME: myapp   # optional tool-level env (per-command env overrides)
    commands:
      - name: up
        command: docker compose up
      - name: logs
        command: docker compose logs -f
```

Enabling a name that is neither built in nor defined globally is a non-fatal
warning (shown in the selector banner and reported by `plt lint`); the rest of your
commands still load and run.

### Custom Parsers

Create `~/.paleta/parsers.yaml` to define custom command parsers:

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

Instead of having `.pltrc` files in each project, you can maintain centralized configurations in `~/.config/paleta/projects/`. This is useful for backing up your project configurations or managing them across multiple machines.

**Setup:**

Create project configuration files in `~/.config/paleta/projects/`:

```bash
mkdir -p ~/.config/paleta/projects
```

**Example Project Configuration:**

Create `~/.config/paleta/projects/my-project.yaml`:

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

1. plt first looks for `.pltrc` in the current directory and parent directories
2. If a local `.pltrc` is found, it takes priority (global frecency settings still apply)
3. If no local config is found, plt scans `~/.config/paleta/projects/*.yaml`
4. It finds the project whose `root` matches or is a parent of the current directory
5. If multiple projects match, the closest (most specific) one is used

**Benefits:**

- **Centralized Configuration**: Keep all project configs in one place for easy backup
- **No Per-Project Files**: Useful for projects where you can't or don't want to add a `.pltrc`
- **Multiple Machines**: Sync your `~/.config/paleta/projects/` across machines
- **Flexible Override**: Local `.pltrc` still takes priority when needed

**Global Settings:**

You can also configure global frecency settings in `~/.config/paleta/config.yaml`:

```yaml
frecency:
  enabled: true
  recency_weight: 0.5    # 0.0 to 1.0
  frequency_weight: 0.5  # 0.0 to 1.0
```

These settings apply to all projects unless overridden in a local `.pltrc`.

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
        command: "docker compose up"
      - name: "docker-down"
        command: "docker compose down"
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
│   ├── config/         # Configuration parsing and discovery
│   ├── commands/       # Command execution and formatting
│   ├── projecttypes/   # Project type parsers
│   └── ui/            # TUI components
├── examples/          # Example configurations
├── packaging/plt    # Shell wrapper script (installed as `plt`)
├── install.sh        # Installation script
└── flake.nix         # Nix flake configuration
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
