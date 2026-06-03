# Nix Usage Guide for paleta

This project includes a comprehensive Nix flake for easy installation and development.

## Installation

### Using Nix Flakes

Install directly from the repository:

```bash
# Install to your profile
nix profile install github:martinhrvn/paleta

# Or run without installing
nix run github:martinhrvn/paleta
```

### Development

Enter the development environment:

```bash
# Clone the repository
git clone https://github.com/martinhrvn/paleta
cd paleta

# Enter development shell
nix develop

# Or use direnv (rename .envrc-flake to .envrc)
mv .envrc-flake .envrc
direnv allow
```

## Available Packages

The flake provides several packages:

- `plt` - Complete package with binary and wrapper (default)
- `plt-bin` - Just the Go binary
- `plt-wrapper` - Just the shell wrapper
- `plt-completions` - Shell completions for bash/zsh

## Usage Examples

### Run plt

```bash
# Interactive command selection
nix run .#plt

# List available commands
nix run .#plt -- list

# Show help
nix run .#plt -- help
```

### Development Commands

```bash
# Build the binary
nix build .#plt-bin

# Build everything
nix build .#plt

# Run tests
nix develop -c go test ./...

# Format code
nix develop -c go fmt ./...
```

## Nix Environment Features

The development environment includes:

- **Go** - Latest stable version
- **jq** - For JSON parsing in shell scripts
- **gopls** - Go Language Server
- **golangci-lint** - Go linter
- **delve** - Go debugger

## Shell Completions

The flake automatically installs shell completions:

- **Bash**: `$out/share/bash-completion/completions/plt`
- **Zsh**: `$out/share/zsh/site-functions/_plt`

## Integration with NixOS

Add to your NixOS configuration:

```nix
# configuration.nix
{
  environment.systemPackages = with pkgs; [
    (import (fetchTarball "https://github.com/martinhrvn/paleta/archive/main.tar.gz")).packages.${system}.plt
  ];
}
```

Or using flakes in your system configuration:

```nix
# flake.nix
{
  inputs.plt.url = "github:martinhrvn/paleta";
  
  outputs = { self, nixpkgs, plt, ... }: {
    nixosConfigurations.myhost = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        {
          environment.systemPackages = [ plt.packages.x86_64-linux.plt ];
        }
      ];
    };
  };
}
```

## Home Manager Integration

```nix
# home.nix
{
  home.packages = with pkgs; [
    (import (fetchTarball "https://github.com/martinhrvn/paleta/archive/main.tar.gz")).packages.${system}.plt
  ];
}
```

## Cross-Platform Support

The flake supports multiple platforms:

- `x86_64-linux`
- `aarch64-linux`
- `x86_64-darwin`
- `aarch64-darwin`

## Troubleshooting

### Vendor Hash Mismatch

If you get a vendor hash mismatch error, run:

```bash
nix build .#plt-bin --extra-experimental-features "nix-command flakes"
```

Update the `vendorHash` in `flake.nix` with the correct hash from the error message.

### Git Tree is Dirty

This warning appears when you have uncommitted changes. It's safe to ignore during development.

### jq Not Found

The wrapper script depends on `jq`. In the Nix environment, this is automatically provided, but make sure your flake includes it in the runtime dependencies.

## Contributing

The flake makes development easy:

1. Clone the repository
2. Run `nix develop` to enter the development environment
3. Make your changes
4. Test with `nix build`
5. Submit a pull request

The development environment includes all necessary tools for Go development, testing, and debugging.