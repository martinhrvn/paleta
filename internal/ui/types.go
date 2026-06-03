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
	Type          string            // Project type (npm, go, etc.)
	Env           map[string]string // Resolved environment variables for this command
	FrecencyScore float64           // Score for sorting
}
