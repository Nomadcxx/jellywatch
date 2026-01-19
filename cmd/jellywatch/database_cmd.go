package main

import (
	"fmt"
	"os"

	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/database"
	"github.com/spf13/cobra"
)

func newDatabaseCmdDeprecated() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "database",
		Aliases: []string{"db"},
		Short:   "[DEPRECATED] Use 'jellywatch tools database' instead",
		Long:    `DEPRECATED: This command is deprecated. Use 'jellywatch tools database' instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "⚠️  Warning: 'jellywatch database' is deprecated. Use 'jellywatch tools database' instead.")
			cmd.Help()
			return nil
		},
	}
	cmd.Hidden = true
	return cmd
}

func newDatabaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "database",
		Aliases: []string{"db"},
		Short:   "Database management commands",
		Long:    `Commands for managing the HOLDEN database (media.db)`,
	}

	cmd.AddCommand(newDatabaseInitCmd())
	cmd.AddCommand(newDatabaseResetCmd())
	cmd.AddCommand(newDatabasePathCmd())

	return cmd
}

func newDatabaseInitCmd() *cobra.Command {
	var scan bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a fresh database",
		Long: `Initialize a fresh HOLDEN database.

If a database already exists, this command will fail unless used with 'database reset'.
Use --scan to immediately populate the database after initialization.

Examples:
  jellywatch database init              # Create empty database
  jellywatch database init --scan       # Create database and scan libraries`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDatabaseInit(scan, false)
		},
	}

	cmd.Flags().BoolVar(&scan, "scan", false, "Scan libraries after initialization")

	return cmd
}

func newDatabaseResetCmd() *cobra.Command {
	var scan bool
	var force bool

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Delete and reinitialize the database",
		Long: `Delete the existing database and create a fresh one.

WARNING: This will delete all data including:
- All learned media paths
- Duplicate detection results
- AI parse cache
- Sync history

Use --scan to immediately repopulate the database after reset.

Examples:
  jellywatch database reset              # Reset with confirmation
  jellywatch database reset --force      # Reset without confirmation
  jellywatch database reset --scan       # Reset and scan libraries`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDatabaseReset(scan, force)
		},
	}

	cmd.Flags().BoolVar(&scan, "scan", false, "Scan libraries after reset")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}

func newDatabasePathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Show database file path",
		Long:  `Display the path to the HOLDEN database file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := config.GetDatabasePath()
			fmt.Println(dbPath)

			// Check if it exists
			if _, err := os.Stat(dbPath); err == nil {
				info, _ := os.Stat(dbPath)
				size := info.Size()
				if size < 1024 {
					fmt.Printf("Size: %d bytes\n", size)
				} else if size < 1024*1024 {
					fmt.Printf("Size: %.1f KB\n", float64(size)/1024)
				} else {
					fmt.Printf("Size: %.1f MB\n", float64(size)/(1024*1024))
				}
			} else {
				fmt.Println("Status: Not initialized")
			}

			return nil
		},
	}

	return cmd
}

func runDatabaseInit(scan bool, allowOverwrite bool) error {
	dbPath := config.GetDatabasePath()

	// Check if database already exists
	if _, err := os.Stat(dbPath); err == nil && !allowOverwrite {
		return fmt.Errorf("database already exists at %s\nUse 'jellywatch database reset' to reinitialize", dbPath)
	}

	fmt.Printf("Initializing database at %s\n", dbPath)

	// Create/open database (this will run migrations)
	db, err := database.OpenPath(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	db.Close()

	fmt.Println("✓ Database initialized successfully")

	// Run scan if requested
	if scan {
		fmt.Println()
		return runScan(false, false, true, true)
	}

	fmt.Println("\nRun 'jellywatch scan' to populate the database")

	return nil
}

func runDatabaseReset(scan bool, force bool) error {
	dbPath := config.GetDatabasePath()

	// Check if database exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Printf("No database found at %s\n", dbPath)
		fmt.Println("Creating fresh database...")
		return runDatabaseInit(scan, true)
	}

	// Confirm deletion unless --force
	if !force {
		fmt.Printf("This will DELETE the database at:\n  %s\n\n", dbPath)
		fmt.Println("All data will be lost including:")
		fmt.Println("  - Learned media paths")
		fmt.Println("  - Duplicate detection results")
		fmt.Println("  - AI parse cache")
		fmt.Println("  - Sync history")
		fmt.Print("\nAre you sure? (y/N): ")

		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" && response != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Delete the database
	fmt.Printf("Deleting database...\n")
	if err := os.Remove(dbPath); err != nil {
		return fmt.Errorf("failed to delete database: %w", err)
	}

	fmt.Println("✓ Database deleted")
	fmt.Println()

	// Initialize fresh database
	return runDatabaseInit(scan, true)
}
