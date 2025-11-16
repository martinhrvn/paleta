package commands

import (
	"fmt"
	"os"
)

// DefaultConfigTemplate returns the default .gopmrc template
func DefaultConfigTemplate() string {
	return `# gopm configuration file
# This file defines locations and commands for your project

locations:
  # Example: NPM/Yarn/PNPM project with automatic script detection
  - name: "frontend"
    location: "packages/frontend"
    type: "npm"  # Automatically discovers package.json scripts
    commands:
      # Additional custom commands beyond package.json scripts
      - "npm run dev"

  # Example: Backend service with manual commands
  - name: "backend"
    location: "packages/backend"
    commands:
      - "npm start"
      - "npm test"
      - "npm run build"

  # Example: Using glob patterns to match multiple directories
  # - location: "packages/*"
  #   type: "npm"

  # Example: Scripts directory with shell scripts
  # - name: "scripts"
  #   location: "scripts"
  #   commands:
  #     - "./deploy.sh"
  #     - "./backup.sh"

# Configuration reference:
#
# location fields:
#   name:     (optional) Display name shown in selection UI
#   location: (required) Path to project directory (supports glob patterns like "packages/*")
#   type:     (optional) Project type for automatic command detection (npm, yarn, pnpm, go)
#   commands: (optional) List of commands to make available
#
# Supported project types:
#   npm, yarn, pnpm: Automatically discovers scripts from package.json
#   go: Discovers standard Go commands (planned)
#
# For more information, see: https://github.com/martin/go-pm
`
}

// CreateDefaultConfig creates a default .gopmrc file at the specified path
// Returns an error if the file already exists
func CreateDefaultConfig(configPath string) error {
	return CreateDefaultConfigWithForce(configPath, false)
}

// CreateDefaultConfigWithForce creates a default .gopmrc file at the specified path
// If force is true, overwrites existing file
func CreateDefaultConfigWithForce(configPath string, force bool) error {
	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		if !force {
			return fmt.Errorf("config file already exists at %s (use --force to overwrite)", configPath)
		}
	}

	// Create the file with default template
	content := DefaultConfigTemplate()
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
