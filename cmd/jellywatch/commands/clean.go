package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

type cleanMode int

const (
	cleanModeInteractive cleanMode = iota
	cleanModeDuplicates
	cleanModeNaming
	cleanModeAll
)

func NewCleanCmd() *cobra.Command {
	var (
		duplicates bool
		naming     bool
		all        bool
		dryRun     bool
		fix        bool
		safeOnly   bool
		generate   bool
		execute    bool
		status     bool
	)

	cmd := &cobra.Command{
		Use:   "clean [--duplicates|--naming|--all]",
		Short: "Clean up library issues (duplicates, naming, etc.)",
		Long: `Clean up your library by removing duplicates and fixing naming issues.

Modes:
  --duplicates    Remove duplicate files (keeps highest quality)
  --naming        Fix Jellyfin naming compliance issues
  --all           Run all cleanup tasks
  (no flags)      Interactive mode - choose what to clean

Examples:
  jellywatch clean                    # Interactive cleanup wizard
  jellywatch clean --duplicates       # Remove duplicates only
  jellywatch clean --naming --dry-run # Preview naming fixes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := selectCleanMode(duplicates, naming, all)

			switch mode {
			case cleanModeDuplicates:
				return runCleanDuplicates(dryRun, generate, execute, status)
			case cleanModeNaming:
				return runCleanNaming(dryRun, fix, safeOnly)
			case cleanModeAll:
				return runCleanAll(dryRun, fix, safeOnly, generate, execute, status)
			case cleanModeInteractive:
				return runCleanInteractive()
			default:
				return fmt.Errorf("invalid clean mode")
			}
		},
	}

	cmd.Flags().BoolVar(&duplicates, "duplicates", false, "Remove duplicate files")
	cmd.Flags().BoolVar(&naming, "naming", false, "Fix naming issues")
	cmd.Flags().BoolVar(&all, "all", false, "Run all cleanup tasks")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Preview changes")
	cmd.Flags().BoolVar(&fix, "fix", false, "Execute fixes (for naming)")
	cmd.Flags().BoolVar(&safeOnly, "safe-only", true, "Only fix safe issues (case/punctuation)")
	cmd.Flags().BoolVar(&generate, "generate", false, "Generate plans (for duplicates)")
	cmd.Flags().BoolVar(&execute, "execute", false, "Execute plans (for duplicates)")
	cmd.Flags().BoolVar(&status, "status", false, "Show plan status (for duplicates)")

	return cmd
}

func selectCleanMode(duplicates, naming, all bool) cleanMode {
	if all {
		return cleanModeAll
	}
	if duplicates {
		return cleanModeDuplicates
	}
	if naming {
		return cleanModeNaming
	}
	return cleanModeInteractive
}

func runCleanDuplicates(dryRun, generate, execute, status bool) error {
	// Generate plans if needed
	if generate {
		return RunConsolidate(true, false, false, false)
	}

	// Show status if requested
	if status {
		return RunConsolidate(false, false, false, true)
	}

	// Execute with dry-run flag
	if execute || dryRun {
		return RunConsolidate(false, dryRun, execute, false)
	}

	// Default: show summary
	return RunConsolidate(false, false, false, false)
}

func runCleanNaming(dryRun, fix, safeOnly bool) error {
	fixDry := dryRun && !fix
	actualFix := fix && !dryRun
	return RunCompliance(fixDry, actualFix, safeOnly, false, false)
}

func runCleanAll(dryRun, fix, safeOnly, generate, execute, status bool) error {
	// Run duplicates first
	if err := runCleanDuplicates(dryRun, generate, execute, status); err != nil {
		return fmt.Errorf("duplicates cleanup failed: %w", err)
	}
	// Then naming
	return runCleanNaming(dryRun, fix, safeOnly)
}

func runCleanInteractive() error {
	// TODO: Phase 4 - Implement TUI
	return fmt.Errorf("interactive mode not yet implemented - use --duplicates or --naming")
}
