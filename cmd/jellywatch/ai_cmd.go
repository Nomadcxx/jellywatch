package main

import (
	"fmt"

	"github.com/Nomadcxx/jellywatch/internal/app"
	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/database"
	"github.com/spf13/cobra"
)

func newAICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "AI title enhancement commands",
		Long:  `Commands for interacting with the AI title enhancement feature.`,
	}

	cmd.AddCommand(newAIStatusCmd())
	cmd.AddCommand(newAITestCmd())

	return cmd
}

func newAIStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show AI integration status",
		Long: `Display AI integration configuration and metrics.

Shows:
  - Configuration status (enabled/disabled)
  - Ollama endpoint and model
  - Timeout and cache settings
  - Confidence threshold
  - Cache hit rate and AI call metrics`,
		RunE: runAIStatus,
	}
}

func runAIStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println("AI Integration Status")
	fmt.Println("======================")

	if !cfg.AI.Enabled {
		fmt.Println("Status: Disabled")
		fmt.Printf("Endpoint: %s\n", cfg.AI.OllamaEndpoint)
		fmt.Printf("Model: %s\n", cfg.AI.Model)
		fmt.Println("\nEnable with 'enabled = true' in config.toml or use --ai flag.")
		return nil
	}

	fmt.Println("Status: Enabled")
	fmt.Printf("Endpoint: %s\n", cfg.AI.OllamaEndpoint)
	fmt.Printf("Model: %s\n", cfg.AI.Model)
	if cfg.AI.CloudModel != "" {
		fmt.Printf("Cloud Model: %s\n", cfg.AI.CloudModel)
	}
	fmt.Printf("Timeout: %ds\n", cfg.AI.TimeoutSeconds)
	fmt.Printf("Cache: %v\n", cfg.AI.CacheEnabled)
	fmt.Printf("Confidence Threshold: %.2f\n", cfg.AI.ConfidenceThreshold)

	// Try to connect and get metrics
	dbPath := config.GetDatabasePath()
	db, err := database.OpenPath(dbPath)
	if err != nil {
		fmt.Printf("\nWarning: Could not open database: %v\n", err)
		return nil
	}
	defer db.Close()

	aiIntegrator, err := app.InitAI(cfg, db)
	if err != nil {
		fmt.Printf("\nError: AI initialization failed: %v\n", err)
		return nil
	}
	if aiIntegrator == nil {
		fmt.Println("\nAI integrator not available (check configuration)")
		return nil
	}
	defer aiIntegrator.Close()

	metrics := aiIntegrator.GetMetrics()
	summary := metrics.Summary()

	fmt.Println("\nAI Metrics:")
	fmt.Printf("  Total Parses: %d\n", summary["total"])
	fmt.Printf("  Cache Hits: %d (%.1f%%)\n",
		metrics.CacheHits.Load(), summary["cache_hit_rate"])
	fmt.Printf("  Regex Used: %d (%.1f%%)\n",
		metrics.RegexUsed.Load(), summary["regex_rate"])
	fmt.Printf("  AI Calls: %d (%.1f%%)\n",
		metrics.AIUsed.Load(), summary["ai_rate"])
	if summary["ai_avg_latency_ms"].(int64) > 0 {
		fmt.Printf("  Avg AI Latency: %dms\n", summary["ai_avg_latency_ms"])
	}
	if summary["ai_fallback_rate"].(float64) > 0 {
		fmt.Printf("  AI Fallback Rate: %.1f%%\n", summary["ai_fallback_rate"])
	}
	if summary["ai_timeouts"].(int64) > 0 {
		fmt.Printf("  AI Timeouts: %d\n", summary["ai_timeouts"])
	}
	if summary["ai_errors"].(int64) > 0 {
		fmt.Printf("  AI Errors: %d\n", summary["ai_errors"])
	}

	return nil
}

func newAITestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <filename>",
		Short: "Test AI title enhancement on a filename",
		Long: `Test the AI title enhancement on a specific filename.

This command shows how the AI enhancement system would process a filename,
including whether it uses cached results, regex, or AI matching.`,
		Args: cobra.ExactArgs(1),
		RunE: runAITest,
	}
}

func runAITest(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.AI.Enabled {
		return fmt.Errorf("AI is not enabled. Enable it in config.toml [ai] section")
	}

	filename := args[0]

	dbPath := config.GetDatabasePath()
	db, err := database.OpenPath(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	aiIntegrator, err := app.InitAI(cfg, db)
	if err != nil {
		return fmt.Errorf("AI initialization failed: %w", err)
	}
	if aiIntegrator == nil {
		return fmt.Errorf("AI integrator not available")
	}
	defer aiIntegrator.Close()

	// Determine media type
	mediaType := "movie"
	if filenameContainsEpisodePattern(filename) {
		mediaType = "tv"
	}

	// Run enhancement with filename as baseline
	enhanced, source, err := aiIntegrator.EnhanceTitle(filename, filename, mediaType)
	if err != nil {
		return fmt.Errorf("AI enhancement failed: %w", err)
	}

	fmt.Println("AI Title Enhancement Test")
	fmt.Println("========================")
	fmt.Printf("Filename: %s\n", filename)
	fmt.Printf("Media Type: %s\n\n", mediaType)

	fmt.Printf("Original: %s\n", filename)
	fmt.Printf("Enhanced: %s\n", enhanced)
	fmt.Printf("Source: %s\n", source)

	return nil
}

// filenameContainsEpisodePattern checks if filename contains TV episode patterns
func filenameContainsEpisodePattern(filename string) bool {
	return containsEPattern(filename) || containsXPattern(filename)
}

// containsEPattern checks for S##E## pattern
func containsEPattern(s string) bool {
	for i := 0; i < len(s)-3; i++ {
		if (s[i] == 's' || s[i] == 'S') && i+1 < len(s) {
			if s[i+1] >= '0' && s[i+1] <= '9' {
				j := i + 2
				for j < len(s) && s[j] >= '0' && s[j] <= '9' {
					j++
				}
				if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
					return true
				}
			}
		}
	}
	return false
}

// containsXPattern checks for ##x## pattern
func containsXPattern(s string) bool {
	for i := 0; i < len(s)-2; i++ {
		if s[i] >= '0' && s[i] <= '9' && (s[i+1] == 'x' || s[i+1] == 'X') {
			if i+2 < len(s) && s[i+2] >= '0' && s[i+2] <= '9' {
				return true
			}
		}
	}
	return false
}
