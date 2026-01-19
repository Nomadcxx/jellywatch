package compliance

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Nomadcxx/jellywatch/internal/logging"
	"github.com/Nomadcxx/jellywatch/internal/naming"
)

// ComplianceResult represents the compliance check result for database storage
type ComplianceResult struct {
	IsCompliant bool
	Issues      []string
}

// IssueType categorizes compliance issues
const (
	IssueInvalidFilename        = "invalid_filename"
	IssueReleaseMarkers         = "release_markers"
	IssueMissingYear            = "missing_year"
	IssueInvalidYearFormat      = "invalid_year_format"
	IssueInvalidFolderStructure = "invalid_folder_structure"
	IssueWrongSeasonFolder      = "wrong_season_folder"
	IssueSpecialCharacters      = "special_characters"
	IssueInvalidPadding         = "invalid_padding"
	IssueObfuscated             = "obfuscated_filename"
)

// Checker validates media files against Jellyfin naming conventions
type Checker struct {
	libraryRoot string
	logger      *logging.Logger
}

// NewChecker creates a new compliance checker
func NewChecker(libraryRoot string) *Checker {
	return &Checker{
		libraryRoot: libraryRoot,
		logger:      logging.Nop(),
	}
}

func (c *Checker) WithLogger(logger *logging.Logger) *Checker {
	c.logger = logger
	return c
}

func NewCheckerWithLogger(libraryRoot string, logger *logging.Logger) *Checker {
	return &Checker{
		libraryRoot: libraryRoot,
		logger:      logger,
	}
}

// CheckMovie validates a movie file and its path structure
//
// Expected structure:
//   - Movies/Movie Name (YYYY)/Movie Name (YYYY).ext
//
// Checks:
//  1. File is in a movie folder with year
//  2. Filename matches folder name
//  3. Year is in parentheses format (YYYY)
//  4. No release markers (1080p, BluRay, x264, etc)
//  5. No special characters that break Jellyfin
func (c *Checker) CheckMovie(fullPath string) ComplianceResult {
	result := ComplianceResult{
		IsCompliant: true,
		Issues:      []string{},
	}

	filename := filepath.Base(fullPath)
	parentDir := filepath.Base(filepath.Dir(fullPath))
	ext := filepath.Ext(filename)

	// Parse movie from filename
	movie, err := naming.ParseMovieName(filename)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("%s: %v", IssueInvalidFilename, err))
		result.IsCompliant = false
		return result
	}

	// Check year format (must be in parentheses)
	if movie.Year == "" {
		result.Issues = append(result.Issues, fmt.Sprintf("%s: missing year", IssueMissingYear))
	} else if !naming.HasYearInParentheses(filename) {
		result.Issues = append(result.Issues, fmt.Sprintf("%s: year must be in format (YYYY)", IssueInvalidYearFormat))
	}

	// Check for release markers
	if hasReleaseMarkers(filename) {
		result.Issues = append(result.Issues, fmt.Sprintf("%s: contains quality/codec markers", IssueReleaseMarkers))
	}

	// Check special characters
	if invalidChars := findInvalidCharacters(filename); len(invalidChars) > 0 {
		result.Issues = append(result.Issues, fmt.Sprintf("%s: contains invalid characters: %s", IssueSpecialCharacters, strings.Join(invalidChars, ", ")))
	}

	// Validate expected filename
	expectedFilename := naming.FormatMovieFilename(movie.Title, movie.Year, ext[1:])
	if filename != expectedFilename {
		result.Issues = append(result.Issues, fmt.Sprintf("%s: expected '%s'", IssueInvalidFilename, expectedFilename))
	}

	// Validate expected folder name
	expectedFolder := naming.NormalizeMovieName(movie.Title, movie.Year)
	if parentDir != expectedFolder {
		result.Issues = append(result.Issues, fmt.Sprintf("%s: folder should be '%s'", IssueInvalidFolderStructure, expectedFolder))
	}

	// Also check if folder name differs from title (catches missing year cases)
	if movie.Year == "" && parentDir != movie.Title {
		result.Issues = append(result.Issues, fmt.Sprintf("%s: folder name doesn't match title", IssueInvalidFolderStructure))
	}

	result.IsCompliant = len(result.Issues) == 0
	return result
}

// CheckEpisode validates a TV episode file and its path structure
//
// Expected structure:
//   - TV Shows/Show Name (Year)/Season XX/Show Name (Year) SXXEXX.ext
//
// Checks:
//  1. File is in proper Season folder
//  2. Season number is zero-padded (Season 01, not Season 1)
//  3. Filename contains SXXEXX format with zero-padding
//  4. No release markers
//  5. Year in parentheses
//  6. No special characters
func (c *Checker) CheckEpisode(fullPath string) ComplianceResult {
	logger := c.logger

	result := ComplianceResult{
		IsCompliant: true,
		Issues:      []string{},
	}

	filename := filepath.Base(fullPath)

	if naming.IsObfuscatedFilename(filename) {
		logger.Debug("compliance", "Obfuscated filename detected")
		result.Issues = append(result.Issues, fmt.Sprintf("%s: obfuscated filename requires AI review", IssueObfuscated))
		result.IsCompliant = false
		return result
	}

	ctx, err := ExtractFolderContext(fullPath)
	if err != nil {
		logger.Warn("compliance", "Failed to extract folder context", logging.F("error", err))
		result.Issues = append(result.Issues, fmt.Sprintf("%s: %v", IssueInvalidFolderStructure, err))
		result.IsCompliant = false
		return result
	}

	logger.Debug("compliance", "Checking episode compliance",
		logging.F("show", ctx.ShowName),
		logging.F("year", ctx.Year),
		logging.F("season_folder", ctx.SeasonFolder),
	)

	tv, err := naming.ParseTVShowName(filename)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("%s: %v", IssueInvalidFilename, err))
		result.IsCompliant = false
		return result
	}

	// Use validator for all compliance checks
	validator := NewEpisodeValidator(c)
	issues := validator.Validate(ctx, tv, filename)

	// Convert issues to strings
	for _, issue := range issues {
		result.Issues = append(result.Issues, issue.String())
	}

	if len(result.Issues) > 0 {
		logger.Info("compliance", "Compliance check found issues",
			logging.F("issue_count", len(result.Issues)),
			logging.F("issues", result.Issues),
		)
	}

	result.IsCompliant = len(result.Issues) == 0
	return result
}

// CheckFile determines media type and runs appropriate validation
func (c *Checker) CheckFile(fullPath string) ComplianceResult {
	filename := filepath.Base(fullPath)

	if naming.IsTVEpisodeFilename(filename) {
		return c.CheckEpisode(fullPath)
	}

	if naming.IsMovieFilename(filename) {
		return c.CheckMovie(fullPath)
	}

	// Unknown media type
	return ComplianceResult{
		IsCompliant: false,
		Issues:      []string{fmt.Sprintf("%s: unable to determine media type", IssueInvalidFilename)},
	}
}

// hasReleaseMarkers checks if filename contains quality/release markers
func hasReleaseMarkers(filename string) bool {
	upper := strings.ToUpper(filename)

	markers := []string{
		"2160P", "1080P", "720P", "480P", "4K", "UHD", "8K",
		"BLURAY", "BLU-RAY", "BDRIP", "BRRIP", "BD-RIP",
		"WEB-DL", "WEBDL", "WEBRIP", "WEB-RIP",
		"HDTV", "DVDRIP", "DVD-RIP", "DVDSCR",
		"X264", "X265", "H264", "H265", "H.264", "H.265",
		"HEVC", "AVC", "AV1", "XVID",
		"AAC", "AC3", "DTS", "DD5.1", "ATMOS", "TRUEHD",
		"HDR", "HDR10", "DOLBY", "REMUX",
		"-GROUP", ".GROUP", "[GROUP]",
	}

	for _, marker := range markers {
		if strings.Contains(upper, marker) {
			return true
		}
	}

	return false
}

// findInvalidCharacters returns characters that are problematic for filesystems
// Jellyfin doesn't support: < > : " / \ | ? *
func findInvalidCharacters(filename string) []string {
	invalidChars := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*"}
	found := []string{}

	for _, char := range invalidChars {
		if strings.Contains(filename, char) {
			found = append(found, char)
		}
	}

	return found
}

// isValidSeasonFolder checks if season folder uses proper zero-padding
func isValidSeasonFolder(folder string) bool {
	// Valid formats: "Season 01", "Season 02", ..., "Season 99"
	// Invalid: "Season 1", "season 01", "S01"

	if !strings.HasPrefix(folder, "Season ") {
		return false
	}

	seasonNum := strings.TrimPrefix(folder, "Season ")

	// Must be exactly 2 digits
	if len(seasonNum) != 2 {
		return false
	}

	// Must be numeric
	for _, c := range seasonNum {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// ComplianceSuggestion contains the suggested compliant path and action
type ComplianceSuggestion struct {
	OriginalPath  string
	SuggestedPath string
	Action        string // "rename", "move", or "reorganize"
	Description   string
	Issues        []string
	IsSafeAutoFix bool // true if this is a low-risk fix (case/punctuation only)
}

// SuggestCompliantPath returns a suggested Jellyfin-compliant path for a file
func (c *Checker) SuggestCompliantPath(fullPath string) (*ComplianceSuggestion, error) {
	filename := filepath.Base(fullPath)
	ext := filepath.Ext(filename)
	if ext != "" {
		ext = ext[1:] // Remove leading dot
	}

	// Check current compliance
	result := c.CheckFile(fullPath)
	if result.IsCompliant {
		return nil, nil // Already compliant
	}

	suggestion := &ComplianceSuggestion{
		OriginalPath: fullPath,
		Issues:       result.Issues,
	}

	// Determine media type and compute correct path
	if naming.IsTVEpisodeFilename(filename) {
		return c.suggestTVPath(fullPath, ext, suggestion)
	}

	if naming.IsMovieFilename(filename) {
		return c.suggestMoviePath(fullPath, ext, suggestion)
	}

	return nil, fmt.Errorf("unable to determine media type for: %s", filename)
}

func (c *Checker) suggestMoviePath(fullPath, ext string, suggestion *ComplianceSuggestion) (*ComplianceSuggestion, error) {
	// Use PathComponents to cache parsed path elements
	components, err := ParsePathComponents(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path components: %w", err)
	}

	ctx, err := components.GetContext()
	if err != nil {
		return nil, err
	}

	movie, err := naming.ParseMovieName(components.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to parse movie name: %w", err)
	}

	// Use folder context for show name and year (authoritative)
	showName := ctx.ShowName
	year := ctx.Year
	if year == "" && movie.Year != "" {
		year = movie.Year
	}

	correctFolder := showName
	if year != "" {
		correctFolder = fmt.Sprintf("%s (%s)", showName, year)
	}
	correctFilename := naming.FormatMovieFilename(showName, year, ext)
	correctPath := filepath.Join(ctx.LibraryRoot, correctFolder, correctFilename)

	suggestion.SuggestedPath = correctPath

	// Use cached components instead of repeated filepath operations
	folderDiff := components.ShowFolder != correctFolder
	fileDiff := components.Filename != correctFilename

	if folderDiff && fileDiff {
		suggestion.Action = "reorganize"
		suggestion.Description = fmt.Sprintf("Move to: %s/%s", correctFolder, correctFilename)
	} else if folderDiff {
		suggestion.Action = "move"
		suggestion.Description = fmt.Sprintf("Move to folder: %s", correctFolder)
	} else {
		suggestion.Action = "rename"
		suggestion.Description = fmt.Sprintf("Rename to: %s", correctFilename)
	}

	suggestion.IsSafeAutoFix = isCaseOrPunctuationOnly(components.ShowFolder, correctFolder) &&
		isCaseOrPunctuationOnly(components.Filename, correctFilename)

	return suggestion, nil
}

func (c *Checker) suggestTVPath(fullPath, ext string, suggestion *ComplianceSuggestion) (*ComplianceSuggestion, error) {
	// Use PathComponents to cache parsed path elements
	components, err := ParsePathComponents(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path components: %w", err)
	}

	ctx, err := components.GetContext()
	if err != nil {
		return nil, err
	}

	tv, err := naming.ParseTVShowName(components.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TV show name: %w", err)
	}

	showName := ctx.ShowName
	year := ctx.Year
	if year == "" && tv.Year != "" {
		year = tv.Year
	}

	showFolder := showName
	if year != "" {
		showFolder = fmt.Sprintf("%s (%s)", showName, year)
	}

	seasonFolder := naming.FormatSeasonFolder(tv.Season)
	correctFilename := naming.FormatTVEpisodeFilename(showName, year, tv.Season, tv.Episode, ext)
	correctPath := filepath.Join(ctx.LibraryRoot, showFolder, seasonFolder, correctFilename)

	suggestion.SuggestedPath = correctPath

	// Use cached components instead of repeated filepath operations
	showDiff := components.ShowFolder != showFolder
	seasonDiff := components.SeasonFolder != seasonFolder
	fileDiff := components.Filename != correctFilename

	if showDiff || seasonDiff {
		if fileDiff {
			suggestion.Action = "reorganize"
			suggestion.Description = fmt.Sprintf("Move to: %s/%s/%s", showFolder, seasonFolder, correctFilename)
		} else {
			suggestion.Action = "move"
			suggestion.Description = fmt.Sprintf("Move to: %s/%s/", showFolder, seasonFolder)
		}
	} else {
		suggestion.Action = "rename"
		suggestion.Description = fmt.Sprintf("Rename to: %s", correctFilename)
	}

	suggestion.IsSafeAutoFix = isCaseOrPunctuationOnly(components.ShowFolder, showFolder) &&
		isCaseOrPunctuationOnly(components.SeasonFolder, seasonFolder) &&
		isCaseOrPunctuationOnly(components.Filename, correctFilename)

	return suggestion, nil
}

// isCaseOrPunctuationOnly returns true if the only differences are case or minor punctuation
func isCaseOrPunctuationOnly(original, suggested string) bool {
	// Normalize: lowercase, remove common punctuation variations
	normalize := func(s string) string {
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, "'", "")
		s = strings.ReplaceAll(s, "'", "")
		return s
	}
	return normalize(original) == normalize(suggested)
}
