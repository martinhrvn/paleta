#!/usr/bin/env bash

# paleta - a command palette for your monorepo
# Core shell logic for plt wrapper

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
    echo "paleta - a command palette for your monorepo"
    echo
    echo "USAGE:"
    echo "    plt [command]"
    echo
    echo "COMMANDS:"
    echo "    run        Interactive command selection and execution (default)"
    echo "    init       Interactively scan for projects and build .pltrc"
    echo "    edit       Open nearest .pltrc in \$EDITOR"
    echo "    list       List all available commands"
    echo "    stats      Show command usage history (runs, recency, frecency)"
    echo "    lint       Check .pltrc names (add --fix to auto-repair)"
    echo "    version    Show the plt version"
    echo "    help       Show this help message"
    echo
    echo "EXAMPLES:"
    echo "    plt              # Interactive selection and execution"
    echo "    plt run          # Same as above"
    echo "    plt init         # Build .pltrc from detected projects"
    echo "    plt init --template  # Write a static starter .pltrc"
    echo "    plt list         # List all commands"
    echo "    plt stats        # Usage history table (--by=count|recent, --limit=N)"
    echo
    echo "CONFIGURATION:"
    echo "    plt looks for .pltrc files starting from the current directory"
    echo "    and traversing up the directory tree until it finds one."
}

# Function to check dependencies
check_dependencies() {
    # Check if jq command exists (either direct or via JQ_CMD variable)
    local jq_cmd="${JQ_CMD:-jq}"
    if ! command -v "$jq_cmd" &> /dev/null; then
        print_error "jq is required but not installed."
        if [ -z "$JQ_CMD" ]; then
            echo "Please install jq to use plt:"
            echo "  Ubuntu/Debian: sudo apt-get install jq"
            echo "  macOS: brew install jq"
            echo "  CentOS/RHEL: sudo yum install jq"
        fi
        exit 1
    fi
}

# Open a prepared command line in a new tmux window / zellij tab, starting in
# start_dir. After the command finishes the new tab/window drops into an
# interactive shell so its output stays visible (override with PLT_PANE_SHELL).
plt_open_in_mux() {
    local start_dir="$1" runline="$2"
    local keep="${PLT_PANE_SHELL:-${SHELL:-bash}}"
    local full="$runline"$'\n'"exec $keep"

    if [ -n "$ZELLIJ" ]; then
        print_info "Opening in a new zellij pane..."
        # zellij's CLI launches commands into a new pane (there is no
        # new-tab-with-command primitive); tmux below gets a real new window.
        zellij run --cwd "$start_dir" --name "plt" -- bash -c "$full"
    elif [ -n "$TMUX" ]; then
        print_info "Opening in a new tmux window..."
        tmux new-window -c "$start_dir" "$full"
    else
        print_error "No tmux or zellij session detected; cannot open a new pane."
        return 1
    fi
}

# One "cd dir && cmd" segment for the selection object at jq path $2 ("" for a
# single selection, ".[i]" for one element of a multi-select). The env is applied
# in a subshell so $VAR in the command expands against it and nothing leaks into
# later commands or the shell. Fails when the object can't be parsed.
plt_segment() {
    local selection_json="$1" path="$2"
    local jq_cmd="${JQ_CMD:-jq}"
    local dir cmd envprefix
    dir=$(echo "$selection_json" | "$jq_cmd" -r "$path.directory")
    cmd=$(echo "$selection_json" | "$jq_cmd" -r "$path.command")
    if [ "$dir" = "null" ] || [ "$cmd" = "null" ]; then
        return 1
    fi
    envprefix=$(echo "$selection_json" | "$jq_cmd" -r "$path.env // {} | to_entries | map(\"\(.key)=\" + (.value|@sh)) | join(\" \")")
    if [ -n "$envprefix" ]; then
        echo "cd '$dir' && ( export $envprefix; $cmd )"
    else
        echo "cd '$dir' && $cmd"
    fi
}

# Record the selection object at jq path $2 ("" or ".[i]") in history — unless the user edited
# the command (Ctrl+E): a one-off edit shouldn't count toward the original's
# frecency. Uses the raw command, not the env-wrapped form, so the key is stable.
plt_record() {
    local selection_json="$1" path="$2"
    local jq_cmd="${JQ_CMD:-jq}"
    local action name cmd
    action=$(echo "$selection_json" | "$jq_cmd" -r "$path.action // \"execute\"")
    if [ "$action" = "edit" ]; then
        return 0
    fi
    name=$(echo "$selection_json" | "$jq_cmd" -r "$path.display_name")
    cmd=$(echo "$selection_json" | "$jq_cmd" -r "$path.command")
    if [ -n "$name" ] && [ "$name" != "null" ]; then
        "$PLT_BINARY" record "$name" "$cmd" 2>/dev/null || true
    fi
}

# Turn a selection (single object or multi-select array) into one command line
# that runs each command in its own directory, in order, recording each in
# history. Prints the line; fails when the selection can't be parsed.
plt_selection_runline() {
    local selection_json="$1" first_char="$2"
    local jq_cmd="${JQ_CMD:-jq}"
    local runline="" segment
    if [ "$first_char" = "[" ]; then
        local count i=0
        count=$(echo "$selection_json" | "$jq_cmd" 'length')
        while [ "$i" -lt "$count" ]; do
            segment=$(plt_segment "$selection_json" ".[$i]") || return 1
            plt_record "$selection_json" ".[$i]"
            if [ -z "$runline" ]; then
                runline="$segment"
            else
                runline="$runline && $segment"
            fi
            i=$((i + 1))
        done
    else
        runline=$(plt_segment "$selection_json" "") || return 1
        plt_record "$selection_json" ""
    fi
    echo "$runline"
}

# Build the command line for a selection and open it in a new tmux window /
# zellij tab. Records history just like a normal run so pane launches still
# count toward frecency.
plt_run_selection_in_pane() {
    local selection_json="$1" first_char="$2"
    local runline
    if ! runline=$(plt_selection_runline "$selection_json" "$first_char"); then
        print_error "Failed to parse selection from plt output."
        return 1
    fi
    plt_open_in_mux "$PWD" "$runline"
}

# Function to run command interactively (supports multi-select)
run_command() {
    check_dependencies

    # PLT_BINARY must be set by the wrapper
    if [ -z "$PLT_BINARY" ]; then
        print_error "PLT_BINARY environment variable not set."
        exit 1
    fi

    # Check if binary exists
    if [ ! -f "$PLT_BINARY" ] || [ ! -x "$PLT_BINARY" ]; then
        print_error "plt binary not found at: $PLT_BINARY"
        exit 1
    fi

    # Call plt select to get the selection as JSON. Capture stderr so the
    # binary's own messages (e.g. the "run plt init" hint when no .pltrc exists)
    # reach the user instead of being swallowed.
    local selection_json select_err
    select_err=$(mktemp)
    if ! selection_json=$("$PLT_BINARY" select 2>"$select_err"); then
        if [ -s "$select_err" ]; then
            cat "$select_err" >&2
        else
            print_error "Selection cancelled or failed."
        fi
        rm -f "$select_err"
        exit 1
    fi
    rm -f "$select_err"

    # Check if we got valid JSON
    if [ -z "$selection_json" ]; then
        print_error "No selection made."
        exit 1
    fi

    local jq_cmd="${JQ_CMD:-jq}"

    # Check if result is array (multi-select) or single object
    local first_char="${selection_json:0:1}"

    # Determine the action (from the object, or the first array element). The
    # "pane" action runs the selection in a new tmux window / zellij tab instead
    # of the current shell.
    local action
    if [ "$first_char" = "[" ]; then
        action=$(echo "$selection_json" | "$jq_cmd" -r '.[0].action // "execute"')
    else
        action=$(echo "$selection_json" | "$jq_cmd" -r '.action // "execute"')
    fi

    if [ "$action" = "pane" ]; then
        plt_run_selection_in_pane "$selection_json" "$first_char"
        return $?
    fi

    if [ "$first_char" = "[" ]; then
        # Multi-select: JSON array, run as one compound command in queue order.
        local count
        count=$(echo "$selection_json" | "$jq_cmd" 'length')

        if [ "$count" -eq 0 ]; then
            print_error "No commands selected."
            exit 1
        fi

        local compound_cmd
        if ! compound_cmd=$(plt_selection_runline "$selection_json" "$first_char"); then
            print_error "Failed to parse selection from plt output."
            exit 1
        fi

        print_info "Running $count command(s)..."
        echo
        eval "$compound_cmd"
    else
        # Single selection: JSON object
        local dir cmd
        dir=$(echo "$selection_json" | "$jq_cmd" -r '.directory')
        cmd=$(echo "$selection_json" | "$jq_cmd" -r '.command')

        # Validate parsed values
        if [ "$dir" = "null" ] || [ "$cmd" = "null" ]; then
            print_error "Failed to parse selection from plt output."
            exit 1
        fi

        # Check if directory exists
        if [ ! -d "$dir" ]; then
            print_error "Directory '$dir' does not exist."
            exit 1
        fi

        # Show what we're about to do
        print_info "Running: $cmd"
        print_info "In: $dir"
        echo

        # Record this command execution in history (skipped for an edited command)
        plt_record "$selection_json" ""

        # Change to the directory and run the command
        cd "$dir"

        # Build a safely-quoted "KEY='val' ..." prefix from the env object.
        local envprefix
        envprefix=$(echo "$selection_json" | "$jq_cmd" -r '.env // {} | to_entries | map("\(.key)=" + (.value|@sh)) | join(" ")')

        # Use BASH_CMD if set (for Nix), otherwise use bash
        local bash_cmd="${BASH_CMD:-bash}"

        # Export env first (separate statement) so $VAR in the command expands
        # against it; the child bash isolates it from the parent shell.
        if [ -n "$envprefix" ]; then
            exec "$bash_cmd" -c "export $envprefix; $cmd"
        else
            exec "$bash_cmd" -c "$cmd"
        fi
    fi
}

# Function to list commands
list_commands() {
    # PLT_BINARY must be set by the wrapper
    if [ -z "$PLT_BINARY" ]; then
        print_error "PLT_BINARY environment variable not set."
        exit 1
    fi

    if [ ! -f "$PLT_BINARY" ] || [ ! -x "$PLT_BINARY" ]; then
        print_error "plt binary not found at: $PLT_BINARY"
        exit 1
    fi

    "$PLT_BINARY" list
}

# Main function - to be called by wrappers
plt_main() {
    case "${1:-run}" in
        run|tui|"")
            run_command
            ;;
        init)
            # Interactive wizard (or --template). Runs attached to the terminal
            # and writes .pltrc itself, so pass through args and all fds.
            "$PLT_BINARY" "$@"
            ;;
        edit)
            "$PLT_BINARY" edit
            ;;
        list)
            list_commands
            ;;
        stats)
            # Usage history table. Pass through flags like --by= and --limit=.
            "$PLT_BINARY" "$@"
            ;;
        help|--help|-h)
            show_usage
            ;;
        version|--version|-v)
            "$PLT_BINARY" version
            ;;
        select|record)
            "$PLT_BINARY" "$@"
            ;;
        lint)
            # Check .pltrc names (optionally --fix). Plain passthrough; pass flags.
            "$PLT_BINARY" "$@"
            ;;
        *)
            print_error "Unknown command '$1'"
            show_usage
            exit 1
            ;;
    esac
}
