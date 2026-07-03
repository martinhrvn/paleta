package ui

// SelectionResult represents the result of a user selection
type SelectionResult struct {
	Directory   string            // The actual directory path where command should be executed
	Command     string            // The command to run (raw, without env)
	DisplayName string            // The display name shown in fzf (for reference)
	Action      string            // "execute" (default, empty) or "edit"
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
}
