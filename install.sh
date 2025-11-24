#!/usr/bin/env bash

# gopm installer script
# This script downloads and installs gopm for easy use

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_error() { echo -e "${RED}Error:${NC} $1" >&2; }
print_success() { echo -e "${GREEN}$1${NC}"; }
print_info() { echo -e "${YELLOW}$1${NC}"; }
print_step() { echo -e "${BLUE}$1${NC}"; }

# Configuration
REPO_URL="https://github.com/martin/go-pm"  # Replace with actual repo
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
BINARY_NAME="gopm-bin"
WRAPPER_NAME="gopm"

# Function to detect OS and architecture
detect_platform() {
    local os arch
    
    case "$(uname -s)" in
        Linux*)   os="linux" ;;
        Darwin*)  os="darwin" ;;
        CYGWIN*|MINGW*|MSYS*) os="windows" ;;
        *)        print_error "Unsupported OS: $(uname -s)"; exit 1 ;;
    esac
    
    case "$(uname -m)" in
        x86_64|amd64) arch="amd64" ;;
        i686|i386)    arch="386" ;;
        aarch64|arm64) arch="arm64" ;;
        armv7l)       arch="arm" ;;
        *)            print_error "Unsupported architecture: $(uname -m)"; exit 1 ;;
    esac
    
    echo "${os}_${arch}"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check dependencies
check_dependencies() {
    local missing_deps=()
    
    if ! command_exists curl && ! command_exists wget; then
        missing_deps+=("curl or wget")
    fi
    
    if ! command_exists jq; then
        missing_deps+=("jq")
    fi
    
    if [ ${#missing_deps[@]} -gt 0 ]; then
        print_error "Missing dependencies: ${missing_deps[*]}"
        echo
        echo "Please install the missing dependencies:"
        echo "  Ubuntu/Debian: sudo apt-get install curl jq"
        echo "  macOS: brew install curl jq"
        echo "  CentOS/RHEL: sudo yum install curl jq"
        exit 1
    fi
}

# Function to download file
download_file() {
    local url="$1"
    local output="$2"
    
    if command_exists curl; then
        curl -sSL -o "$output" "$url"
    elif command_exists wget; then
        wget -q -O "$output" "$url"
    else
        print_error "Neither curl nor wget is available for downloading"
        exit 1
    fi
}

# Function to build from source (fallback)
build_from_source() {
    print_step "Building from source..."
    
    if ! command_exists go; then
        print_error "Go is required to build from source but is not installed."
        echo "Please install Go from https://golang.org/dl/"
        exit 1
    fi
    
    local temp_dir
    temp_dir=$(mktemp -d)
    
    (
        cd "$temp_dir"
        git clone "$REPO_URL" .
        go build -o "$BINARY_NAME" .
        mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    )
    
    rm -rf "$temp_dir"
}

# Function to install binary
install_binary() {
    print_step "Installing gopm binary..."
    
    # Create install directory if it doesn't exist
    mkdir -p "$INSTALL_DIR"
    
    # For now, we'll build from source since we don't have releases yet
    # In the future, this would download from GitHub releases
    if [ -f "gopm" ]; then
        # If we're in the development directory
        cp "gopm" "$INSTALL_DIR/$BINARY_NAME"
    elif [ -f "$(dirname "$0")/gopm" ]; then
        # If running from the repo directory
        cp "$(dirname "$0")/gopm" "$INSTALL_DIR/$BINARY_NAME"
    else
        build_from_source
    fi
    
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    print_success "Binary installed to $INSTALL_DIR/$BINARY_NAME"
}

# Function to install wrapper script
install_wrapper() {
    print_step "Installing gopm wrapper script..."
    
    # Create the wrapper script
    cat > "$INSTALL_DIR/$WRAPPER_NAME" << 'EOF'
#!/usr/bin/env bash

# gopm - Go Project Manager
# Shell wrapper for gopm binary that handles command selection and execution

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_error() { echo -e "${RED}Error:${NC} $1" >&2; }
print_success() { echo -e "${GREEN}$1${NC}"; }
print_info() { echo -e "${YELLOW}$1${NC}"; }

# Function to find gopm binary
find_gopm_binary() {
    # Try different locations in order of preference
    local candidates=(
        "$(dirname "$0")/gopm-bin"       # Same directory with -bin suffix
        "$(command -v gopm-bin 2>/dev/null)" # In PATH with -bin suffix
        "$HOME/.local/bin/gopm-bin"      # User local bin
        "/usr/local/bin/gopm-bin"        # System local bin
    )

    for candidate in "${candidates[@]}"; do
        if [ -n "$candidate" ] && [ -f "$candidate" ] && [ -x "$candidate" ]; then
            echo "$candidate"
            return 0
        fi
    done

    return 1
}

# Function to show usage
show_usage() {
    echo "gopm - Go Project Manager"
    echo
    echo "USAGE:"
    echo "    gopm [command]"
    echo
    echo "COMMANDS:"
    echo "    run        Interactive command selection and execution (default)"
    echo "    list       List all available commands"
    echo "    help       Show this help message"
    echo
    echo "EXAMPLES:"
    echo "    gopm           # Interactive selection and execution"
    echo "    gopm run       # Same as above"
    echo "    gopm list      # List all commands"
    echo
    echo "CONFIGURATION:"
    echo "    gopm looks for .gopmrc files starting from the current directory"
    echo "    and traversing up the directory tree until it finds one."
}

# Function to check dependencies
check_dependencies() {
    if ! command -v jq &> /dev/null; then
        print_error "jq is required but not installed."
        echo "Please install jq to use gopm:"
        echo "  Ubuntu/Debian: sudo apt-get install jq"
        echo "  macOS: brew install jq"
        echo "  CentOS/RHEL: sudo yum install jq"
        exit 1
    fi
}

# Function to run command interactively
run_command() {
    check_dependencies

    # Find the gopm binary
    GOPM_BINARY=$(find_gopm_binary)
    if [ $? -ne 0 ]; then
        print_error "gopm binary not found."
        echo "Please ensure gopm is installed or the binary is in your PATH."
        exit 1
    fi

    # Call gopm select to get the selection as JSON
    print_info "Loading configuration and starting selection..."
    if ! SELECTION_JSON=$("$GOPM_BINARY" select 2>/dev/null); then
        print_error "Selection cancelled or failed."
        exit 1
    fi

    # Check if we got valid JSON
    if [ -z "$SELECTION_JSON" ]; then
        print_error "No selection made."
        exit 1
    fi

    # Parse JSON to extract directory and command
    DIRECTORY=$(echo "$SELECTION_JSON" | jq -r '.directory')
    COMMAND=$(echo "$SELECTION_JSON" | jq -r '.command')

    # Validate parsed values
    if [ "$DIRECTORY" = "null" ] || [ "$COMMAND" = "null" ]; then
        print_error "Failed to parse selection from gopm output."
        exit 1
    fi

    # Check if directory exists
    if [ ! -d "$DIRECTORY" ]; then
        print_error "Directory '$DIRECTORY' does not exist."
        exit 1
    fi

    # Show what we're about to do
    print_info "Running: $COMMAND"
    print_info "In: $DIRECTORY"
    echo

    # Change to the directory and run the command
    cd "$DIRECTORY"
    exec bash -c "$COMMAND"
}

# Function to list commands
list_commands() {
    GOPM_BINARY=$(find_gopm_binary)
    if [ $? -ne 0 ]; then
        print_error "gopm binary not found."
        echo "Please ensure gopm is installed or the binary is in your PATH."
        exit 1
    fi

    "$GOPM_BINARY" list
}

# Main script logic
case "${1:-run}" in
    run|"")
        run_command
        ;;
    list)
        list_commands
        ;;
    help|--help|-h)
        show_usage
        ;;
    *)
        print_error "Unknown command '$1'"
        show_usage
        exit 1
        ;;
esac
EOF

    chmod +x "$INSTALL_DIR/$WRAPPER_NAME"
    print_success "Wrapper installed to $INSTALL_DIR/$WRAPPER_NAME"
}

# Function to setup PATH
setup_path() {
    print_step "Setting up PATH..."
    
    # Check if install directory is in PATH
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        # Add to shell profile
        local shell_profile=""
        if [ -n "$ZSH_VERSION" ]; then
            shell_profile="$HOME/.zshrc"
        elif [ -n "$BASH_VERSION" ]; then
            shell_profile="$HOME/.bashrc"
        fi
        
        if [ -n "$shell_profile" ]; then
            echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$shell_profile"
            print_info "Added $INSTALL_DIR to PATH in $shell_profile"
            print_info "Please restart your shell or run: source $shell_profile"
        else
            print_info "Please add $INSTALL_DIR to your PATH manually"
        fi
    else
        print_success "Install directory is already in PATH"
    fi
}

# Function to create shell completion
create_completion() {
    print_step "Creating shell completion..."
    
    local completion_dir
    if [ -n "$ZSH_VERSION" ]; then
        completion_dir="$HOME/.zsh/completions"
        mkdir -p "$completion_dir"
        
        cat > "$completion_dir/_gopm" << 'EOF'
#compdef gopm

_gopm() {
    local context state line
    
    _arguments \
        '1: :->command' \
        '*: :->args'
    
    case $state in
        command)
            _values 'gopm commands' \
                'run[Interactive command selection and execution]' \
                'list[List all available commands]' \
                'help[Show help message]'
            ;;
    esac
}

_gopm "$@"
EOF
        print_success "Zsh completion installed to $completion_dir/_gopm"
    elif [ -n "$BASH_VERSION" ]; then
        completion_dir="$HOME/.bash_completion.d"
        mkdir -p "$completion_dir"
        
        cat > "$completion_dir/gopm" << 'EOF'
_gopm_completion() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    
    opts="run list help"
    
    if [[ ${cur} == -* ]]; then
        COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
        return 0
    fi
    
    case "${prev}" in
        gopm)
            COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
            return 0
            ;;
    esac
}

complete -F _gopm_completion gopm
EOF
        print_success "Bash completion installed to $completion_dir/gopm"
    fi
}

# Function to setup zsh integration (keyboard shortcuts)
setup_zsh_integration() {
    print_step "Setting up zsh shell integration..."

    # Only setup for zsh
    if [ -n "$ZSH_VERSION" ] || grep -q "zsh" "$SHELL" 2>/dev/null; then
        local integration_dir="$HOME/.local/share/gopm"
        local integration_file="$integration_dir/gopm-integration.zsh"
        local zshrc="$HOME/.zshrc"

        # Create directory if it doesn't exist
        mkdir -p "$integration_dir"

        # Copy or install integration file
        if [ -f "gopm-integration.zsh" ]; then
            # If we're in the development directory
            cp "gopm-integration.zsh" "$integration_file"
        elif [ -f "$(dirname "$0")/gopm-integration.zsh" ]; then
            # If running from the repo directory
            cp "$(dirname "$0")/gopm-integration.zsh" "$integration_file"
        else
            print_info "gopm-integration.zsh not found, skipping zsh integration"
            return 0
        fi

        print_success "Integration file installed to $integration_file"

        # Add source line to .zshrc if not already present
        if [ -f "$zshrc" ]; then
            if ! grep -q "gopm-integration.zsh" "$zshrc" 2>/dev/null; then
                echo "" >> "$zshrc"
                echo "# gopm keyboard shortcut (Ctrl+P)" >> "$zshrc"
                echo "[ -f \"$integration_file\" ] && source \"$integration_file\"" >> "$zshrc"
                print_success "Added integration to $zshrc"
                print_info "Restart your shell or run: source $zshrc"
                print_info "Press Ctrl+P to launch gopm from anywhere!"
            else
                print_success "Integration already configured in $zshrc"
            fi
        else
            print_info "No .zshrc found. Add this to your zsh config:"
            print_info "  [ -f \"$integration_file\" ] && source \"$integration_file\""
        fi
    else
        print_info "Zsh not detected, skipping zsh integration"
    fi
}

# Main installation function
main() {
    print_step "Starting gopm installation..."

    # Check dependencies
    check_dependencies

    # Install components
    install_binary
    install_wrapper
    setup_path
    create_completion
    setup_zsh_integration

    print_success "gopm installation completed!"
    echo
    echo "Usage:"
    echo "  gopm           # Interactive command selection"
    echo "  gopm list      # List available commands"
    echo "  gopm help      # Show help"
    echo
    echo "Configuration:"
    echo "  Create a .gopmrc file in your project root"
    echo "  See the documentation for configuration examples"
}

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    print_error "Don't run this installer as root"
    exit 1
fi

# Run main installation
main "$@"