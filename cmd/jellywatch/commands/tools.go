package commands

import (
	"github.com/spf13/cobra"
)

// NewToolsCmd creates the tools command group
// Note: Subcommands are created in main.go and added here
func NewToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Advanced tools and integrations",
		Long:  `Advanced tools for Sonarr, Radarr, AI, and database management.`,
	}

	// Subcommands will be added in main.go from existing commands
	return cmd
}
