# paleta Project Structure

This document describes the organization of the paleta codebase.

## Directory Structure

```
.
├── cmd/
│   └── plt/
│       └── main.go              # Application entry point
├── internal/
│   ├── config/                  # Configuration management
│   │   ├── config.go           # Core config types and loading
│   │   ├── config_test.go      # Config loading tests
│   │   ├── config_integration_test.go # Integration tests
│   │   ├── discovery.go        # Config file discovery logic
│   │   ├── discovery_test.go   # Discovery tests
│   │   ├── glob.go            # Glob pattern expansion
│   │   └── glob_test.go       # Glob tests
│   ├── commands/               # Command execution and selection
│   │   ├── fzf.go             # Fuzzy finder integration
│   │   ├── fzf_test.go        # FZF tests
│   │   ├── fuzzy.go           # Custom fuzzy matching
│   │   ├── fuzzy_test.go      # Fuzzy matching tests
│   │   ├── list.go            # Command listing functionality
│   │   └── list_test.go       # List command tests
│   └── projecttypes/           # Project type implementations
│       ├── project_types.go    # Core interface and registry
│       ├── project_types_test.go # Project type tests
│       └── npm_project_type.go # npm/yarn/pnpm implementations
├── test-example/               # Test configuration examples
│   ├── .pltrc                # Example config file
│   └── package.json           # Example package.json
├── scripts/                   # Distribution and installation
│   ├── packaging/plt               # Enhanced shell wrapper
│   ├── install.sh            # Installation script
│   ├── completion.bash       # Bash completion
│   └── completion.zsh        # Zsh completion
└── docs/                     # Documentation
    ├── STRUCTURE.md          # This file
    └── NIX_USAGE.md         # Nix flake usage guide
```

## Package Organization

### `cmd/plt`
- **Purpose**: Application entry point
- **Responsibilities**: Command-line argument parsing, coordinating between internal packages
- **Key files**: `main.go`

### `internal/config`
- **Purpose**: Configuration management
- **Responsibilities**: 
  - YAML parsing and validation
  - Config file discovery (traversing up directory tree)
  - Glob pattern expansion
  - Project type integration
- **Key types**: `Config`, `Location`
- **Key functions**: `LoadConfig()`, `LoadConfigFromDiscovery()`, `ExpandGlobPatterns()`

### `internal/commands`
- **Purpose**: Command execution and selection logic
- **Responsibilities**:
  - Command listing and formatting
  - Fuzzy finder integration (using go-fuzzyfinder)
  - Custom fuzzy matching algorithms
  - JSON output for shell integration
- **Key types**: `SelectionResult`, `CommandInfo`
- **Key functions**: `ListCommands()`, `RunFzf()`, `ProcessFzfSelection()`

### `internal/projecttypes`
- **Purpose**: Project type detection and command parsing
- **Responsibilities**:
  - Project type interface definition
  - npm/yarn/pnpm implementations
  - Package.json parsing
  - Command prefix generation
- **Key types**: `ProjectType` interface, `NpmProjectType`, `YarnProjectType`, `PnpmProjectType`
- **Key functions**: `GetProjectType()`, `DiscoverProjectType()`, `ParseCommands()`

## Design Principles

### 1. **Clear Separation of Concerns**
Each package has a single, well-defined responsibility:
- `config` handles all configuration-related logic
- `commands` handles command selection and execution
- `projecttypes` handles project-specific integrations

### 2. **Internal Package Usage**
The `internal/` directory prevents external packages from importing paleta's internal APIs, following Go best practices for applications.

### 3. **Testability**
Each package has comprehensive tests, with clear separation between unit tests and integration tests.

### 4. **Minimal Dependencies**
- Standard library where possible
- External dependencies: `gopkg.in/yaml.v3`, `github.com/ktr0731/go-fuzzyfinder`
- Runtime dependency: `jq` (for shell integration)

## Build and Development

### Building
```bash
# Build the binary
go build -o plt ./cmd/plt

# Or use make/scripts
./packaging/plt  # Uses local binary
```

### Testing
```bash
# Run all tests
go test ./...

# Test specific packages
go test ./internal/config
go test ./internal/commands
go test ./internal/projecttypes
```

### Development
```bash
# Enter Nix development environment
nix develop

# Or use traditional Go tools
go run ./cmd/plt list
go run ./cmd/plt select
```

## Dependencies

### Runtime Dependencies
- **jq**: Used by shell wrapper for JSON parsing
- **bash**: Shell execution environment

### Go Dependencies
- **gopkg.in/yaml.v3**: YAML parsing
- **github.com/ktr0731/go-fuzzyfinder**: Fuzzy finder implementation

### Development Dependencies (Nix)
- **Go**: Language runtime
- **gopls**: Language server
- **golangci-lint**: Code linting
- **delve**: Debugger

## Future Extensions

The current structure makes it easy to add:

1. **New Project Types**: Add to `internal/projecttypes/`
2. **New Commands**: Add to `internal/commands/`
3. **New Config Formats**: Extend `internal/config/`
4. **New Selection Modes**: Extend `internal/commands/`

The modular design ensures changes in one area don't affect others, making the codebase maintainable and extensible.