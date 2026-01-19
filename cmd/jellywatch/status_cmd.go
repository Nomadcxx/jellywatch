package main

import (
	"fmt"
	"os"

	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/database"
	"github.com/Nomadcxx/jellywatch/internal/ui"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "[DEPRECATED] Use 'jellywatch library status' instead",
		Long: `Display HOLDEN database status, statistics, and health information.

Shows:
  - Database file location and size
  - Total files, series, and movies
  - Duplicate groups count
  - Unresolved conflicts
  - Last sync information
  - Database health metrics`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "⚠️  Warning: 'jellywatch status' is deprecated. Use 'jellywatch library status' instead.")
			return runStatus(cmd, args)
		},
	}

	cmd.Hidden = true // Hide from help output
	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Open database
	dbPath := config.GetDatabasePath()
	db, err := database.OpenPath(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Get database path and size
	info, err := os.Stat(dbPath)
	if err != nil {
		return fmt.Errorf("failed to stat database: %w", err)
	}

	// Get statistics
	stats, err := db.GetConsolidationStats()
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	// Get conflicts
	conflicts, err := db.GetUnresolvedConflicts()
	if err != nil {
		return fmt.Errorf("failed to get conflicts: %w", err)
	}

	// Get sync history
	syncLogs, err := db.GetRecentSyncLogs(10) // Last 10 syncs
	if err != nil {
		return fmt.Errorf("failed to get sync history: %w", err)
	}

	// Get actual episode and movie counts
	episodeCount, err := db.CountMediaFilesByType("episode")
	if err != nil {
		episodeCount = 0
	}
	movieCount, err := db.CountMediaFilesByType("movie")
	if err != nil {
		movieCount = 0
	}

	// Display
	ui.Section("HOLDEN Database Status")
	fmt.Printf("Database: %s\n", ui.Path(dbPath))
	fmt.Printf("Size:     %s\n", ui.FormatBytes(info.Size()))
	fmt.Printf("Modified: %s\n\n", info.ModTime().Format("2006-01-02 15:04:05"))

	ui.Section("Statistics")
	statsRows := [][]string{
		{"Total Files", fmt.Sprintf("%d", stats.TotalFiles)},
		{"TV Episodes", fmt.Sprintf("%d", episodeCount)},
		{"Movies", fmt.Sprintf("%d", movieCount)},
		{"Duplicate Groups", fmt.Sprintf("%d", stats.DuplicateGroups)},
		{"Non-Compliant Files", fmt.Sprintf("%d", stats.NonCompliantFiles)},
	}
	if stats.SpaceReclaimable > 0 {
		statsRows = append(statsRows, []string{"Space Reclaimable", ui.FormatBytes(stats.SpaceReclaimable)})
	}
	ui.CompactTable([]string{"Metric", "Value"}, statsRows)
	fmt.Println()

	ui.Section("Conflicts")
	if len(conflicts) == 0 {
		ui.SuccessMsg("No unresolved conflicts")
	} else {
		ui.WarningMsg("Unresolved: %d", len(conflicts))
		for i, c := range conflicts {
			if i >= 5 {
				fmt.Printf("  ... and %d more\n", len(conflicts)-5)
				break
			}
			yearStr := ""
			if c.Year != nil {
				yearStr = fmt.Sprintf(" (%d)", *c.Year)
			}
			fmt.Printf("  • %s%s - %d locations\n", c.Title, yearStr, len(c.Locations))
		}
	}
	fmt.Println()

	ui.Section("Sync History")
	if len(syncLogs) == 0 {
		ui.InfoMsg("No sync history")
	} else {
		for _, log := range syncLogs {
			statusIcon := ui.Success("✓")
			statusText := log.Status
			if log.Status == "failed" {
				statusIcon = ui.Error("✗")
				statusText = ui.Error(log.Status)
			}
			completedTime := "running"
			if log.CompletedAt != nil {
				completedTime = log.CompletedAt.Format("2006-01-02 15:04")
			}
			fmt.Printf("%s %s - %s (%s)\n",
				statusIcon,
				log.Source,
				completedTime,
				statusText)
		}
	}

	return nil
}
