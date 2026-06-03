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
                'list[List all available commands]' \
                'help[Show help message]'
            ;;
    esac
}

_plt "$@"