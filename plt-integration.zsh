#!/usr/bin/env zsh
#
# paleta - a command palette for your monorepo
# Zsh Shell Integration
#
# This file provides keyboard shortcut integration for paleta.
# By default:
#   - Ctrl+P launches plt's interactive command selector (multi-select with Tab)
#
# Inside the selector, when running under tmux or zellij, Ctrl+O runs the
# selection in a new tmux window / zellij tab instead of the current shell.
#
# Customization:
#   export PLT_KEYBIND='^O'           # Use Ctrl+O instead of Ctrl+P to launch plt
#   export PLT_PANE_SHELL='zsh'       # Shell the new tmux/zellij pane lands in
#

# Function to find plt binary
__plt_find_binary() {
    # First check if PLT_BINARY is already set and exists
    if [[ -n "$PLT_BINARY" && -f "$PLT_BINARY" && -x "$PLT_BINARY" ]]; then
        echo "$PLT_BINARY"
        return 0
    fi

    # Try different locations in order of preference
    local candidates=(
        "${0:a:h}/plt"                    # Same directory as this script
        "${0:a:h}/plt-bin"                # Same directory with -bin suffix
        "${commands[plt-bin]}"            # In PATH with -bin suffix
        "${commands[plt]}"               # In PATH (may be wrapper, but works)
        "$HOME/.local/bin/plt-bin"        # User local bin
        "/usr/local/bin/plt-bin"          # System local bin
    )

    for candidate in $candidates; do
        if [[ -n "$candidate" && -f "$candidate" && -x "$candidate" ]]; then
            echo "$candidate"
            return 0
        fi
    done

    return 1
}

# Open a prepared command line in a new tmux window / zellij tab, starting in the
# current directory. After the command finishes the tab/window drops into an
# interactive shell so its output stays visible (override with PLT_PANE_SHELL).
__plt_open_in_mux() {
    local runline="$1"
    local keep="${PLT_PANE_SHELL:-${SHELL:-bash}}"
    local full="$runline"$'\n'"exec $keep"

    if [[ -n "$ZELLIJ" ]]; then
        # zellij's CLI launches commands into a new pane; tmux gets a new window.
        zellij run --cwd "$PWD" --name "plt" -- bash -c "$full"
    elif [[ -n "$TMUX" ]]; then
        tmux new-window -c "$PWD" "$full"
    else
        echo "\nNo tmux or zellij session detected; cannot open a new pane." >&2
        return 1
    fi
}

# Build the command line for a selection (single object or multi-select array)
# and open it in a new tmux window / zellij tab, recording history like a normal
# run.
__plt_run_selection_in_pane() {
    local plt_binary="$1" jq_cmd="$2" selection_json="$3" first_char="$4"
    local runline=""

    if [[ "$first_char" = "[" ]]; then
        local count=$("$jq_cmd" 'length' <<< "$selection_json")
        local i=0
        while [[ "$i" -lt "$count" ]]; do
            local dir=$("$jq_cmd" -r ".[$i].directory" <<< "$selection_json")
            local cmd=$("$jq_cmd" -r ".[$i].command" <<< "$selection_json")
            local name=$("$jq_cmd" -r ".[$i].display_name" <<< "$selection_json")
            local envprefix=$("$jq_cmd" -r ".[$i].env // {} | to_entries | map(\"\(.key)=\" + (.value|@sh)) | join(\" \")" <<< "$selection_json")

            if [[ -n "$name" && "$name" != "null" ]]; then
                "$plt_binary" record "$name" "$cmd" 2>/dev/null || true
            fi

            local segment
            if [[ -n "$envprefix" ]]; then
                segment="cd '$dir' && ( export $envprefix; $cmd )"
            else
                segment="cd '$dir' && $cmd"
            fi

            if [[ -z "$runline" ]]; then
                runline="$segment"
            else
                runline="$runline && $segment"
            fi
            ((i++))
        done
    else
        local dir=$("$jq_cmd" -r '.directory' <<< "$selection_json")
        local cmd=$("$jq_cmd" -r '.command' <<< "$selection_json")
        local name=$("$jq_cmd" -r '.display_name' <<< "$selection_json")
        local envprefix=$("$jq_cmd" -r '.env // {} | to_entries | map("\(.key)=" + (.value|@sh)) | join(" ")' <<< "$selection_json")

        if [[ "$dir" = "null" || "$cmd" = "null" ]]; then
            echo "\nFailed to parse selection from plt output." >&2
            return 1
        fi

        if [[ -n "$name" && "$name" != "null" ]]; then
            "$plt_binary" record "$name" "$cmd" 2>/dev/null || true
        fi

        if [[ -n "$envprefix" ]]; then
            runline="cd '$dir' && ( export $envprefix; $cmd )"
        else
            runline="cd '$dir' && $cmd"
        fi
    fi

    __plt_open_in_mux "$runline"
}

# Zsh widget function for plt selection and execution (supports multi-select)
__plt_select_widget() {
    # Find the plt binary
    local plt_binary
    plt_binary=$(__plt_find_binary)
    if [[ $? -ne 0 ]]; then
        echo "\nplt binary not found. Please ensure plt is installed." >&2
        zle reset-prompt
        return 1
    fi

    # Check for jq dependency
    local jq_cmd="${JQ_CMD:-jq}"
    if ! command -v "$jq_cmd" &> /dev/null; then
        echo "\njq is required but not installed. Please install jq to use plt." >&2
        zle reset-prompt
        return 1
    fi

    # Clear the current line
    zle kill-whole-line

    # Run plt select and capture the result
    local selection_json
    if selection_json=$("$plt_binary" select 2>/dev/null); then
        # Check if result is array (multi-select) or single object
        local first_char="${selection_json:0:1}"

        # The "pane" action opens the selection in a new tmux window / zellij tab
        # rather than running it in the current shell.
        local pane_action
        if [[ "$first_char" = "[" ]]; then
            pane_action=$("$jq_cmd" -r '.[0].action // "execute"' <<< "$selection_json")
        else
            pane_action=$("$jq_cmd" -r '.action // "execute"' <<< "$selection_json")
        fi
        if [[ "$pane_action" = "pane" ]]; then
            __plt_run_selection_in_pane "$plt_binary" "$jq_cmd" "$selection_json" "$first_char"
            zle reset-prompt
            return 0
        fi

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
                    "$plt_binary" record "$name" "$cmd" 2>/dev/null || true
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
                    "$plt_binary" record "$display_name" "$command" 2>/dev/null || true
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
zle -N __plt_select_widget

# Bind the widget to a key (default: Ctrl+P, customizable via PLT_KEYBIND)
bindkey "${PLT_KEYBIND:-^P}" __plt_select_widget
