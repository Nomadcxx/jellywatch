package compliance

import (
	"path/filepath"
	"regexp"
	"strings"
)

var yearPattern = regexp.MustCompile(`\((\d{4})\)$`)

func ExtractShowFromFolderPath(fullPath string) (showName, year string) {
	seasonFolder := filepath.Dir(fullPath)
	showFolder := filepath.Dir(seasonFolder)
	folderName := filepath.Base(showFolder)

	if matches := yearPattern.FindStringSubmatch(folderName); len(matches) > 1 {
		year = matches[1]
		showName = strings.TrimSpace(yearPattern.ReplaceAllString(folderName, ""))
	} else {
		showName = folderName
	}

	return showName, year
}

type FolderContext struct {
	ShowName     string
	Year         string
	SeasonFolder string
	LibraryRoot  string
}

func ExtractFolderContext(fullPath string) FolderContext {
	seasonFolder := filepath.Dir(fullPath)
	showFolder := filepath.Dir(seasonFolder)

	showName, year := ExtractShowFromFolderPath(fullPath)

	libraryRoot := filepath.Dir(showFolder)

	return FolderContext{
		ShowName:     showName,
		Year:         year,
		SeasonFolder: filepath.Base(seasonFolder),
		LibraryRoot:  libraryRoot,
	}
}

func TitleMatchesFolderContext(parsedTitle string, ctx FolderContext) bool {
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
