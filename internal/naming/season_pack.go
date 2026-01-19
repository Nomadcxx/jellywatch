package naming

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SeasonPackInfo holds parsed season pack folder data
type SeasonPackInfo struct {
	ShowName string
	Year     string
	Season   int
}

var (
	seasonMarkerPattern = regexp.MustCompile(`(?i)\.S(\d{2})\.`)
	yearInNamePattern   = regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
)

// IsReleaseFormatFolder detects release-named folders like:
// "The.Great.2020.S02.1080p.DS4K.HULU.Webrip.DV.HDR10+.DDP5.1.x265.-Vialle"
func IsReleaseFormatFolder(folderName string) bool {
	// Must have season marker (S01, S02, etc.)
	if !seasonMarkerPattern.MatchString(folderName) {
		return false
	}

	// Must have release markers (quality, codec, or source)
	return hasReleaseMarkers(folderName)
}

// hasReleaseMarkers checks if folder name contains release quality/codec markers
func hasReleaseMarkers(folderName string) bool {
	lower := strings.ToLower(folderName)

	// Check for quality markers
	qualityMarkers := []string{"1080p", "720p", "2160p", "4k", "bluray", "webrip", "web-dl", "hdtv"}
	for _, marker := range qualityMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}

	// Check for codec markers
	codecMarkers := []string{"x264", "x265", "h.264", "h.265", "hevc"}
	for _, marker := range codecMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}

	// Check for source markers
	sourceMarkers := []string{"hulu", "netflix", "disney", "hbo", "amazon", "amzn"}
	for _, marker := range sourceMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}

	return false
}

// ParseSeasonPackFolder extracts show info from release folder name
// "The.Great.2020.S02.1080p..." → Show="The Great", Year="2020", Season=2
func ParseSeasonPackFolder(folderName string) (*SeasonPackInfo, error) {
	// Find S## marker
	seasonMatch := seasonMarkerPattern.FindStringSubmatchIndex(folderName)
	if seasonMatch == nil {
		return nil, fmt.Errorf("no season marker found")
	}

	// Everything before S## is show name + year
	beforeSeason := folderName[:seasonMatch[0]]

	// Extract year (if present)
	year := ""
	yearMatch := yearInNamePattern.FindStringSubmatch(beforeSeason)
	if len(yearMatch) > 0 {
		year = yearMatch[0]
	}

	// Clean show name (dots to spaces, remove year)
	showName := strings.ReplaceAll(beforeSeason, ".", " ")
	showName = strings.TrimSpace(showName)
	if year != "" {
		// Remove year from show name
		showName = strings.ReplaceAll(showName, year, "")
		showName = strings.TrimSpace(showName)
	}

	// Extract season number
	seasonStr := folderName[seasonMatch[2]:seasonMatch[3]]
	seasonNum, err := strconv.Atoi(seasonStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse season number: %w", err)
	}

	return &SeasonPackInfo{
		ShowName: showName,
		Year:     year,
		Season:   seasonNum,
	}, nil
}
