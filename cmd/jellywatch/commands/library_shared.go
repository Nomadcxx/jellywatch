package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/database"
	"github.com/Nomadcxx/jellywatch/internal/radarr"
	"github.com/Nomadcxx/jellywatch/internal/sonarr"
	"github.com/Nomadcxx/jellywatch/internal/sync"
	"github.com/Nomadcxx/jellywatch/internal/ui"
	"github.com/Nomadcxx/jellywatch/internal/validator"
	"time"
)

// RunScan executes library scanning
func RunScan(syncSonarr, syncRadarr, syncFilesystem, showStats bool) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open database
	dbPath := config.GetDatabasePath()
	db, err := database.OpenPath(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	fmt.Printf("Database: %s\n\n", dbPath)

	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create sync service
	var sonarrClient *sonarr.Client
	var radarrClient *radarr.Client

	if syncSonarr && cfg.Sonarr.Enabled {
		sonarrClient = sonarr.NewClient(sonarr.Config{
			URL:    cfg.Sonarr.URL,
			APIKey: cfg.Sonarr.APIKey,
		})
	}

	if syncRadarr && cfg.Radarr.Enabled {
		radarrClient = radarr.NewClient(radarr.Config{
			URL:    cfg.Radarr.URL,
			APIKey: cfg.Radarr.APIKey,
		})
	}

	syncService := sync.NewSyncService(sync.SyncConfig{
		DB:             db,
		Sonarr:         sonarrClient,
		Radarr:         radarrClient,
		TVLibraries:    cfg.Libraries.TV,
		MovieLibraries: cfg.Libraries.Movies,
		Logger:         logger,
	})

	ctx := context.Background()
	startTime := time.Now()

	// Sync from Sonarr first (lower priority, will be overwritten by filesystem)
	if syncSonarr && cfg.Sonarr.Enabled {
		fmt.Println("Syncing from Sonarr...")
		if err := syncService.SyncFromSonarr(ctx); err != nil {
			fmt.Printf("  Warning: Sonarr sync failed: %v\n", err)
		} else {
			fmt.Println("  Sonarr sync complete")
		}
	}

	// Sync from Radarr
	if syncRadarr && cfg.Radarr.Enabled {
		fmt.Println("Syncing from Radarr...")
		if err := syncService.SyncFromRadarr(ctx); err != nil {
			fmt.Printf("  Warning: Radarr sync failed: %v\n", err)
		} else {
			fmt.Println("  Radarr sync complete")
		}
	}

	// Scan filesystem (higher priority)
	if syncFilesystem {
		fmt.Printf("Scanning %d TV libraries...\n", len(cfg.Libraries.TV))
		for _, lib := range cfg.Libraries.TV {
			fmt.Printf("  %s\n", lib)
		}
		fmt.Printf("Scanning %d movie libraries...\n", len(cfg.Libraries.Movies))
		for _, lib := range cfg.Libraries.Movies {
			fmt.Printf("  %s\n", lib)
		}
		fmt.Println()

		if err := syncService.SyncFromFilesystem(ctx); err != nil {
			return fmt.Errorf("filesystem sync failed: %w", err)
		}
		fmt.Println("Filesystem sync complete")
	}

	duration := time.Since(startTime)
	fmt.Printf("\nScan completed in %s\n", duration.Round(time.Millisecond))

	// Show stats
	if showStats {
		fmt.Println("\n=== Database Stats ===")

		// Count series
		var seriesCount int
		for _, lib := range cfg.Libraries.TV {
			count, _ := db.CountSeriesInLibrary(lib)
			seriesCount += count
		}
		fmt.Printf("TV Series: %d\n", seriesCount)

		// Count movies
		var movieCount int
		for _, lib := range cfg.Libraries.Movies {
			count, _ := db.CountMoviesInLibrary(lib)
			movieCount += count
		}
		fmt.Printf("Movies: %d\n", movieCount)

		// Check for conflicts
		conflicts, err := db.GetUnresolvedConflicts()
		if err == nil {
			if len(conflicts) > 0 {
				fmt.Printf("\n=== Conflicts Detected: %d ===\n", len(conflicts))
				for _, c := range conflicts {
					yearStr := ""
					if c.Year != nil {
						yearStr = fmt.Sprintf(" (%d)", *c.Year)
					}
					fmt.Printf("  [%s] %s%s\n", c.MediaType, c.Title, yearStr)
					for _, loc := range c.Locations {
						fmt.Printf("    - %s\n", loc)
					}
				}
				fmt.Printf("\nRun 'jellywatch clean --duplicates --dry-run' to see consolidation plan\n")
			} else {
				fmt.Println("\nNo conflicts detected")
			}
		}
	}

	return nil
}

// RunStatus shows database status
func RunStatus() error {
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

// RunValidate validates library files
func RunValidate(path string, recursive, verbose bool) error {
	v := validator.NewValidator()

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot access path: %w", err)
	}

	var valid, invalid int

	validate := func(filePath string) {
		result, err := v.ValidateFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "? %s: %v\n", filepath.Base(filePath), err)
			return
		}

		if result.Valid {
			valid++
			if verbose {
				fmt.Printf("✓ %s\n", filepath.Base(filePath))
			}
		} else {
			invalid++
			fmt.Printf("✗ %s\n", filepath.Base(filePath))
			for _, issue := range result.Issues {
				fmt.Printf("  - %s\n", issue)
			}
			if result.ExpectedName != "" {
				fmt.Printf("  Expected: %s\n", result.ExpectedName)
			}
		}
	}

	if info.IsDir() {
		err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if !recursive && filePath != path {
					return filepath.SkipDir
				}
				return nil
			}
			validate(filePath)
			return nil
		})
	} else {
		validate(path)
	}

	fmt.Printf("\nValidation: %d valid, %d invalid\n", valid, invalid)
	return err
}
