package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Nomadcxx/jellywatch/internal/compliance"
	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/database"
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
		Short: "Check and fix Jellyfin naming compliance",
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

	return cmd
}

func runCompliance(fixDry, fix, safeOnly, moviesOnly, tvOnly bool) error {
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
		fmt.Println("All files are Jellyfin compliant!")
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

	for _, f := range files {
		checker := compliance.NewChecker(f.LibraryRoot)
		suggestion, err := checker.SuggestCompliantPath(f.Path)
		if err != nil {
			fmt.Printf("  ? %s: %v\n", filepath.Base(f.Path), err)
			continue
		}
		if suggestion == nil {
			continue
		}

		suggestions = append(suggestions, suggestion)

		if suggestion.IsSafeAutoFix {
			safeCount++
		} else {
			riskyCount++
		}

		// Display
		tag := "[SAFE]"
		if !suggestion.IsSafeAutoFix {
			tag = "[RISKY]"
		}

		if !fix || fixDry {
			fmt.Printf("%s %s\n", tag, filepath.Base(f.Path))
			fmt.Printf("  Current:  %s\n", f.Path)
			fmt.Printf("  Fix:      %s\n", suggestion.SuggestedPath)
			fmt.Printf("  Action:   %s\n", suggestion.Description)
			if len(suggestion.Issues) > 0 {
				fmt.Printf("  Issues:   %s\n", strings.Join(suggestion.Issues, ", "))
			}
			fmt.Println()
		}
	}

	fmt.Printf("Summary: %d safe fixes, %d risky fixes\n", safeCount, riskyCount)

	if fixDry {
		fmt.Println("\n[dry-run] No changes made")
		return nil
	}

	if !fix {
		fmt.Println("\nRun with --fix-dry to preview or --fix to execute")
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
			fmt.Printf("  Failed: %s: %v\n", filepath.Base(s.OriginalPath), err)
			failed++
			continue
		}

		fmt.Printf("  Fixed: %s\n", filepath.Base(s.OriginalPath))
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

	fmt.Printf("\nResult: %d fixed, %d skipped, %d failed\n", fixed, skipped, failed)

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
