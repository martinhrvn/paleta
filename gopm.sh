#!/usr/bin/env bash

# gopm - Go Project Manager
# Shell wrapper that sources the core logic

set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source the core script
if [ -f "$SCRIPT_DIR/gopm-core.sh" ]; then
    source "$SCRIPT_DIR/gopm-core.sh"
else
    echo "Error: gopm-core.sh not found in $SCRIPT_DIR" >&2
    exit 1
fi

# Function to find gopm binary
find_gopm_binary() {
    # First check if GOPM_BINARY is already set and exists
    if [ -n "$GOPM_BINARY" ] && [ -f "$GOPM_BINARY" ] && [ -x "$GOPM_BINARY" ]; then
        echo "$GOPM_BINARY"
        return 0
    fi
    
    # Try different locations in order of preference
    local candidates=(
        "$SCRIPT_DIR/gopm"               # Same directory as script
        "$SCRIPT_DIR/gopm-bin"           # Same directory with -bin suffix
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

# Find and export the gopm binary path
GOPM_BINARY=$(find_gopm_binary)
if [ $? -ne 0 ]; then
    print_error "gopm binary not found."
    echo "Please ensure gopm is installed or the binary is in your PATH."
    echo "If you're in development, run: go build -o gopm cmd/gopm/main.go"
    exit 1
fi

export GOPM_BINARY

# Call the main function from core script
gopm_main "$@"