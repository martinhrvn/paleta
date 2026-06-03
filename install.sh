#!/usr/bin/env bash

# paleta installer — builds from source and installs the binary, the plt
# wrapper, plt-core.sh, shell completions, and the zsh Ctrl+P integration.
#
# For pre-built packages instead, see the README (AUR / Homebrew / deb / rpm /
# release tarballs). This script is the "git clone && ./install.sh" path.

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_error()   { echo -e "${RED}Error:${NC} $1" >&2; }
print_success() { echo -e "${GREEN}$1${NC}"; }
print_info()    { echo -e "${YELLOW}$1${NC}"; }
print_step()    { echo -e "${BLUE}$1${NC}"; }

# Repo root = directory containing this script.
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

command_exists() { command -v "$1" >/dev/null 2>&1; }

check_dependencies() {
    local missing=()
    command_exists go || missing+=("go (to build from source — https://go.dev/dl/)")
    command_exists jq || missing+=("jq")

    if [ ${#missing[@]} -gt 0 ]; then
        print_error "Missing dependencies: ${missing[*]}"
        echo "  Ubuntu/Debian: sudo apt-get install golang-go jq"
        echo "  macOS:         brew install go jq"
        echo "  Arch:          sudo pacman -S go jq"
        exit 1
    fi
}

build_binary() {
    print_step "Building plt-bin..."
    ( cd "$REPO_DIR" && go build -ldflags "-s -w" -o "$INSTALL_DIR/plt-bin" ./cmd/plt )
    chmod +x "$INSTALL_DIR/plt-bin"
    print_success "Installed binary: $INSTALL_DIR/plt-bin"
}

install_wrapper() {
    print_step "Installing plt wrapper and core script..."
    install -m 0755 "$REPO_DIR/packaging/plt" "$INSTALL_DIR/plt"
    # The wrapper sources plt-core.sh from beside itself first, so co-locate it.
    install -m 0644 "$REPO_DIR/plt-core.sh" "$INSTALL_DIR/plt-core.sh"
    print_success "Installed wrapper: $INSTALL_DIR/plt"
}

install_completions() {
    print_step "Installing shell completions..."

    local bash_dir="$HOME/.local/share/bash-completion/completions"
    mkdir -p "$bash_dir"
    install -m 0644 "$REPO_DIR/completion.bash" "$bash_dir/plt"
    print_success "Bash completion: $bash_dir/plt"

    local zsh_dir="$HOME/.zsh/completions"
    mkdir -p "$zsh_dir"
    install -m 0644 "$REPO_DIR/completion.zsh" "$zsh_dir/_plt"
    print_success "Zsh completion: $zsh_dir/_plt"
    print_info "Ensure '$zsh_dir' is on your \$fpath (in ~/.zshrc, before compinit)."
}

setup_zsh_integration() {
    print_step "Setting up zsh Ctrl+P integration..."
    local dir="$HOME/.local/share/paleta"
    local file="$dir/plt-integration.zsh"
    local zshrc="$HOME/.zshrc"

    mkdir -p "$dir"
    install -m 0644 "$REPO_DIR/plt-integration.zsh" "$file"
    print_success "Integration installed: $file"

    if [ -f "$zshrc" ] && ! grep -q "plt-integration.zsh" "$zshrc" 2>/dev/null; then
        {
            echo ""
            echo "# paleta keyboard shortcut (Ctrl+P)"
            echo "[ -f \"$file\" ] && source \"$file\""
        } >> "$zshrc"
        print_success "Added integration source line to $zshrc"
        print_info "Restart your shell or run: source $zshrc"
    fi
}

setup_path() {
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        print_info "Add $INSTALL_DIR to your PATH:"
        print_info "  export PATH=\"$INSTALL_DIR:\$PATH\""
    fi
}

main() {
    print_step "Installing paleta from $REPO_DIR..."
    mkdir -p "$INSTALL_DIR"
    check_dependencies
    build_binary
    install_wrapper
    install_completions
    setup_zsh_integration
    setup_path

    echo
    print_success "paleta installed!"
    echo "  plt           # Interactive selection"
    echo "  plt list      # List commands"
    echo "  plt version   # Show version"
    echo "  plt help      # Help"
}

if [ "${EUID:-$(id -u)}" -eq 0 ]; then
    print_error "Don't run this installer as root."
    exit 1
fi

main "$@"
