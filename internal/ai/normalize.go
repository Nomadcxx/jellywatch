package ai

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	separatorPattern     = regexp.MustCompile(`[._-]+`)
	whitespacePattern    = regexp.MustCompile(`\s+`)
	seasonEpisodePattern = regexp.MustCompile(`(?i)s(\d{1,2})e(\d{1,2})`)
	// Valid media extensions that should be stripped
	mediaExtensions = map[string]bool{
		".mkv": true, ".mp4": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
		".mpg": true, ".mpeg": true, ".ts": true, ".m2ts": true,
	}
)

// NormalizeForCache normalizes a filename for cache key lookup.
// This ensures different separator variants hit the same cache entry.
func NormalizeForCache(filename string) string {
	// 1. Extract base filename (no path)
	base := filepath.Base(filename)

	// 2. Only strip known media extensions (avoid stripping .S01E01 as an extension)
	ext := strings.ToLower(filepath.Ext(base))
	if mediaExtensions[ext] {
		base = strings.TrimSuffix(base, filepath.Ext(base))
	}

	// 3. Lowercase for case-insensitive matching
	base = strings.ToLower(base)

	// 4. Normalize separators (dots, underscores, dashes → spaces)
	base = separatorPattern.ReplaceAllString(base, " ")

	// 5. Collapse multiple spaces into single space
	base = strings.Join(strings.Fields(base), " ")

	// 6. Trim
	return strings.TrimSpace(base)
}
