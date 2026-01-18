package compliance

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var yearPattern = regexp.MustCompile(`\((\d{4})\)$`)

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

type FolderContext struct {
	ShowName     string
	Year         string
	SeasonFolder string
	LibraryRoot  string
}

func ExtractFolderContext(fullPath string) (FolderContext, error) {
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
	}, nil
}

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

type PathComponents struct {
	FullPath     string
	Filename     string
	SeasonFolder string
	ShowFolder   string
	LibraryRoot  string

	context    *FolderContext
	contextErr error
}

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

func (p *PathComponents) GetContext() (FolderContext, error) {
	if p.contextErr != nil {
		return FolderContext{}, p.contextErr
	}
	if p.context == nil {
		return FolderContext{}, fmt.Errorf("context not cached, call ParsePathComponents first")
	}
	return *p.context, nil
}

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
