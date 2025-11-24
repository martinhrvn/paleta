#!/usr/bin/env zsh
#
# gopm - Go Project Manager
# Zsh Shell Integration
#
# This file provides keyboard shortcut integration for gopm.
# By default, it binds Ctrl+P to launch gopm's interactive command selector.
#
# Customization:
#   export GOPM_KEYBIND='^G'           # Use Ctrl+G instead of Ctrl+P
#   export GOPM_MODE='--enhanced'       # Use enhanced TUI mode by default
#

# Function to find gopm binary
__gopm_find_binary() {
    # First check if GOPM_BINARY is already set and exists
    if [[ -n "$GOPM_BINARY" && -f "$GOPM_BINARY" && -x "$GOPM_BINARY" ]]; then
        echo "$GOPM_BINARY"
        return 0
    fi

    # Try different locations in order of preference
    local candidates=(
        "${0:a:h}/gopm"                    # Same directory as this script
        "${0:a:h}/gopm-bin"                # Same directory with -bin suffix
        "${commands[gopm-bin]}"            # In PATH with -bin suffix
        "$HOME/.local/bin/gopm-bin"        # User local bin
        "/usr/local/bin/gopm-bin"          # System local bin
    )

    for candidate in $candidates; do
        if [[ -n "$candidate" && -f "$candidate" && -x "$candidate" ]]; then
            echo "$candidate"
            return 0
        fi
    done

    return 1
}

# Zsh widget function for gopm selection and execution
__gopm_select_widget() {
    # Find the gopm binary
    local gopm_binary=$(__gopm_find_binary)
    if [[ $? -ne 0 ]]; then
        echo "\ngopm binary not found. Please ensure gopm is installed." >&2
        zle reset-prompt
        return 1
    fi

    # Check for jq dependency
    local jq_cmd="${JQ_CMD:-jq}"
    if ! command -v "$jq_cmd" &> /dev/null; then
        echo "\njq is required but not installed. Please install jq to use gopm." >&2
        zle reset-prompt
        return 1
    fi

    # Clear the current line
    zle kill-whole-line

    # Run gopm select and capture the result
    local mode="${GOPM_MODE:-}"
    local selection_json
    if selection_json=$("$gopm_binary" select $mode 2>/dev/null); then
        # Parse JSON to extract directory, command, and display_name
        local directory=$(echo "$selection_json" | "$jq_cmd" -r '.directory')
        local command=$(echo "$selection_json" | "$jq_cmd" -r '.command')
        local display_name=$(echo "$selection_json" | "$jq_cmd" -r '.display_name')

        # Validate parsed values
        if [[ "$directory" != "null" && "$command" != "null" && -d "$directory" ]]; then
            # Record to history (silently fail if it doesn't work)
            if [[ -n "$display_name" && "$display_name" != "null" ]]; then
                "$gopm_binary" record "$display_name" "$command" 2>/dev/null || true
            fi

            # Show what we're about to run
            echo "\n\033[1;33mRunning:\033[0m $command"
            echo "\033[1;33mIn:\033[0m $directory\n"

            # Change to directory and execute command
            # We need to use eval to execute in the current shell context
            # But first, we need to cd to the directory
            cd "$directory"

            # Use BUFFER to put command in the input buffer and accept it
            # This way it shows up in shell history too
            BUFFER="$command"
            zle accept-line
        else
            echo "\nFailed to parse selection or directory does not exist." >&2
            zle reset-prompt
            return 1
        fi
    else
        # Selection was cancelled or failed - just reset the prompt
        zle reset-prompt
        return 0
    fi
}

# Register the widget
zle -N __gopm_select_widget

# Bind the widget to a key (default: Ctrl+P, customizable via GOPM_KEYBIND)
bindkey "${GOPM_KEYBIND:-^P}" __gopm_select_widget
