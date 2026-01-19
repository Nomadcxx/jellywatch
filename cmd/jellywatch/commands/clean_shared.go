package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Nomadcxx/jellywatch/internal/compliance"
	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/consolidate"
	"github.com/Nomadcxx/jellywatch/internal/database"
	"github.com/Nomadcxx/jellywatch/internal/ui"
)

// RunCompliance executes compliance checking and fixing
func RunCompliance(fixDry, fix, safeOnly, moviesOnly, tvOnly bool) error {
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

	ui.Section("Compliance Check")
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
		notifier := consolidate.NewNotifier(cfg, nil, false)

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

// RunConsolidate executes consolidation plan generation or execution
func RunConsolidate(generate, dryRun, execute, status bool) error {
	// Open database
	db, err := database.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Generate plans if requested
	if generate {
		return runGeneratePlans(ctx, db)
	}

	// Show status if requested
	if status {
		return runConsolidateStatus(db)
	}

	// Execute plans (dry-run or actual)
	if execute || dryRun {
		return runExecutePlans(ctx, db, dryRun)
	}

	// Default: Show summary and guide user
	return runConsolidateSummary(db)
}

func runGeneratePlans(ctx context.Context, db *database.MediaDB) error {
	fmt.Println("🔍 Analyzing database for duplicates and issues...")

	planner := consolidate.NewPlanner(db)
	summary, err := planner.GeneratePlans(ctx)
	if err != nil {
		return fmt.Errorf("failed to generate plans: %w", err)
	}

	fmt.Println("\n✅ Plans generated successfully!")
	fmt.Printf("\nPlan Summary:\n")
	fmt.Printf("  Total plans:      %d\n", summary.TotalPlans)
	fmt.Printf("  Delete plans:     %d\n", summary.DeletePlans)
	fmt.Printf("  Move plans:       %d\n", summary.MovePlans)
	fmt.Printf("  Rename plans:     %d\n", summary.RenamePlans)
	fmt.Printf("  Duplicate groups: %d\n", summary.DuplicateGroups)
	fmt.Printf("  Space reclaimable: %s\n", ui.FormatBytes(summary.SpaceToReclaim))

	if summary.TotalPlans > 0 {
		fmt.Println("\nNext steps:")
		fmt.Println("  jellywatch clean --duplicates --dry-run   # Preview what will happen")
		fmt.Println("  jellywatch clean --duplicates             # Execute the plans")
	} else {
		fmt.Println("\n✨ No consolidation needed - your library is already optimized!")
	}

	return nil
}

func runConsolidateStatus(db *database.MediaDB) error {
	planner := consolidate.NewPlanner(db)
	summary, err := planner.GetPlanSummary()
	if err != nil {
		return fmt.Errorf("failed to get plan summary: %w", err)
	}

	fmt.Println("=== Consolidation Status ===")
	fmt.Printf("\nPending Plans:\n")
	fmt.Printf("  Total:    %d\n", summary.TotalPlans)
	fmt.Printf("  Delete:   %d\n", summary.DeletePlans)
	fmt.Printf("  Move:     %d\n", summary.MovePlans)
	fmt.Printf("  Rename:   %d\n", summary.RenamePlans)
	fmt.Printf("\nSpace to reclaim: %s\n", ui.FormatBytes(summary.SpaceToReclaim))

	if summary.TotalPlans == 0 {
		fmt.Println("\nNo pending plans. Run 'jellywatch clean --duplicates --generate' to create new plans.")
	} else {
		fmt.Println("\nRun 'jellywatch clean --duplicates --dry-run' to preview actions.")
		fmt.Println("Run 'jellywatch clean --duplicates' to execute plans.")
	}

	return nil
}

func runConsolidateSummary(db *database.MediaDB) error {
	planner := consolidate.NewPlanner(db)
	summary, err := planner.GetPlanSummary()
	if err != nil {
		return fmt.Errorf("failed to get plan summary: %w", err)
	}

	if summary.TotalPlans == 0 {
		fmt.Println("No pending plans to execute.")
		fmt.Println("Run 'jellywatch clean --duplicates --generate' to create plans.")
		return nil
	}

	fmt.Println("=== Consolidation Summary ===")
	fmt.Printf("Pending plans: %d\n", summary.TotalPlans)
	fmt.Printf("Space to reclaim: %s\n", ui.FormatBytes(summary.SpaceToReclaim))
	fmt.Print("\nRun 'jellywatch clean --duplicates --dry-run' to preview or 'jellywatch clean --duplicates' to execute.\n")

	return nil
}

func runExecutePlans(ctx context.Context, db *database.MediaDB, dryRun bool) error {
	planner := consolidate.NewPlanner(db)

	plans, err := planner.GetPendingPlans()
	if err != nil {
		return fmt.Errorf("failed to get pending plans: %w", err)
	}

	if len(plans) == 0 {
		fmt.Println("No pending plans to execute.")
		fmt.Println("Run 'jellywatch clean --duplicates --generate' to create plans.")
		return nil
	}

	if dryRun {
		fmt.Print("🔍 DRY RUN - No changes will be made\n\n")
	} else {
		fmt.Print("⚠️  Executing consolidation plans...\n\n")
	}

	fmt.Printf("Found %d pending plans:\n\n", len(plans))

	deleteCount := 0
	moveCount := 0
	renameCount := 0

	for i, plan := range plans {
		switch plan.Action {
		case "delete":
			deleteCount++
			// Get file info for size
			file, _ := db.GetMediaFileByID(plan.SourceFileID)
			size := ""
			if file != nil {
				size = ui.FormatBytes(file.Size)
			}
			fmt.Printf("%d. DELETE: %s (%s)\n", i+1, plan.SourcePath, size)
			fmt.Printf("   Reason: %s\n", plan.Reason)
			if plan.ReasonDetails != "" {
				fmt.Printf("   %s\n", plan.ReasonDetails)
			}
		case "move":
			moveCount++
			fmt.Printf("%d. MOVE: %s\n", i+1, plan.SourcePath)
			fmt.Printf("   To: %s\n", plan.TargetPath)
			fmt.Printf("   Reason: %s\n", plan.Reason)
		case "rename":
			renameCount++
			fmt.Printf("%d. RENAME: %s\n", i+1, plan.SourcePath)
			fmt.Printf("   To: %s\n", plan.TargetPath)
			fmt.Printf("   Reason: %s\n", plan.Reason)
		}
		fmt.Println()
	}

	fmt.Println()

	fmt.Printf("Summary: %d deletes, %d moves, %d renames\n\n", deleteCount, moveCount, renameCount)

	if dryRun {
		fmt.Println("✅ Dry run complete - no changes made")
		fmt.Println("\nTo execute these plans, run:")
		fmt.Println("  jellywatch clean --duplicates")
		return nil
	}

	// Execute plans
	fmt.Println("Executing plans...")
	executor := consolidate.NewExecutor(db, dryRun)
	result, err := executor.ExecutePlans(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute plans: %w", err)
	}

	fmt.Println("\n=== Execution Complete ===")
	fmt.Printf("Plans executed:  %d\n", result.PlansExecuted)
	fmt.Printf("Succeeded:       %d\n", result.PlansSucceeded)
	fmt.Printf("Failed:          %d\n", result.PlansFailed)
	fmt.Printf("Files deleted:   %d\n", result.FilesDeleted)
	fmt.Printf("Files moved:     %d\n", result.FilesMoved)
	fmt.Printf("Files renamed:   %d\n", result.FilesRenamed)
	fmt.Printf("Space reclaimed: %s\n", ui.FormatBytes(result.SpaceReclaimed))

	return nil
}
