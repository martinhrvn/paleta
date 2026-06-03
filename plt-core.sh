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
    echo "    version    Show the plt version"
    echo "    help       Show this help message"
    echo
    echo "EXAMPLES:"
    echo "    plt              # Interactive selection and execution"
    echo "    plt run          # Same as above"
    echo "    plt init         # Build .pltrc from detected projects"
    echo "    plt init --template  # Write a static starter .pltrc"
    echo "    plt list         # List all commands"
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

    if [ "$first_char" = "[" ]; then
        # Multi-select: JSON array
        local count
        count=$(echo "$selection_json" | "$jq_cmd" 'length')

        if [ "$count" -eq 0 ]; then
            print_error "No commands selected."
            exit 1
        fi

        # Build compound command: cd dir1 && cmd1 && cd dir2 && cmd2 ...
        local compound_cmd=""
        local i=0
        while [ "$i" -lt "$count" ]; do
            local dir cmd name envprefix segment
            dir=$(echo "$selection_json" | "$jq_cmd" -r ".[$i].directory")
            cmd=$(echo "$selection_json" | "$jq_cmd" -r ".[$i].command")
            name=$(echo "$selection_json" | "$jq_cmd" -r ".[$i].display_name")

            # Record each command (uses the raw command, not the env-wrapped form)
            if [ -n "$name" ] && [ "$name" != "null" ]; then
                "$PLT_BINARY" record "$name" "$cmd" 2>/dev/null || true
            fi

            # Build a safely-quoted "KEY='val' ..." prefix from the env object.
            envprefix=$(echo "$selection_json" | "$jq_cmd" -r ".[$i].env // {} | to_entries | map(\"\(.key)=\" + (.value|@sh)) | join(\" \")")

            # Apply env in a subshell so $VAR in the command expands against it
            # and the variables don't leak into later commands or the shell.
            if [ -n "$envprefix" ]; then
                segment="cd '$dir' && ( export $envprefix; $cmd )"
            else
                segment="cd '$dir' && $cmd"
            fi

            # Build compound command
            if [ -z "$compound_cmd" ]; then
                compound_cmd="$segment"
            else
                compound_cmd="$compound_cmd && $segment"
            fi

            i=$((i + 1))
        done

        # Execute compound command
        print_info "Running $count command(s)..."
        echo
        eval "$compound_cmd"
    else
        # Single selection: JSON object
        local dir cmd name
        dir=$(echo "$selection_json" | "$jq_cmd" -r '.directory')
        cmd=$(echo "$selection_json" | "$jq_cmd" -r '.command')
        name=$(echo "$selection_json" | "$jq_cmd" -r '.display_name')

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

        # Record this command execution in history
        if [ -n "$name" ] && [ "$name" != "null" ]; then
            "$PLT_BINARY" record "$name" "$cmd" 2>/dev/null || true
        fi

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
        help|--help|-h)
            show_usage
            ;;
        version|--version|-v)
            "$PLT_BINARY" version
            ;;
        select|record)
            "$PLT_BINARY" "$@"
            ;;
        *)
            print_error "Unknown command '$1'"
            show_usage
            exit 1
            ;;
    esac
}
