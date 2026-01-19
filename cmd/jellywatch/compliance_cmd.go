package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Nomadcxx/jellywatch/internal/compliance"
	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/consolidate"
	"github.com/Nomadcxx/jellywatch/internal/database"
	"github.com/Nomadcxx/jellywatch/internal/ui"
	"github.com/spf13/cobra"
)

func newComplianceCmd() *cobra.Command {
	var (
		fixDry   bool
		fix      bool
		safeOnly bool
		movies   bool
		tv       bool
	)

	cmd := &cobra.Command{
		Use:   "compliance",
		Short: "[DEPRECATED] Use 'jellywatch clean --naming' instead",
		Long: `Analyze media files for Jellyfin naming compliance and optionally fix them.

This command identifies files that don't follow Jellyfin naming conventions:
  - Wrong case in titles (e.g., "Of" instead of "of")
  - Missing year in parentheses
  - Release markers in filename
  - Wrong folder structure

Examples:
  jellywatch compliance              # List all non-compliant files
  jellywatch compliance --fix-dry    # Show what would be fixed
  jellywatch compliance --fix        # Fix safe issues (case/punctuation only)
  jellywatch compliance --fix --all  # Fix all issues (including risky)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "⚠️  Warning: 'jellywatch compliance' is deprecated. Use 'jellywatch clean --naming' instead.")
			return runCompliance(fixDry, fix, safeOnly, movies, tv)
		},
	}

	cmd.Flags().BoolVar(&fixDry, "fix-dry", false, "Preview fixes without executing")
	cmd.Flags().BoolVar(&fix, "fix", false, "Execute fixes")
	cmd.Flags().BoolVar(&safeOnly, "safe-only", true, "Only fix safe issues (case/punctuation)")
	cmd.Flags().BoolVar(&movies, "movies", false, "Only check movies")
	cmd.Flags().BoolVar(&tv, "tv", false, "Only check TV shows")
	cmd.Flags().Bool("all", false, "Fix all issues including risky (use with --fix)")

	// Hide --all flag, only show in help when needed
	cmd.Flags().MarkHidden("all")
	cmd.Hidden = true // Hide from help output

	return cmd
}

func runCompliance(fixDry, fix, safeOnly, moviesOnly, tvOnly bool) error {
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

	// Get non-compliant files from database
	files, err := db.FindNonCompliantFiles()
	if err != nil {
		return fmt.Errorf("failed to query non-compliant files: %w", err)
	}

	if len(files) == 0 {
		ui.SuccessMsg("All files are Jellyfin compliant!")
		return nil
	}

	// Filter by type if requested
	var filtered []*database.MediaFile
	for _, f := range files {
		if moviesOnly && f.MediaType != "movie" {
			continue
		}
		if tvOnly && f.MediaType != "episode" {
			continue
		}
		filtered = append(filtered, f)
	}
	files = filtered

	fmt.Printf("Found %d non-compliant files\n\n", len(files))

	// Group by issue type for display
	safeCount := 0
	riskyCount := 0
	var suggestions []*compliance.ComplianceSuggestion
	var tableRows [][]string

	for _, f := range files {
		checker := compliance.NewChecker(f.LibraryRoot)
		suggestion, err := checker.SuggestCompliantPath(f.Path)
		if err != nil {
			ui.ErrorMsg("%s: %v", filepath.Base(f.Path), err)
			continue
		}
		if suggestion == nil {
			continue
		}

		suggestions = append(suggestions, suggestion)

		classification, err := compliance.ClassifySuggestion(f.Path, suggestion.SuggestedPath)
		if err != nil {
			ui.ErrorMsg("%s: failed to classify: %v", filepath.Base(f.Path), err)
			continue
		}

		switch classification {
		case compliance.ClassificationSafe:
			safeCount++
		case compliance.ClassificationRisky:
			riskyCount++
		case compliance.ClassificationUnknown:
		}

		// Build table row
		if !fix || fixDry {
			statusStr := classification.String()
			if classification == compliance.ClassificationSafe {
				statusStr = ui.Safe(statusStr)
			} else if classification == compliance.ClassificationRisky {
				statusStr = ui.Risky(statusStr)
			}
			
			issuesStr := strings.Join(suggestion.Issues, ", ")
			if len(issuesStr) > 50 {
				issuesStr = issuesStr[:47] + "..."
			}
			
			tableRows = append(tableRows, []string{
				filepath.Base(f.Path),
				statusStr,
				issuesStr,
			})
		}
	}

	// Display table if not fixing
	if (!fix || fixDry) && len(tableRows) > 0 {
		table := ui.NewTable("File", "Status", "Issues")
		for _, row := range tableRows {
			table.AddRow(row...)
		}
		table.Render()
		fmt.Println()
	}

	ui.Section("Summary")
	fmt.Printf("Safe fixes:  %s\n", ui.Safe(fmt.Sprintf("%d", safeCount)))
	fmt.Printf("Risky fixes: %s\n", ui.Risky(fmt.Sprintf("%d", riskyCount)))

	if fixDry {
		ui.InfoMsg("Dry run complete - no changes made")
		return nil
	}

	if !fix {
		fmt.Println()
		ui.InfoMsg("Run with --fix-dry to preview or --fix to execute")
		return nil
	}

	// Execute fixes
	fixed := 0
	skipped := 0
	failed := 0

	for _, s := range suggestions {
		// Skip risky if safe-only
		if safeOnly && !s.IsSafeAutoFix {
			skipped++
			continue
		}

		// Execute the fix
		err := executeComplianceFix(s, db)
		if err != nil {
			ui.ErrorMsg("%s: %v", filepath.Base(s.OriginalPath), err)
			failed++
			continue
		}

		ui.SuccessMsg("Fixed: %s", filepath.Base(s.OriginalPath))
		fixed++

		// Log operation
		db.LogOperation(
			database.OpRename,
			s.OriginalPath,
			s.SuggestedPath,
			s.Description,
			0, 0, 0,
			database.ExecCLI,
		)
	}

	ui.Section("Fix Results")
	fmt.Printf("Fixed:   %s\n", ui.Success(fmt.Sprintf("%d", fixed)))
	fmt.Printf("Skipped: %d\n", skipped)
	if failed > 0 {
		fmt.Printf("Failed:  %s\n", ui.Error(fmt.Sprintf("%d", failed)))
	}

	// Notify Sonarr/Radarr of changes
	if fixed > 0 {
		notifier := consolidate.NewNotifier(cfg, slog.Default(), false)

		var tvPaths, moviePaths []string
		for _, s := range suggestions {
			if s.IsSafeAutoFix || !safeOnly {
				if strings.Contains(s.SuggestedPath, "/Season") || strings.Contains(s.SuggestedPath, "\\Season") {
					tvPaths = append(tvPaths, s.SuggestedPath)
				} else {
					moviePaths = append(moviePaths, s.SuggestedPath)
				}
			}
		}

		if err := notifier.NotifyBulkComplete(tvPaths, moviePaths); err != nil {
			fmt.Printf("Warning: some *arr notifications failed: %v\n", err)
		} else if len(tvPaths) > 0 || len(moviePaths) > 0 {
			fmt.Println("Sonarr/Radarr notified of changes")
		}
	}

	return nil
}

func executeComplianceFix(s *compliance.ComplianceSuggestion, db *database.MediaDB) error {
	// Ensure target directory exists
	targetDir := filepath.Dir(s.SuggestedPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Rename/move the file
	if err := os.Rename(s.OriginalPath, s.SuggestedPath); err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	// Try to remove empty source directories
	sourceDir := filepath.Dir(s.OriginalPath)
	for sourceDir != "/" {
		entries, err := os.ReadDir(sourceDir)
		if err != nil || len(entries) > 0 {
			break
		}
		os.Remove(sourceDir)
		sourceDir = filepath.Dir(sourceDir)
	}

	// Update database
	return db.UpdateMediaFilePath(s.OriginalPath, s.SuggestedPath)
}
