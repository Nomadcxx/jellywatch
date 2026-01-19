package main

import (
	"fmt"
	"strings"

	"github.com/Nomadcxx/jellywatch/internal/database"
	"github.com/Nomadcxx/jellywatch/internal/ui"
	"github.com/spf13/cobra"
)

func newDuplicatesCmd() *cobra.Command {
	var (
		moviesOnly bool
		tvOnly     bool
		showFilter string
	)

	cmd := &cobra.Command{
		Use:   "duplicates [flags]",
		Short: "List duplicate media files",
		Long: `List all duplicate media files found in the database.

Duplicates are files with the same normalized title, year, and episode (for TV shows)
but different quality scores. The CONDOR system identifies which file should be kept
based on quality scoring (Resolution > Source > Size).

Examples:
  jellywatch duplicates              # List all duplicates
  jellywatch duplicates --movies     # Only movies
  jellywatch duplicates --tv         # Only TV episodes
  jellywatch duplicates --show=Silo  # Duplicates for specific show
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDuplicates(moviesOnly, tvOnly, showFilter)
		},
	}

	cmd.Flags().BoolVar(&moviesOnly, "movies", false, "Show only movie duplicates")
	cmd.Flags().BoolVar(&tvOnly, "tv", false, "Show only TV episode duplicates")
	cmd.Flags().StringVar(&showFilter, "show", "", "Filter by show name")

	return cmd
}

func runDuplicates(moviesOnly, tvOnly bool, showFilter string) error {
	// Open database
	db, err := database.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	totalGroups := 0
	totalFiles := 0
	totalReclaimable := int64(0)

	// Show movie duplicates
	if !tvOnly {
		movieGroups, err := db.FindDuplicateMovies()
		if err != nil {
			return fmt.Errorf("failed to find duplicate movies: %w", err)
		}

		if len(movieGroups) > 0 {
			ui.Section("Duplicate Movies")

			for _, group := range movieGroups {
				if len(group.Files) < 2 {
					continue
				}

				totalGroups++
				totalFiles += len(group.Files)

				yearStr := ""
				if group.Year != nil {
					yearStr = fmt.Sprintf(" (%d)", *group.Year)
				}

				fmt.Printf("\n%s%s\n", group.NormalizedTitle, yearStr)
				table := ui.NewTable("Quality", "Size", "Resolution", "Source", "Path", "Action")
				for _, file := range group.Files {
					action := ui.Error("DELETE")
					if group.BestFile != nil && file.ID == group.BestFile.ID {
						action = ui.Success("KEEP")
					} else {
						totalReclaimable += file.Size
					}
					table.AddRow(
						fmt.Sprintf("%d", file.QualityScore),
						ui.FormatBytes(file.Size),
						file.Resolution,
						file.SourceType,
						file.Path,
						action,
					)
				}
				table.Render()
				fmt.Printf("\nSpace reclaimable: %s\n\n", ui.FormatBytes(group.SpaceReclaimable))
			}
		}
	}

	// Show TV duplicates
	if !moviesOnly {
		episodeGroups, err := db.FindDuplicateEpisodes()
		if err != nil {
			return fmt.Errorf("failed to find duplicate episodes: %w", err)
		}

		if len(episodeGroups) > 0 {
			ui.Section("Duplicate TV Episodes")

			for _, group := range episodeGroups {
				if len(group.Files) < 2 {
					continue
				}

				// Apply show filter if specified
				if showFilter != "" && !strings.Contains(strings.ToLower(group.NormalizedTitle), strings.ToLower(showFilter)) {
					continue
				}

				totalGroups++
				totalFiles += len(group.Files)

				yearStr := ""
				if group.Year != nil {
					yearStr = fmt.Sprintf(" (%d)", *group.Year)
				}

				episodeStr := ""
				if group.Season != nil && group.Episode != nil {
					episodeStr = fmt.Sprintf(" S%02dE%02d", *group.Season, *group.Episode)
				}

				fmt.Printf("\n%s%s%s\n", group.NormalizedTitle, yearStr, episodeStr)
				table := ui.NewTable("Quality", "Size", "Resolution", "Source", "Path", "Action")
				for _, file := range group.Files {
					action := ui.Error("DELETE")
					if group.BestFile != nil && file.ID == group.BestFile.ID {
						action = ui.Success("KEEP")
					} else {
						totalReclaimable += file.Size
					}
					table.AddRow(
						fmt.Sprintf("%d", file.QualityScore),
						ui.FormatBytes(file.Size),
						file.Resolution,
						file.SourceType,
						file.Path,
						action,
					)
				}
				table.Render()
				fmt.Printf("\nSpace reclaimable: %s\n\n", ui.FormatBytes(group.SpaceReclaimable))
			}
		}
	}

	// Summary
	if totalGroups == 0 {
		ui.SuccessMsg("No duplicates found!")
	} else {
		ui.Section("Summary")
		summaryRows := [][]string{
			{"Duplicate groups", fmt.Sprintf("%d", totalGroups)},
			{"Total files", fmt.Sprintf("%d", totalFiles)},
			{"Space reclaimable", ui.FormatBytes(totalReclaimable)},
		}
		ui.CompactTable([]string{"Metric", "Value"}, summaryRows)
		fmt.Println()
		ui.InfoMsg("To remove duplicates:")
		fmt.Println("  jellywatch consolidate --generate  # Generate cleanup plans")
		fmt.Println("  jellywatch consolidate --dry-run   # Preview actions")
		fmt.Println("  jellywatch consolidate --execute   # Execute cleanup")
	}

	return nil
}
