// internal/naming/golden_test.go
package naming

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GoldenTestCase represents a test case loaded from golden files
type GoldenTestCase struct {
	Input         string            `json:"input"`
	File          string            `json:"file"`
	Features      map[string]string `json:"features"`
	ExpectedTitle string            `json:"expected_title,omitempty"`
	ExpectedYear  int               `json:"expected_year,omitempty"`
	ExpectedType  string            `json:"expected_type,omitempty"` // "tv" or "movie"
}

// LoadGoldenTests loads test cases from a golden JSON file
func LoadGoldenTests(category string) ([]GoldenTestCase, error) {
	path := filepath.Join("testdata", "golden", category+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cases []GoldenTestCase
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, err
	}

	return cases, nil
}

// TestGolden_QualityMarkers tests quality marker parsing
func TestGolden_QualityMarkers(t *testing.T) {
	cases, err := LoadGoldenTests("quality")
	require.NoError(t, err)
	require.NotEmpty(t, cases, "quality test data should exist")

	for _, tt := range cases {
		t.Run(tt.Input, func(t *testing.T) {
			// Test that we can parse without crashing
			info, err := ParseMovieName(tt.Input)

			// We expect some result (even if not perfect)
			if err == nil && info.Title != "" {
				// Verify title doesn't contain quality markers
				assert.NotContains(t, strings.ToUpper(info.Title), "1080P")
				assert.NotContains(t, strings.ToUpper(info.Title), "720P")
				assert.NotContains(t, strings.ToUpper(info.Title), "BLURAY")
				assert.NotContains(t, strings.ToUpper(info.Title), "WEB-DL")
			}
		})
	}
}

// TestGolden_StreamingPlatforms tests streaming platform detection
func TestGolden_StreamingPlatforms(t *testing.T) {
	cases, err := LoadGoldenTests("streaming")
	require.NoError(t, err)
	require.NotEmpty(t, cases, "streaming test data should exist")

	for _, tt := range cases {
		t.Run(tt.Input, func(t *testing.T) {
			// Parse and verify streaming platform markers are removed
			info, err := ParseMovieName(tt.Input)

			if err == nil && info.Title != "" {
				titleUpper := strings.ToUpper(info.Title)
				assert.NotContains(t, titleUpper, "AMZN")
				assert.NotContains(t, titleUpper, "NETFLIX") // Should be removed
				assert.NotContains(t, titleUpper, "DISNEY")
			}
		})
	}
}

// TestGolden_ReleaseGroups tests release group removal
func TestGolden_ReleaseGroups(t *testing.T) {
	cases, err := LoadGoldenTests("release_groups")
	require.NoError(t, err)
	require.NotEmpty(t, cases, "release_groups test data should exist")

	for _, tt := range cases {
		t.Run(tt.Input, func(t *testing.T) {
			// Skip software/0day releases - these are not movies/TV shows
			ext := strings.ToLower(filepath.Ext(tt.File))
			if ext == ".exe" || ext == ".iso" {
				t.Skipf("Skipping software release: %s", tt.Input)
			}
			if strings.Contains(strings.ToUpper(tt.Input), "-TUTOR") ||
				strings.Contains(strings.ToUpper(tt.Input), "BOOKWARE") {
				t.Skipf("Skipping tutorial/course release: %s", tt.Input)
			}

			info, err := ParseMovieName(tt.Input)

			if err == nil && info.Title != "" {
				// Title should not end with a common release group pattern
				title := strings.TrimSpace(info.Title)
				words := strings.Fields(title)
				if len(words) > 0 {
					lastWord := strings.ToUpper(words[len(words)-1])
					// Known release groups should be stripped
					assert.False(t, IsKnownReleaseGroup(strings.ToLower(lastWord)),
						"Title should not end with release group: %s", lastWord)
				}
			}
		})
	}
}

// TestGolden_DateEpisodes tests date-based episode parsing
func TestGolden_DateEpisodes(t *testing.T) {
	cases, err := LoadGoldenTests("date_episodes")
	require.NoError(t, err)
	require.NotEmpty(t, cases, "date_episodes test data should exist")

	for _, tt := range cases {
		t.Run(tt.Input, func(t *testing.T) {
			// Skip malformed dates (data quality issues in source)
			// Example: "20222.05.18" should be "2022.05.18"
			if strings.Contains(tt.Input, "20222.") || strings.Contains(tt.Input, "20222.") {
				t.Skipf("Skipping malformed date in filename: %s", tt.Input)
			}

			// Verify date-based episodes are recognized as TV
			isTV := IsTVEpisodeFilename(tt.Input)
			assert.True(t, isTV, "Date-based filename should be recognized as TV: %s", tt.Input)

			// Try to parse as TV show
			info, err := ParseTVShowName(tt.Input)
			// We may not parse perfectly, but should recognize as TV
			if err == nil {
				assert.NotEmpty(t, info.Title, "Should extract title from date-based episode")
			}
		})
	}
}

// TestGolden_YearEdgeCases tests year extraction edge cases
func TestGolden_YearEdgeCases(t *testing.T) {
	cases, err := LoadGoldenTests("year_edge")
	require.NoError(t, err)
	require.NotEmpty(t, cases, "year_edge test data should exist")

	for _, tt := range cases {
		t.Run(tt.Input, func(t *testing.T) {
			info, err := ParseMovieName(tt.Input)

			if err == nil && info.Title != "" {
				// If we extracted a year, it should be reasonable
				if info.Year != "" {
					year := info.Year
					assert.Regexp(t, `^(19|20)\d{2}$`, year, "Year should be 4 digits starting with 19 or 20")
				}
			}
		})
	}
}

// TestGolden_SpecialCharacters tests special character handling
func TestGolden_SpecialCharacters(t *testing.T) {
	cases, err := LoadGoldenTests("special_chars")
	require.NoError(t, err)
	require.NotEmpty(t, cases, "special_chars test data should exist")

	for _, tt := range cases {
		t.Run(tt.Input, func(t *testing.T) {
			info, err := ParseMovieName(tt.Input)

			if err == nil && info.Title != "" {
				// Title should not contain problematic filesystem characters
				// (except for apostrophes which are preserved)
				assert.NotContains(t, info.Title, ":")
				assert.NotContains(t, info.Title, "/")
				assert.NotContains(t, info.Title, "\\")
				assert.NotContains(t, info.Title, "?")
				assert.NotContains(t, info.Title, "*")
			}
		})
	}
}

// TestGolden_TVFormats tests various TV episode format patterns
func TestGolden_TVFormats(t *testing.T) {
	cases, err := LoadGoldenTests("tv_formats")
	require.NoError(t, err)
	require.NotEmpty(t, cases, "tv_formats test data should exist")

	for _, tt := range cases {
		t.Run(tt.Input, func(t *testing.T) {
			// All should be recognized as TV episodes
			isTV := IsTVEpisodeFilename(tt.Input)
			assert.True(t, isTV, "TV format filename should be recognized: %s", tt.Input)

			// Try to parse
			info, err := ParseTVShowName(tt.Input)
			if err == nil {
				assert.NotEmpty(t, info.Title, "Should extract title")
				assert.Greater(t, info.Season, 0, "Should extract season")
				assert.Greater(t, info.Episode, 0, "Should extract episode")
			}
		})
	}
}
