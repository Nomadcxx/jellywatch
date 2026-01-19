package compliance

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Nomadcxx/jellywatch/internal/naming"
)

// Issue represents a single compliance issue found during validation.
type Issue struct {
	Type    string // Issue type constant (e.g., IssueInvalidFilename)
	Message string // Human-readable message describing the issue
}

func (i Issue) String() string {
	return fmt.Sprintf("%s: %s", i.Type, i.Message)
}

// EpisodeValidator validates TV episode files for Jellyfin naming compliance.
// It performs multiple checks and returns all issues found.
type EpisodeValidator struct {
	checker *Checker
}

// NewEpisodeValidator creates a new episode validator.
func NewEpisodeValidator(checker *Checker) *EpisodeValidator {
	return &EpisodeValidator{checker: checker}
}

// Validate performs all compliance checks for an episode file.
// Returns a list of all issues found. Empty list means the file is compliant.
//
// Checks performed:
//   - Title matches folder context (fuzzy matching)
//   - Season folder format and padding
//   - Year present in filename or folder
//   - No release markers in filename
//   - No invalid special characters
//   - Filename matches expected Jellyfin format
func (v *EpisodeValidator) Validate(ctx FolderContext, tv *naming.TVShowInfo, filename string) []Issue {
	var issues []Issue

	if issue := v.validateTitleMatch(tv.Title, ctx); issue != nil {
		issues = append(issues, *issue)
	}

	seasonIssues := v.validateSeasonFolder(ctx.SeasonFolder, tv.Season)
	issues = append(issues, seasonIssues...)

	if issue := v.validateYear(ctx.Year, tv.Year); issue != nil {
		issues = append(issues, *issue)
	}

	if issue := v.validateReleaseMarkers(filename); issue != nil {
		issues = append(issues, *issue)
	}

	if issue := v.validateSpecialCharacters(filename); issue != nil {
		issues = append(issues, *issue)
	}

	if issue := v.validateFilenameFormat(ctx, tv, filename); issue != nil {
		issues = append(issues, *issue)
	}

	return issues
}

// validateTitleMatch checks if parsed title matches folder context
func (v *EpisodeValidator) validateTitleMatch(parsedTitle string, ctx FolderContext) *Issue {
	if !TitleMatchesFolderContext(parsedTitle, ctx) {
		return &Issue{
			Type:    IssueInvalidFolderStructure,
			Message: fmt.Sprintf("filename title '%s' doesn't match show folder '%s'", parsedTitle, ctx.ShowName),
		}
	}
	return nil
}

// validateSeasonFolder checks season folder format and padding
// Returns multiple issues if both are wrong
func (v *EpisodeValidator) validateSeasonFolder(seasonFolder string, season int) []Issue {
	var issues []Issue
	expected := naming.FormatSeasonFolder(season)
	
	// Check if folder name matches expected
	if !strings.EqualFold(seasonFolder, expected) {
		issues = append(issues, Issue{
			Type:    IssueWrongSeasonFolder,
			Message: fmt.Sprintf("expected '%s', found '%s'", expected, seasonFolder),
		})
	}
	
	// Check padding (separate check - can be wrong even if name matches)
	if !isValidSeasonFolder(seasonFolder) {
		issues = append(issues, Issue{
			Type:    IssueInvalidPadding,
			Message: "season folder must be zero-padded (Season 01, not Season 1)",
		})
	}

	return issues
}

// validateYear checks if year is present in either filename or folder
func (v *EpisodeValidator) validateYear(folderYear, filenameYear string) *Issue {
	if folderYear == "" && filenameYear == "" {
		return &Issue{
			Type:    IssueMissingYear,
			Message: "missing year in both filename and folder",
		}
	}
	return nil
}

// validateReleaseMarkers checks for quality/codec markers in filename
func (v *EpisodeValidator) validateReleaseMarkers(filename string) *Issue {
	if hasReleaseMarkers(filename) {
		return &Issue{
			Type:    IssueReleaseMarkers,
			Message: "contains quality/codec markers",
		}
	}
	return nil
}

// validateSpecialCharacters checks for invalid characters in filename
func (v *EpisodeValidator) validateSpecialCharacters(filename string) *Issue {
	if invalidChars := findInvalidCharacters(filename); len(invalidChars) > 0 {
		return &Issue{
			Type:    IssueSpecialCharacters,
			Message: fmt.Sprintf("contains invalid characters: %s", strings.Join(invalidChars, ", ")),
		}
	}
	return nil
}

// validateFilenameFormat checks if filename matches expected Jellyfin format
func (v *EpisodeValidator) validateFilenameFormat(ctx FolderContext, tv *naming.TVShowInfo, filename string) *Issue {
	ext := filepath.Ext(filename)
	if ext != "" {
		ext = ext[1:] // Remove leading dot
	}

	effectiveYear := tv.Year
	if effectiveYear == "" && ctx.Year != "" {
		effectiveYear = ctx.Year
	}

	expectedFilename := naming.FormatTVEpisodeFilename(ctx.ShowName, effectiveYear, tv.Season, tv.Episode, ext)
	if filename != expectedFilename {
		return &Issue{
			Type:    IssueInvalidFilename,
			Message: fmt.Sprintf("expected '%s'", expectedFilename),
		}
	}
	return nil
}
