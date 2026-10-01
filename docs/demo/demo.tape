# VHS tape for the README demo GIF.
# Render with: ./docs/demo/render.sh   (see https://github.com/charmbracelet/vhs)

Output docs/demo/demo.gif

Require bash

Set Shell "bash"
Set FontSize 18
Set Width 1200
Set Height 640
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 90ms
Set CursorBlink false

# Setup: throwaway $HOME and a copy of the demo repo so no personal history,
# config or paths leak into the recording; tmux/zellij unset to hide the ^O hint.
Hide
Type `unset TMUX ZELLIJ; rm -rf /tmp/plt-demo && mkdir -p /tmp/plt-demo && cp -r docs/demo/repo /tmp/plt-demo/acme && export HOME=/tmp/plt-demo PLT_NO_ICONS=1 PLT_BINARY=$PWD/bin/plt-bin PS1='\[\e[1;35m\]❯\[\e[0m\] '` Enter
Type `alias plt=$PWD/packaging/plt && cd /tmp/plt-demo/acme && clear` Enter
Sleep 500ms
Show

# Every command across the monorepo, detected from package.json / go.mod / .pltrc
Type "plt" Sleep 400ms Enter
Sleep 2s
Down@250ms 4
Sleep 1s

# Fuzzy filter
Type "api test"
Sleep 1.5s
Backspace@40ms 8
Sleep 300ms
Type "web dev"
Sleep 1.5s

# Run a custom script command from .pltrc
Backspace@40ms 8
Sleep 400ms
Type "deploy:sta"
Sleep 1.2s
Enter
Sleep 3s
