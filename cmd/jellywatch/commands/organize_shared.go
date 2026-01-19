package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Nomadcxx/jellywatch/internal/ai"
	"github.com/Nomadcxx/jellywatch/internal/app"
	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/database"
	"github.com/Nomadcxx/jellywatch/internal/naming"
	"github.com/Nomadcxx/jellywatch/internal/organizer"
	"github.com/Nomadcxx/jellywatch/internal/transfer"
	"github.com/Nomadcxx/jellywatch/internal/ui"
)

// CloseableOrganizer wraps an organizer with resources that need cleanup
type CloseableOrganizer struct {
	*organizer.Organizer
	db           *database.MediaDB
	aiIntegrator *ai.Integrator
}

// Close releases resources held by the organizer
func (c *CloseableOrganizer) Close() {
	if c.aiIntegrator != nil {
		c.aiIntegrator.Close()
	}
	if c.db != nil {
		c.db.Close()
	}
}

// CreateOrganizer creates an organizer with optional AI integration.
// Returns a CloseableOrganizer that must be Closed by the caller.
func CreateOrganizer(target string, dryRun, keepSource, forceOverwrite bool, timeout time.Duration, verifyChecksum bool, backendName string, useAI, noAI bool) (*CloseableOrganizer, error) {
	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{} // Use defaults if config not found
	}

	opts := []func(*organizer.Organizer){
		organizer.WithDryRun(dryRun),
		organizer.WithKeepSource(keepSource),
		organizer.WithForceOverwrite(forceOverwrite),
		organizer.WithTimeout(timeout),
		organizer.WithChecksumVerify(verifyChecksum),
		organizer.WithBackend(transfer.ParseBackend(backendName)),
	}

	var db *database.MediaDB
	var aiIntegrator *ai.Integrator

	// Add AI integrator if enabled
	if useAI || (cfg.AI.Enabled && !noAI) {
		db, err = database.OpenPath(config.GetDatabasePath())
		if err != nil {
			return nil, fmt.Errorf("failed to open database: %w", err)
		}

		aiIntegrator, err = app.InitAIWithOverride(cfg, db, useAI, nil)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to initialize AI: %w", err)
		}
		if aiIntegrator != nil {
			opts = append(opts, organizer.WithAIIntegrator(aiIntegrator))
		}
	}

	return &CloseableOrganizer{
		Organizer:    organizer.NewOrganizer([]string{target}, opts...),
		db:           db,
		aiIntegrator: aiIntegrator,
	}, nil
}

// OrganizeFile processes a single media file
func OrganizeFile(org *organizer.Organizer, source, target string, dryRun, keepSource, verbose bool) (*organizer.OrganizationResult, error) {
	filename := filepath.Base(source)

	if naming.IsTVEpisodeFilename(filename) {
		result, err := org.OrganizeTVEpisode(source, target)
		if err != nil {
			return nil, err
		}
		printResult(result, dryRun, keepSource, verbose)
		return result, nil
	}

	if naming.IsMovieFilename(filename) {
		result, err := org.OrganizeMovie(source, target)
		if err != nil {
			return nil, err
		}
		printResult(result, dryRun, keepSource, verbose)
		return result, nil
	}

	return nil, fmt.Errorf("cannot determine media type for: %s", filename)
}

// OrganizeDirectory processes all media files in a directory
func OrganizeDirectory(org *organizer.Organizer, source, target string, recursive, dryRun, keepSource, verbose bool) error {
	var processed, succeeded, failed, skipped int

	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if !recursive && path != source {
				return filepath.SkipDir
			}
			return nil
		}

		if !isMediaFile(path) {
			return nil
		}

		processed++
		result, err := OrganizeFile(org, path, target, dryRun, keepSource, verbose)
		if err != nil {
			failed++
			ui.ErrorMsg("%s: %v", filepath.Base(path), err)
		} else if result.Skipped {
			skipped++
		} else if result.Success {
			succeeded++
		} else {
			failed++
		}

		return nil
	})

	fmt.Printf("\nSummary: %d processed, %d succeeded, %d skipped, %d failed\n", processed, succeeded, skipped, failed)
	return err
}

// OrganizeFolder intelligently organizes a download folder
func OrganizeFolder(org *organizer.Organizer, source, target string, keepExtras, verbose bool) (*organizer.FolderOrganizationResult, error) {
	result, err := org.OrganizeFolder(source, target, keepExtras)
	if err != nil {
		return nil, err
	}
	printFolderResult(result, keepExtras, verbose)
	return result, nil
}

// Helper functions
func isMediaFile(path string) bool {
	ext := filepath.Ext(path)
	mediaExts := []string{".mkv", ".mp4", ".avi", ".mov", ".m4v", ".mpg", ".mpeg", ".ts", ".m2ts"}
	for _, me := range mediaExts {
		if strings.EqualFold(ext, me) {
			return true
		}
	}
	return false
}

func printResult(result *organizer.OrganizationResult, dryRun, keepSource bool, verbose bool) {
	if result.Skipped {
		fmt.Printf("⊘ skipped %s\n", filepath.Base(result.SourcePath))
		if verbose {
			fmt.Printf("  Reason: %s\n", result.SkipReason)
		}
		return
	}

	if result.Success {
		action := "moved"
		if keepSource {
			action = "copied"
		}
		if dryRun {
			action = "would " + action[:len(action)-1]
		}
		ui.SuccessMsg("%s %s", action, filepath.Base(result.SourcePath))
		if verbose {
			fmt.Printf("  → %s\n", result.TargetPath)
			if result.BytesCopied > 0 {
				fmt.Printf("  %s in %s\n", ui.FormatBytes(result.BytesCopied), result.Duration)
			}
			if result.SourceQuality != nil {
				fmt.Printf("  Quality: %s\n", result.SourceQuality.String())
			}
		}
	} else {
		ui.ErrorMsg("%s: %v", filepath.Base(result.SourcePath), result.Error)
	}
}

func printFolderResult(result *organizer.FolderOrganizationResult, keepExtras, verbose bool) {
	if result.Analysis != nil {
		fmt.Printf("📁 Analyzed: %s\n", result.Analysis.Path)
		fmt.Printf("   Type: %s\n", result.Analysis.MediaType.String())
		if result.Analysis.MainMediaFile != nil {
			fmt.Printf("   Main: %s\n", result.Analysis.MainMediaFile.Name)
		}
		fmt.Printf("   Files: %d media, %d samples, %d junk, %d subtitles\n",
			len(result.Analysis.MediaFiles),
			len(result.Analysis.SampleFiles),
			len(result.Analysis.JunkFiles),
			len(result.Analysis.SubtitleFiles))
	}

	if result.MediaResult != nil {
		printResult(result.MediaResult, false, false, verbose) // dryRun/keepSource passed from command
	}

	if len(result.SubtitlesCopied) > 0 {
		fmt.Printf("📝 Subtitles copied: %d\n", len(result.SubtitlesCopied))
		if verbose {
			for _, s := range result.SubtitlesCopied {
				fmt.Printf("   - %s\n", s)
			}
		}
	}

	if len(result.JunkRemoved) > 0 {
		fmt.Printf("🗑  Junk removed: %d\n", len(result.JunkRemoved))
		if verbose {
			for _, j := range result.JunkRemoved {
				fmt.Printf("   - %s\n", j)
			}
		}
	}

	if len(result.SamplesRemoved) > 0 {
		fmt.Printf("🗑  Samples removed: %d\n", len(result.SamplesRemoved))
	}

	if len(result.ExtrasSkipped) > 0 && !keepExtras {
		fmt.Printf("⏭  Extras skipped: %d\n", len(result.ExtrasSkipped))
		if verbose {
			for _, e := range result.ExtrasSkipped {
				fmt.Printf("   - %s\n", e)
			}
		}
	}

	if result.Error != nil {
		ui.ErrorMsg("%v", result.Error)
	}
}
