package compliance

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Nomadcxx/jellywatch/internal/naming"
)

var yearPattern = regexp.MustCompile(`\((\d{4})\)$`)

// ExtractShowFromFolderPath extracts show name and year from a file's parent folder path.
//
// Expected path structure:
//   /library/Show Name (YYYY)/Season XX/filename.ext
//
// The folder name is parsed to extract:
//   - Show name: Everything before the year pattern
//   - Year: Four-digit year in parentheses at the end
//
// Examples:
//   ExtractShowFromFolderPath("/tv/The Simpsons (1989)/Season 01/episode.mkv")
//   // Returns: "The Simpsons", "1989", nil
//
//   ExtractShowFromFolderPath("/tv/Show Name/Season 01/episode.mkv")
//   // Returns: "Show Name", "", nil
//
// Returns an error if:
//   - Path is empty or invalid
//   - Path structure doesn't match expected format (less than 3 levels)
//   - Show name cannot be extracted
//   - Year is out of reasonable range (1900-2100)
func ExtractShowFromFolderPath(fullPath string) (showName, year string, err error) {
	if fullPath == "" {
		return "", "", fmt.Errorf("empty path")
	}

	fullPath = filepath.Clean(fullPath)

	parts := strings.Split(fullPath, string(filepath.Separator))
	filteredParts := []string{}
	for _, p := range parts {
		if p != "" {
			filteredParts = append(filteredParts, p)
		}
	}

	if len(filteredParts) < 3 {
		return "", "", fmt.Errorf("path too shallow: expected at least 3 levels (library/show/season/file), got %d", len(filteredParts))
	}

	seasonFolder := filepath.Dir(fullPath)
	showFolder := filepath.Dir(seasonFolder)

	if showFolder == seasonFolder || showFolder == "/" || showFolder == "." {
		return "", "", fmt.Errorf("invalid path structure: show folder is root or same as season folder")
	}

	folderName := filepath.Base(showFolder)
	if folderName == "" || folderName == "." || folderName == ".." {
		return "", "", fmt.Errorf("invalid folder name: %q", folderName)
	}

	if matches := yearPattern.FindStringSubmatch(folderName); len(matches) > 1 {
		year = matches[1]

		if yearInt, parseErr := strconv.Atoi(year); parseErr != nil {
			return "", "", fmt.Errorf("invalid year format in folder name: %q", year)
		} else if yearInt < 1900 || yearInt > 2100 {
			return "", "", fmt.Errorf("year out of reasonable range: %d", yearInt)
		}

		showName = strings.TrimSpace(yearPattern.ReplaceAllString(folderName, ""))
	} else {
		showName = folderName
	}

	if showName == "" {
		return "", "", fmt.Errorf("empty show name after extraction from folder: %q", folderName)
	}

	return showName, year, nil
}

// FolderContext holds authoritative metadata extracted from a file's parent folder structure.
//
// The folder structure is considered authoritative because it's typically created
// by Sonarr/Radarr and represents the canonical organization.
type FolderContext struct {
	ShowName     string // Show name extracted from folder
	Year         string // Year extracted from folder (empty if not present)
	SeasonFolder string // Season folder name (e.g., "Season 01")
	LibraryRoot  string // Library root directory
	IsSeasonPack bool   // true if from release-format folder
}

// ExtractFolderContext extracts authoritative metadata from a file's parent folder structure.
//
// Expected path structure:
//   /library/Show Name (YYYY)/Season XX/filename.ext
//
// The folder structure is considered authoritative because it's typically created
// by Sonarr/Radarr and represents the canonical organization.
//
// Returns an error if:
//   - Path is empty or invalid
//   - Path structure doesn't match expected format
//   - Show name cannot be extracted
//
// Example:
//   ctx, err := ExtractFolderContext("/tv/The Simpsons (1989)/Season 01/episode.mkv")
//   // ctx.ShowName = "The Simpsons"
//   // ctx.Year = "1989"
//   // ctx.SeasonFolder = "Season 01"
func ExtractFolderContext(fullPath string) (FolderContext, error) {
	parentFolder := filepath.Base(filepath.Dir(fullPath))

	// Check if parent is a release-format season pack
	if naming.IsReleaseFormatFolder(parentFolder) {
		info, err := naming.ParseSeasonPackFolder(parentFolder)
		if err != nil {
			return FolderContext{}, fmt.Errorf("failed to parse season pack folder: %w", err)
		}
		return FolderContext{
			ShowName:     info.ShowName,
			Year:         info.Year,
			SeasonFolder: fmt.Sprintf("Season %02d", info.Season),
			LibraryRoot:  filepath.Dir(filepath.Dir(fullPath)),
			IsSeasonPack: true,
		}, nil
	}

	// Existing logic for proper Jellyfin structure
	showName, year, err := ExtractShowFromFolderPath(fullPath)
	if err != nil {
		return FolderContext{}, fmt.Errorf("failed to extract show from path: %w", err)
	}

	seasonFolder := filepath.Dir(fullPath)
	showFolder := filepath.Dir(seasonFolder)
	libraryRoot := filepath.Dir(showFolder)

	return FolderContext{
		ShowName:     showName,
		Year:         year,
		SeasonFolder: filepath.Base(seasonFolder),
		LibraryRoot:  libraryRoot,
		IsSeasonPack: false,
	}, nil
}

// TitleMatchesFolderContext checks if a parsed title from a filename
// could belong to the show represented by the folder context.
//
// This uses fuzzy matching to handle common cases where filenames have
// incomplete titles (e.g., "Simpsons" vs "The Simpsons"):
//
//   - Exact match: "The Simpsons" matches "The Simpsons"
//   - Substring match: "Simpsons" matches "The Simpsons"
//   - Contains match: "The Simpsons Movie" matches "The Simpsons"
//   - Case insensitive: "the simpsons" matches "The Simpsons"
//   - Prefix removal: "Simpsons" matches "The Simpsons" (removes "The ")
//
// Examples:
//   TitleMatchesFolderContext("Simpsons", FolderContext{ShowName: "The Simpsons"})
//   // Returns: true
//
//   TitleMatchesFolderContext("Family Guy", FolderContext{ShowName: "The Simpsons"})
//   // Returns: false
//
// Returns false if either input is empty or whitespace-only.
func TitleMatchesFolderContext(parsedTitle string, ctx FolderContext) bool {
	if parsedTitle == "" || strings.TrimSpace(parsedTitle) == "" {
		return false
	}

	if ctx.ShowName == "" || strings.TrimSpace(ctx.ShowName) == "" {
		return false
	}

	normalizedParsed := strings.ToLower(strings.TrimSpace(parsedTitle))
	normalizedFolder := strings.ToLower(strings.TrimSpace(ctx.ShowName))

	if normalizedParsed == normalizedFolder {
		return true
	}

	if strings.Contains(normalizedFolder, normalizedParsed) {
		return true
	}

	if strings.Contains(normalizedParsed, normalizedFolder) {
		return true
	}

	withoutThe := strings.TrimPrefix(normalizedFolder, "the ")
	if normalizedParsed == withoutThe || strings.Contains(withoutThe, normalizedParsed) {
		return true
	}

	return false
}

type Classification string

const (
	ClassificationSafe    Classification = "SAFE"
	ClassificationRisky   Classification = "RISKY"
	ClassificationUnknown Classification = "UNKNOWN"
)

func (c Classification) String() string {
	return string(c)
}

func (c Classification) IsSafe() bool {
	return c == ClassificationSafe
}

func (c Classification) IsRisky() bool {
	return c == ClassificationRisky
}

// PathComponents caches parsed path components to avoid repeated filepath operations.
// Use this when you need to access multiple path components multiple times.
type PathComponents struct {
	FullPath     string // Full file path
	Filename     string // Just the filename
	SeasonFolder string // Season folder name
	ShowFolder   string // Show folder name
	LibraryRoot  string // Library root directory

	context    *FolderContext // Cached folder context
	contextErr error          // Error from context extraction (if any)
}

// ParsePathComponents extracts and caches all path components from a full file path.
// This is more efficient than calling filepath.Dir() multiple times.
//
// Example:
//   components, _ := ParsePathComponents("/tv/Show (1989)/Season 01/file.mkv")
//   // components.Filename = "file.mkv"
//   // components.ShowFolder = "Show (1989)"
func ParsePathComponents(fullPath string) (*PathComponents, error) {
	if fullPath == "" {
		return nil, fmt.Errorf("empty path")
	}

	fullPath = filepath.Clean(fullPath)

	components := &PathComponents{
		FullPath: fullPath,
	}

	components.Filename = filepath.Base(fullPath)
	seasonDir := filepath.Dir(fullPath)
	components.SeasonFolder = filepath.Base(seasonDir)

	showDir := filepath.Dir(seasonDir)
	components.ShowFolder = filepath.Base(showDir)
	components.LibraryRoot = filepath.Dir(showDir)

	ctx, err := ExtractFolderContext(fullPath)
	if err != nil {
		components.contextErr = err
		return components, nil
	}
	components.context = &ctx

	return components, nil
}

// GetContext returns the cached folder context or error.
// Returns an error if context extraction failed during ParsePathComponents.
func (p *PathComponents) GetContext() (FolderContext, error) {
	if p.contextErr != nil {
		return FolderContext{}, p.contextErr
	}
	if p.context == nil {
		return FolderContext{}, fmt.Errorf("context not cached, call ParsePathComponents first")
	}
	return *p.context, nil
}

// ClassifySuggestion determines if a compliance suggestion is SAFE or RISKY.
//
// A suggestion is classified as:
//   - SAFE: File stays in same show folder, only filename/season folder changes
//   - RISKY: Show folder would change, library would change, or target folder doesn't exist
//   - UNKNOWN: Unable to extract context from paths (returns error)
//
// Examples:
//   ClassifySuggestion("/tv/Show (1989)/Season 01/old.mkv", "/tv/Show (1989)/Season 01/new.mkv")
//   // Returns: ClassificationSafe, nil (same folder, just rename)
//
//   ClassifySuggestion("/tv/Show1 (1989)/Season 01/file.mkv", "/tv/Show2 (1989)/Season 01/file.mkv")
//   // Returns: ClassificationRisky, nil (different show folder)
//
// Returns an error if context cannot be extracted from either path.
func ClassifySuggestion(currentPath, suggestedPath string) (Classification, error) {
	// Obfuscated filenames need AI review
	// Import naming package if not already imported
	// For now, we'll skip this check - it's handled in compliance.go CheckEpisode

	// Same directory? Just rename = always SAFE
	if filepath.Dir(currentPath) == filepath.Dir(suggestedPath) {
		return ClassificationSafe, nil
	}

	// Extract contexts for comparison
	currentCtx, err := ExtractFolderContext(currentPath)
	if err != nil {
		return ClassificationUnknown, fmt.Errorf("failed to extract current path context: %w", err)
	}

	suggestedCtx, err := ExtractFolderContext(suggestedPath)
	if err != nil {
		return ClassificationUnknown, fmt.Errorf("failed to extract suggested path context: %w", err)
	}

	// Show name mismatch? Could be misparse - RISKY
	if !TitleMatchesFolderContext(currentCtx.ShowName, suggestedCtx) {
		return ClassificationRisky, nil
	}

	// Check if target show folder exists
	// If creating new folder, mark as RISKY (might be duplicate)
	suggestedShowFolder := filepath.Dir(filepath.Dir(suggestedPath))
	if _, err := os.Stat(suggestedShowFolder); os.IsNotExist(err) {
		return ClassificationRisky, nil
	}

	// Moving to existing folder with matching show name = SAFE
	return ClassificationSafe, nil
}
