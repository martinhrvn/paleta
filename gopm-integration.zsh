#!/usr/bin/env zsh
#
# gopm - Go Project Manager
# Zsh Shell Integration
#
# This file provides keyboard shortcut integration for gopm.
# By default:
#   - Ctrl+P launches gopm's interactive command selector (multi-select with Tab)
#
# Customization:
#   export GOPM_KEYBIND='^O'           # Use Ctrl+O instead of Ctrl+P
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

# Zsh widget function for gopm selection and execution (supports multi-select)
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
    local selection_json
    if selection_json=$("$gopm_binary" select 2>/dev/null); then
        # Check if result is array (multi-select) or single object
        local first_char="${selection_json:0:1}"

        if [[ "$first_char" = "[" ]]; then
            # Multi-select: JSON array - build compound command
            local count=$("$jq_cmd" 'length' <<< "$selection_json")

            if [[ "$count" -eq 0 ]]; then
                zle reset-prompt
                return 0
            fi

            # Check action from first element
            local action=$("$jq_cmd" -r '.[0].action // "execute"' <<< "$selection_json")

            local compound_cmd=""
            local i=0
            while [[ "$i" -lt "$count" ]]; do
                local dir=$("$jq_cmd" -r ".[$i].directory" <<< "$selection_json")
                local cmd=$("$jq_cmd" -r ".[$i].command" <<< "$selection_json")
                local name=$("$jq_cmd" -r ".[$i].display_name" <<< "$selection_json")

                # Record each command (only for execute mode)
                if [[ "$action" != "edit" && -n "$name" && "$name" != "null" ]]; then
                    "$gopm_binary" record "$name" "$cmd" 2>/dev/null || true
                fi

                # Build compound command
                if [[ -z "$compound_cmd" ]]; then
                    compound_cmd="cd '$dir' && $cmd"
                else
                    compound_cmd="$compound_cmd && cd '$dir' && $cmd"
                fi

                ((i++))
            done

            # Put compound command in buffer
            BUFFER="$compound_cmd"
            if [[ "$action" = "edit" ]]; then
                zle reset-prompt
            else
                zle accept-line
            fi
        else
            # Single selection: JSON object
            local directory=$("$jq_cmd" -r '.directory' <<< "$selection_json")
            local command=$("$jq_cmd" -r '.command' <<< "$selection_json")
            local display_name=$("$jq_cmd" -r '.display_name' <<< "$selection_json")
            local action=$("$jq_cmd" -r '.action // "execute"' <<< "$selection_json")

            if [[ "$directory" != "null" && "$command" != "null" && -d "$directory" ]]; then
                # Record to history (only for execute mode)
                if [[ "$action" != "edit" && -n "$display_name" && "$display_name" != "null" ]]; then
                    "$gopm_binary" record "$display_name" "$command" 2>/dev/null || true
                fi

                # Change to directory
                cd "$directory"

                BUFFER="$command"
                if [[ "$action" = "edit" ]]; then
                    zle reset-prompt
                else
                    echo "\n\033[1;33mRunning:\033[0m $command"
                    echo "\033[1;33mIn:\033[0m $directory\n"
                    zle accept-line
                fi
            else
                echo "\nFailed to parse selection or directory does not exist." >&2
                zle reset-prompt
                return 1
            fi
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
