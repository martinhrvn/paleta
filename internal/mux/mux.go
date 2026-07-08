// Package mux detects the terminal multiplexer (tmux or zellij) that plt is
// running inside, so the selector can offer a "run in a new tab/window"
// shortcut only when there is a multiplexer to open one in.
package mux

import "os"

// Multiplexer identifies the terminal multiplexer hosting the current session.
type Multiplexer string

const (
	// None means plt is not running inside a supported multiplexer.
	None Multiplexer = ""
	// Tmux means plt is running inside a tmux session.
	Tmux Multiplexer = "tmux"
	// Zellij means plt is running inside a zellij session.
	Zellij Multiplexer = "zellij"
)

// Detect reports which multiplexer plt is running inside, using the given
// environment lookup. tmux exports $TMUX and zellij exports $ZELLIJ into every
// child process, so their presence is a reliable signal.
//
// Zellij is checked first: it is possible (if unusual) to nest a zellij session
// inside tmux, and the inner-most context is the one whose pane commands would
// actually land where the user is looking.
func Detect(getenv func(string) string) Multiplexer {
	if getenv("ZELLIJ") != "" {
		return Zellij
	}
	if getenv("TMUX") != "" {
		return Tmux
	}
	return None
}

// DetectEnv detects the multiplexer from the current process environment.
func DetectEnv() Multiplexer { return Detect(os.Getenv) }

// Active reports whether a supported multiplexer was detected.
func (m Multiplexer) Active() bool { return m != None }

// Label is the human-friendly name shown in help text (e.g. "tmux"), or "" when
// no multiplexer is active.
func (m Multiplexer) Label() string { return string(m) }
