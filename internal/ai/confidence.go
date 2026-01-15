package ai

import (
	"embed"
	"regexp"
	"strings"
)

//go:embed data/release_groups.txt data/known_titles.txt
var dataFS embed.FS

var (
	yearPattern    = regexp.MustCompile(`\((?:19|20)\d{2}\)`)
	allCapsPattern = regexp.MustCompile(`^[A-Z0-9]+$`)
	releasePattern = regexp.MustCompile(`(?i)^(x264|h264|h265|hevc|aac|dts|bluray|webrip|hdtv|repack|proper|internal)`)
)

type ConfidenceCalculator struct {
	releaseGroups map[string]struct{}
	knownTitles   map[string]struct{}
}

func NewConfidenceCalculator() *ConfidenceCalculator {
	return &ConfidenceCalculator{
		releaseGroups: loadSet("data/release_groups.txt"),
		knownTitles:   loadSet("data/known_titles.txt"),
	}
}

func loadSet(filename string) map[string]struct{} {
	data, err := dataFS.ReadFile(filename)
	if err != nil {
		return make(map[string]struct{})
	}

	lines := strings.Split(string(data), "\n")
	set := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			set[strings.ToLower(line)] = struct{}{}
		}
	}
	return set
}

func (c *ConfidenceCalculator) CalculateTV(title, original string) float64 {
	return c.calculateConfidence(title, original)
}

// CalculateMovie calculates confidence for a movie title extraction
func (c *ConfidenceCalculator) CalculateMovie(title, original string) float64 {
	return c.calculateConfidence(title, original)
}

func (c *ConfidenceCalculator) calculateConfidence(title, original string) float64 {
	confidence := 1.0
	lowerTitle := strings.ToLower(title)

	if _, exists := c.releaseGroups[lowerTitle]; exists {
		confidence -= 0.8
	}

	if releasePattern.MatchString(title) {
		confidence -= 0.7
	}

	if len(title) < 3 {
		confidence -= 0.5
	}

	if len(title) > 4 && allCapsPattern.MatchString(title) {
		confidence -= 0.4
	}

	if hasGarbagePrefix(title) {
		confidence -= 0.6
	}

	words := strings.Fields(title)
	if len(words) == 1 && len(title) > 2 {
		if _, exists := c.knownTitles[lowerTitle]; !exists {
			confidence -= 0.15
		}
	}

	if yearPattern.MatchString(original) {
		confidence += 0.1
	}

	if confidence < 0.0 {
		confidence = 0.0
	} else if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

func hasGarbagePrefix(title string) bool {
	lower := strings.ToLower(title)
	prefixes := []string{"www.", "http", "[", "1080", "720", "2160", "x264", "h264"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}
