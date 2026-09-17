package ui

// SelectionResult represents the result of a user selection
type SelectionResult struct {
	Directory   string            // The actual directory path where command should be executed
	Command     string            // The command to run (raw, without env)
	DisplayName string            // The display name shown in fzf (for reference)
	Action      string            // "" (execute, the default), "edit", or "pane"
	Env         map[string]string // Resolved environment variables to apply when running
}

// CommandInfo holds information about a command for display
type CommandInfo struct {
	Display       string
	Directory     string
	Command       string
	DisplayName   string
	Name          string            // Command name (e.g. "build"); empty for unnamed commands
	Type          string            // Project type (npm, go, etc.)
	Env           map[string]string // Resolved environment variables for this command
	FrecencyScore float64           // Score for sorting
	// Invalid is true when this command's name (or its location's name) falls
	// outside the alias-safe charset, so it can't be referenced as an alias.
	Invalid bool
	// InvalidReason is a short human explanation shown when Invalid (e.g.
	// "contains a space").
	InvalidReason string
	// IsTool marks a row that comes from an enabled tool rather than a location's
	// commands. Tool rows are pinned to the end of the default list.
	IsTool bool
	// Loading marks a placeholder standing in for a location whose project type
	// is still being resolved in the background (a `make`/`gradle` parser has to
	// run a shell command to list its targets). It carries no command, and is
	// neither queueable nor runnable.
	Loading bool
	// matched holds the byte offsets in Display the current query matched, set by
	// fuzzyFilter so the renderer highlights without re-running the matcher.
	matched []int
}
