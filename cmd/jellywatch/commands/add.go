package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/spf13/cobra"
)

func isDirectory(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

func NewAddCmd() *cobra.Command {
	var (
		libraryPath    string
		keepSource     bool
		forceOverwrite bool
		dryRun         bool
		recursive      bool
		timeout        time.Duration
		verifyChecksum bool
		backendName    string
		useAI          bool
		noAI           bool
		keepExtras     bool
		verbose        bool
	)

	cmd := &cobra.Command{
		Use:   "add <path> [target-library]",
		Short: "Add media to library (auto-detects files and folders)",
		Long: `Intelligently add media files or folders to your library.

Automatically detects whether you're adding:
  - A single file (movie or episode)
  - A folder with media files
  - A season pack folder

Examples:
  jellywatch add /downloads/Silo.S02E02.mkv
  jellywatch add /downloads/The.Matrix.1999.1080p/
  jellywatch add /downloads/folder/ /media/TV`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]

			target := libraryPath
			if len(args) > 1 {
				target = args[1]
			}

			if target == "" {
				cfg, err := config.Load()
				if err == nil {
					// Default to first available library
					if len(cfg.Libraries.TV) > 0 {
						target = cfg.Libraries.TV[0]
					} else if len(cfg.Libraries.Movies) > 0 {
						target = cfg.Libraries.Movies[0]
					}
				}
				if target == "" {
					return fmt.Errorf("no target library specified (use --library or config file)")
				}
			}

			// Auto-detect file vs directory
			isDir, err := isDirectory(source)
			if err != nil {
				return fmt.Errorf("cannot access source: %w", err)
			}

			// Create organizer
			org, err := CreateOrganizer(target, dryRun, keepSource, forceOverwrite, timeout, verifyChecksum, backendName, useAI, noAI)
			if err != nil {
				return err
			}
			defer org.Close()

			// Delegate to appropriate handler
			if isDir {
				// Check if it looks like a download folder (has samples, junk, etc.)
				// For now, try OrganizeFolder first, fall back to OrganizeDirectory
				result, err := OrganizeFolder(org.Organizer, source, target, keepExtras, verbose)
				if err != nil {
					// Fall back to simple directory organization
					return OrganizeDirectory(org.Organizer, source, target, recursive, dryRun, keepSource, verbose)
				}
				if result.Error != nil {
					return result.Error
				}
				return nil
			}
			_, err = OrganizeFile(org.Organizer, source, target, dryRun, keepSource, verbose)
			return err
		},
	}

	cmd.Flags().StringVarP(&libraryPath, "library", "l", "", "target library path")
	cmd.Flags().BoolVarP(&keepSource, "keep", "k", false, "copy instead of move")
	cmd.Flags().BoolVarP(&forceOverwrite, "force", "f", false, "overwrite existing files")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "preview changes")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", true, "process subdirectories")
	cmd.Flags().DurationVarP(&timeout, "timeout", "t", 5*time.Minute, "transfer timeout")
	cmd.Flags().BoolVar(&verifyChecksum, "checksum", false, "verify checksum after transfer")
	cmd.Flags().StringVarP(&backendName, "backend", "b", "auto", "transfer backend: auto, pv, rsync, native")
	cmd.Flags().BoolVar(&useAI, "ai", false, "enable AI title enhancement")
	cmd.Flags().BoolVar(&noAI, "no-ai", false, "disable AI title enhancement")
	cmd.Flags().BoolVar(&keepExtras, "keep-extras", false, "preserve extra files (trailers, featurettes)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	return cmd
}
