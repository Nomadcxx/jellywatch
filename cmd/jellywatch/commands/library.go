package commands

import (
	"github.com/spf13/cobra"
)

func NewLibraryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "library",
		Short: "Library management commands",
		Long:  `Commands for scanning, validating, and viewing library statistics.`,
	}

	cmd.AddCommand(newLibraryScanCmd())
	cmd.AddCommand(newLibraryStatusCmd())
	cmd.AddCommand(newLibraryValidateCmd())

	return cmd
}

func newLibraryScanCmd() *cobra.Command {
	var (
		syncSonarr     bool
		syncRadarr     bool
		syncFilesystem bool
		showStats      bool
	)

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan and index libraries into database",
		Long: `Scan all configured libraries and populate the database.

This command scans your TV and movie libraries to build the HOLDEN database,
which enables instant lookups and conflict detection.

By default, scans the filesystem. Use flags to also sync from Sonarr/Radarr.

Examples:
  jellywatch library scan                    # Scan filesystem only
  jellywatch library scan --sonarr --radarr  # Also sync from Sonarr and Radarr
  jellywatch library scan --stats            # Show database stats after scan`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunScan(syncSonarr, syncRadarr, syncFilesystem, showStats)
		},
	}

	cmd.Flags().BoolVar(&syncSonarr, "sonarr", false, "Also sync from Sonarr")
	cmd.Flags().BoolVar(&syncRadarr, "radarr", false, "Also sync from Radarr")
	cmd.Flags().BoolVar(&syncFilesystem, "filesystem", true, "Scan filesystem (default: true)")
	cmd.Flags().BoolVar(&showStats, "stats", true, "Show database stats after scan")

	return cmd
}

func newLibraryStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show library statistics and health",
		Long: `Display HOLDEN database status, statistics, and health information.

Shows:
  - Database file location and size
  - Total files, series, and movies
  - Duplicate groups count
  - Unresolved conflicts
  - Last sync information
  - Database health metrics`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunStatus()
		},
	}

	return cmd
}

func newLibraryValidateCmd() *cobra.Command {
	var recursive bool
	var verbose bool

	cmd := &cobra.Command{
		Use:   "validate <path>",
		Short: "Check library for naming issues",
		Long: `Check if media files follow Jellyfin naming conventions.

Reports issues like:
  - Missing year in parentheses
  - Release group markers in filename
  - Wrong season folder structure

Examples:
  jellywatch library validate /media/TV/Silo/
  jellywatch library validate /media/Movies/ --recursive`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunValidate(args[0], recursive, verbose)
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", true, "validate subdirectories")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	return cmd
}
