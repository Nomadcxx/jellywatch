package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/database"
	"github.com/Nomadcxx/jellywatch/internal/ui"
	"github.com/spf13/cobra"
)

func NewSetupCmd() *cobra.Command {
	var (
		nonInteractive bool
		configPath     string
	)

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive setup wizard",
		Long: `Run the interactive setup wizard to configure JellyWatch.

This wizard will:
  - Detect Jellyfin library paths
  - Configure Sonarr/Radarr integration
  - Set up AI enhancement (optional)
  - Initialize the database
  - Create configuration file

Examples:
  jellywatch setup                    # Interactive wizard
  jellywatch setup --non-interactive  # Use defaults only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunSetup(nonInteractive, configPath)
		},
	}

	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Use defaults, skip prompts")
	cmd.Flags().StringVar(&configPath, "config", "", "Config file path (default: ~/.config/jellywatch/config.toml)")

	return cmd
}

func RunSetup(nonInteractive bool, configPath string) error {
	ui.Section("JellyWatch Setup Wizard")

	// Check if config already exists
	cfgPath := configPath
	if cfgPath == "" {
		var err error
		cfgPath, err = config.ConfigPath()
		if err != nil {
			return fmt.Errorf("failed to get config path: %w", err)
		}
	}

	if _, err := os.Stat(cfgPath); err == nil {
		if !nonInteractive {
			fmt.Printf("\n⚠️  Configuration file already exists: %s\n", cfgPath)
			if !confirm("Do you want to overwrite it?", false) {
				fmt.Println("Setup cancelled.")
				return nil
			}
		}
	}

	cfg := &config.Config{}

	// Step 1: Library paths
	ui.Subsection("Library Paths")
	fmt.Println("Enter your Jellyfin library paths.")
	fmt.Println("Press Enter after each path, empty line to finish.")
	fmt.Println()

	if nonInteractive {
		// Try to detect common paths
		cfg.Libraries.TV = detectTVLibraries()
		cfg.Libraries.Movies = detectMovieLibraries()
		fmt.Printf("Auto-detected TV libraries: %v\n", cfg.Libraries.TV)
		fmt.Printf("Auto-detected Movie libraries: %v\n", cfg.Libraries.Movies)
	} else {
		cfg.Libraries.TV = promptPaths("TV library paths")
		cfg.Libraries.Movies = promptPaths("Movie library paths")
	}

	if len(cfg.Libraries.TV) == 0 && len(cfg.Libraries.Movies) == 0 {
		return fmt.Errorf("at least one library path is required")
	}

	// Step 2: Watch directories
	ui.Subsection("Watch Directories")
	fmt.Println("Enter directories to watch for new media files.")

	if nonInteractive {
		cfg.Watch.TV = detectWatchDirectories()
		cfg.Watch.Movies = detectWatchDirectories()
		fmt.Printf("Auto-detected watch directories: %v\n", cfg.Watch.TV)
	} else {
		cfg.Watch.TV = promptPaths("TV watch directories")
		cfg.Watch.Movies = promptPaths("Movie watch directories")
	}

	// Step 3: Sonarr configuration
	ui.Subsection("Sonarr Integration")
	if nonInteractive || !confirm("Configure Sonarr integration?", false) {
		cfg.Sonarr.Enabled = false
	} else {
		cfg.Sonarr.Enabled = true
		cfg.Sonarr.URL = promptString("Sonarr URL", "http://localhost:8989")
		cfg.Sonarr.APIKey = promptString("Sonarr API Key", "")
		if cfg.Sonarr.APIKey == "" {
			fmt.Println("⚠️  Warning: No API key provided. Sonarr integration will be disabled.")
			cfg.Sonarr.Enabled = false
		}
	}

	// Step 4: Radarr configuration
	ui.Subsection("Radarr Integration")
	if nonInteractive || !confirm("Configure Radarr integration?", false) {
		cfg.Radarr.Enabled = false
	} else {
		cfg.Radarr.Enabled = true
		cfg.Radarr.URL = promptString("Radarr URL", "http://localhost:7878")
		cfg.Radarr.APIKey = promptString("Radarr API Key", "")
		if cfg.Radarr.APIKey == "" {
			fmt.Println("⚠️  Warning: No API key provided. Radarr integration will be disabled.")
			cfg.Radarr.Enabled = false
		}
	}

	// Step 5: AI configuration
	ui.Subsection("AI Enhancement")
	if nonInteractive || !confirm("Enable AI title enhancement?", false) {
		cfg.AI.Enabled = false
	} else {
		cfg.AI.Enabled = true
		cfg.AI.OllamaEndpoint = promptString("Ollama endpoint", "http://localhost:11434")
		cfg.AI.Model = promptString("Model name", "qwen2.5vl:7b")
		timeoutStr := promptString("Timeout (seconds)", "5")
		var timeout int
		fmt.Sscanf(timeoutStr, "%d", &timeout)
		cfg.AI.TimeoutSeconds = timeout
		cfg.AI.ConfidenceThreshold = promptFloat("Confidence threshold (0.0-1.0)", 0.8)
		cfg.AI.CacheEnabled = confirm("Enable AI cache?", true)
	}

	// Step 6: Permissions
	ui.Subsection("File Permissions")
	if !nonInteractive && confirm("Configure file permissions?", false) {
		cfg.Permissions.User = promptString("File owner (username or UID, empty to skip)", "")
		cfg.Permissions.Group = promptString("File group (groupname or GID, empty to skip)", "")
		cfg.Permissions.FileMode = promptString("File mode (octal, e.g., 0644, empty to skip)", "")
		cfg.Permissions.DirMode = promptString("Directory mode (octal, e.g., 0755, empty to skip)", "")
	}

	// Step 7: Save configuration
	ui.Section("Saving Configuration")
	if cfgPath != "" {
		// If custom path provided, we need to save manually
		configDir := filepath.Dir(cfgPath)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
		if err := os.WriteFile(cfgPath, []byte(cfg.ToTOML()), 0644); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}
		ui.SuccessMsg("Configuration saved to: %s", cfgPath)
	} else {
		// Use default Save method
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}
		path, _ := config.ConfigPath()
		ui.SuccessMsg("Configuration saved to: %s", path)
	}

	// Step 8: Initialize database
	ui.Section("Initializing Database")
	dbPath := config.GetDatabasePath()
	db, err := database.OpenPath(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	ui.SuccessMsg("Database initialized: %s", dbPath)

	// Step 9: Summary
	ui.Section("Setup Complete")
	fmt.Println("\nConfiguration Summary:")
	fmt.Printf("  TV Libraries:     %d\n", len(cfg.Libraries.TV))
	fmt.Printf("  Movie Libraries:  %d\n", len(cfg.Libraries.Movies))
	fmt.Printf("  TV Watch Dirs:    %d\n", len(cfg.Watch.TV))
	fmt.Printf("  Movie Watch Dirs: %d\n", len(cfg.Watch.Movies))
	fmt.Printf("  Sonarr:           %s\n", boolString(cfg.Sonarr.Enabled))
	fmt.Printf("  Radarr:           %s\n", boolString(cfg.Radarr.Enabled))
	fmt.Printf("  AI Enhancement:   %s\n", boolString(cfg.AI.Enabled))

	fmt.Println("\nNext steps:")
	fmt.Println("  1. Run 'jellywatch library scan' to populate the database")
	fmt.Println("  2. Run 'jellywatch watch <dir>' to start monitoring")
	fmt.Println("  3. Run 'jellywatch add <path>' to organize media")

	return nil
}

// Helper functions

func promptString(prompt, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" && defaultValue != "" {
		return defaultValue
	}
	return input
}

func promptInt(prompt string, defaultValue int) int {
	fmt.Printf("%s [%d]: ", prompt, defaultValue)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultValue
	}

	var result int
	if _, err := fmt.Sscanf(input, "%d", &result); err != nil {
		return defaultValue
	}
	return result
}

func promptFloat(prompt string, defaultValue float64) float64 {
	fmt.Printf("%s [%.2f]: ", prompt, defaultValue)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultValue
	}

	var result float64
	if _, err := fmt.Sscanf(input, "%f", &result); err != nil {
		return defaultValue
	}
	return result
}


func promptPaths(prompt string) []string {
	fmt.Printf("%s (one per line, empty line to finish):\n", prompt)
	var paths []string
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("  > ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if line == "" {
			break
		}

		// Validate path exists
		if info, err := os.Stat(line); err == nil && info.IsDir() {
			paths = append(paths, line)
		} else {
			fmt.Printf("    ⚠️  Path does not exist or is not a directory: %s\n", line)
		}
	}

	return paths
}

func confirm(prompt string, defaultValue bool) bool {
	defaultStr := "y/N"
	if defaultValue {
		defaultStr = "Y/n"
	}
	fmt.Printf("%s [%s]: ", prompt, defaultStr)

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))

	if input == "" {
		return defaultValue
	}
	return input == "y" || input == "yes"
}

func boolString(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}

func detectTVLibraries() []string {
	commonPaths := []string{
		"/media/TV",
		"/mnt/TV",
		"/storage/TV",
		"/home/*/Videos/TV",
		"/home/*/Media/TV",
	}
	return detectPaths(commonPaths)
}

func detectMovieLibraries() []string {
	commonPaths := []string{
		"/media/Movies",
		"/mnt/Movies",
		"/storage/Movies",
		"/home/*/Videos/Movies",
		"/home/*/Media/Movies",
	}
	return detectPaths(commonPaths)
}

func detectWatchDirectories() []string {
	commonPaths := []string{
		"/downloads",
		"/home/*/Downloads",
		"/tmp/downloads",
	}
	return detectPaths(commonPaths)
}

func detectPaths(patterns []string) []string {
	var found []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err == nil {
			for _, match := range matches {
				if info, err := os.Stat(match); err == nil && info.IsDir() {
					found = append(found, match)
				}
			}
		}
	}
	return found
}
