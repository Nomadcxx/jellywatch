package main

import (
	"fmt"

	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/database"
	"github.com/spf13/cobra"
)

func newReviewCmd() *cobra.Command {
	var (
		retryAI bool
		summary bool
	)

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Review skipped items that need manual attention",
		Long: `Review and resolve items that couldn't be processed automatically.

Items end up here when:
  - Filename couldn't be parsed
  - No clear quality winner among duplicates
  - AI failed to resolve ambiguity

Examples:
  jellywatch review              # List all pending items
  jellywatch review --summary    # Show counts by reason
  jellywatch review --retry-ai   # Retry all with AI (if available)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReview(retryAI, summary)
		},
	}

	cmd.Flags().BoolVar(&retryAI, "retry-ai", false, "Retry all pending items with AI")
	cmd.Flags().BoolVar(&summary, "summary", false, "Show summary statistics only")

	return cmd
}

func runReview(retryAI, summaryOnly bool) error {
	// Open database
	dbPath := config.GetDatabasePath()
	db, err := database.OpenPath(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if summaryOnly {
		return showReviewSummary(db)
	}

	// Get pending items
	items, err := db.GetPendingSkippedItems()
	if err != nil {
		return fmt.Errorf("failed to get skipped items: %w", err)
	}

	if len(items) == 0 {
		fmt.Println("No items pending review")
		return nil
	}

	fmt.Printf("=== Pending Review: %d items ===\n\n", len(items))

	// Group by reason
	byReason := make(map[database.SkipReason][]database.SkippedItem)
	for _, item := range items {
		byReason[item.SkipReason] = append(byReason[item.SkipReason], item)
	}

	for reason, items := range byReason {
		fmt.Printf("[%s] %d items\n", reason, len(items))
		for _, item := range items {
			fmt.Printf("  %s\n", item.Path)
			if item.ErrorDetails != "" {
				fmt.Printf("    Error: %s\n", item.ErrorDetails)
			}
			if item.AIAttempted {
				fmt.Printf("    AI attempted: %s\n", item.AIResult)
			}
			fmt.Printf("    Attempts: %d\n", item.Attempts)
		}
		fmt.Println()
	}

	if retryAI {
		return retryWithAI(db, items)
	}

	fmt.Println("Options:")
	fmt.Println("  --retry-ai    Retry all with AI assistance")
	fmt.Println("  --summary     Show statistics only")

	return nil
}

func showReviewSummary(db *database.MediaDB) error {
	byReason, err := db.CountSkippedByReason()
	if err != nil {
		return err
	}

	byStatus, err := db.CountSkippedByStatus()
	if err != nil {
		return err
	}

	fmt.Println("=== Review Queue Summary ===\n")

	fmt.Println("By Status:")
	for status, count := range byStatus {
		fmt.Printf("  %s: %d\n", status, count)
	}

	fmt.Println("\nPending by Reason:")
	total := 0
	for reason, count := range byReason {
		fmt.Printf("  %s: %d\n", reason, count)
		total += count
	}

	fmt.Printf("\nTotal pending: %d\n", total)

	return nil
}

func retryWithAI(db *database.MediaDB, items []database.SkippedItem) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.AI.Enabled {
		fmt.Println("AI is not enabled. Configure [ai] section in config.toml")
		return nil
	}

	fmt.Printf("Retrying %d items with AI...\n\n", len(items))

	// TODO: Integrate with AI package
	// For now, just mark as attempted
	for _, item := range items {
		if item.AIAttempted {
			fmt.Printf("  Skipped (already tried): %s\n", item.Path)
			continue
		}

		// Would call AI here
		fmt.Printf("  Would process: %s\n", item.Path)
		db.MarkAIAttempted(item.ID, "AI retry not yet implemented")
	}

	return nil
}
