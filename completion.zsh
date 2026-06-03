#compdef plt

# Zsh completion for plt
# Copy to your zsh completion directory or source this file

_plt() {
    local context state line
    
    _arguments \
        '1: :->command' \
        '*: :->args'
    
    case $state in
        command)
            _values 'plt commands' \
                'run[Interactive command selection and execution]' \
                'init[Interactively scan for projects and build .pltrc]' \
                'edit[Open nearest .pltrc in $EDITOR]' \
                'list[List all available commands]' \
                'version[Show the plt version]' \
                'help[Show help message]'
            ;;
    esac
}

_plt "$@"