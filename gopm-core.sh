#!/usr/bin/env bash

# gopm - Go Project Manager
# Core shell logic for gopm wrapper

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
    # Check if jq command exists (either direct or via JQ_CMD variable)
    local jq_cmd="${JQ_CMD:-jq}"
    if ! command -v "$jq_cmd" &> /dev/null; then
        print_error "jq is required but not installed."
        if [ -z "$JQ_CMD" ]; then
            echo "Please install jq to use gopm:"
            echo "  Ubuntu/Debian: sudo apt-get install jq"
            echo "  macOS: brew install jq"
            echo "  CentOS/RHEL: sudo yum install jq"
        fi
        exit 1
    fi
}

# Function to run command interactively
run_command() {
    check_dependencies

    # GOPM_BINARY must be set by the wrapper
    if [ -z "$GOPM_BINARY" ]; then
        print_error "GOPM_BINARY environment variable not set."
        exit 1
    fi

    # Check if binary exists
    if [ ! -f "$GOPM_BINARY" ] || [ ! -x "$GOPM_BINARY" ]; then
        print_error "gopm binary not found at: $GOPM_BINARY"
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

    # Parse JSON to extract directory, command, and display_name
    local jq_cmd="${JQ_CMD:-jq}"
    DIRECTORY=$(echo "$SELECTION_JSON" | "$jq_cmd" -r '.directory')
    COMMAND=$(echo "$SELECTION_JSON" | "$jq_cmd" -r '.command')
    DISPLAY_NAME=$(echo "$SELECTION_JSON" | "$jq_cmd" -r '.display_name')

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

    # Record this command execution in history (silently fail if it doesn't work)
    if [ -n "$DISPLAY_NAME" ] && [ "$DISPLAY_NAME" != "null" ]; then
        "$GOPM_BINARY" record "$DISPLAY_NAME" "$COMMAND" 2>/dev/null || true
    fi

    # Change to the directory and run the command
    cd "$DIRECTORY"

    # Use BASH_CMD if set (for Nix), otherwise use bash
    local bash_cmd="${BASH_CMD:-bash}"
    exec "$bash_cmd" -c "$COMMAND"
}

# Function to list commands
list_commands() {
    # GOPM_BINARY must be set by the wrapper
    if [ -z "$GOPM_BINARY" ]; then
        print_error "GOPM_BINARY environment variable not set."
        exit 1
    fi

    if [ ! -f "$GOPM_BINARY" ] || [ ! -x "$GOPM_BINARY" ]; then
        print_error "gopm binary not found at: $GOPM_BINARY"
        exit 1
    fi

    "$GOPM_BINARY" list
}

# Main function - to be called by wrappers
gopm_main() {
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
}