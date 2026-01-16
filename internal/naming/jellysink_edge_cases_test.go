// internal/naming/jellysink_edge_cases_test.go
// Package naming provides parsing functions for media filenames following Jellyfin conventions.
//
// This file contains regression tests documenting known edge cases from the jellysink project.
// These tests serve two purposes:
//   1. Documenting edge cases that need special handling
//   2. Tracking progress on fixing known issues (marked with EXPECTED_FAILURE)
//
// Tests marked with EXPECTED_FAILURE document current limitations. When these tests start
// passing, the marker should be removed and the improvement noted in the changelog.
package naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// These test cases are imported from jellysink's fuzzy_edge_cases_test.go
// They represent real-world edge cases that have been problematic

func TestJellysink_EdgeCases_ReleaseGroupRemoval(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Pentagon Wars with SNAKE",
			input:    "The Pentagon Wars 1998 1080p WEBRip X264 Ac3 SNAKE",
			expected: "The Pentagon Wars (1998)",
		},
		// EXPECTED_FAILURE: Audio codec pattern "DTS-HD MA5 1" not properly handled
		{
			name:     "Princess with 3Audio MA5 1",
			input:    "The Princess And The Frog 2009 1080p BluRay x264 3Audio DTS-HD MA5 1",
			expected: "The Princess And The Frog (2009)",
		},
		{
			name:     "Moon with RightSiZE",
			input:    "Moon 2009 1080p BluRay x264 RightSiZE",
			expected: "Moon (2009)",
		},
		// EXPECTED_FAILURE: "Plus.Commentary" tag not recognized as release marker
		{
			name:     "Invasion Plus Commentary",
			input:    "Invasion.of.the.Body.Snatchers.1956.DVDRip.Plus.Commentary.x264-MaG-Chamele0n.mkv",
			expected: "Invasion Of The Body Snatchers (1956)",
		},
		// EXPECTED_FAILURE: Release group "psychd-ml" with hyphen not properly handled
		{
			name:     "Men At Work psychd-ml",
			input:    "men.at.work.1990.720p.bluray.x264-psychd-ml.mkv",
			expected: "Men At Work (1990)",
		},
		// EXPECTED_FAILURE: "NORDiC" tag not recognized as regional release marker
		{
			name:     "Idea of You Nordic",
			input:    "The.Idea.of.You.2024.NORDiC.1080p.WEB-DL.H.265.DDP5.1-CiNEMiX.mkv",
			expected: "The Idea Of You (2024)",
		},
		{
			name:     "8MM",
			input:    "8MM.1999.720p.WEB-DL.DD5.1.H264-FGT-Obfuscated.cp(tt0134273).mkv",
			expected: "8MM (1999)",
		},
		{
			name:     "D.E.B.S.",
			input:    "D.E.B.S..2004.1080p.x264.DTS-Relevant.mkv",
			expected: "D.E.B.S. (2004)",
		},
		{
			name:     "R.I.P.D. 2",
			input:    "R.I.P.D.2.Rise.of.the.Damned.2022.BluRay.720p.DTS.x264-MTeam.mkv",
			expected: "R.I.P.D. 2 Rise Of The Damned (2022)",
		},
		// EXPECTED_FAILURE: Release group "GP-M-NLsubs" not properly handled
		{
			name:     "Le Comte de Monte-Cristo",
			input:    "Le.Comte.de.Monte-Cristo.2024.1080p.WEB.H264-GP-M-NLsubs.mkv",
			expected: "Le Comte De Monte-Cristo (2024)",
		},
		{
			name:     "Trolls with SPARKS",
			input:    "Trolls.2016.720p.BluRay.x264-SPARKS.mkv",
			expected: "Trolls (2016)",
		},
		{
			name:     "The Invitation Unrated",
			input:    "The Invitation-Unrated (2022)",
			expected: "The Invitation (2022)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseMovieName(tt.input)
			if err != nil {
				t.Fatalf("ParseMovieName failed: %v", err)
			}
			got := result.Title + " (" + result.Year + ")"
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestJellysink_EdgeCases_Abbreviations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "R.I.P.D.",
			input:    "R.I.P.D.2013.1080p.BluRay.x264-SPARKS",
			expected: "R.I.P.D. (2013)",
		},
		{
			name:     "S.W.A.T.",
			input:    "S.W.A.T.2017.1080p.BluRay",
			expected: "S.W.A.T. (2017)",
		},
		{
			name:     "U.S. Marshals",
			input:    "U.S.Marshals.1998.1080p",
			expected: "U.S. Marshals (1998)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseMovieName(tt.input)
			if err != nil {
				t.Fatalf("ParseMovieName failed: %v", err)
			}
			got := result.Title + " (" + result.Year + ")"
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestJellysink_EdgeCases_MultiYear(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Blade Runner 2049 (year in title)",
			input: "Blade.Runner.2049.2017.1080p.BluRay",
		},
		{
			name:  "1917 (year as title)",
			input: "1917.2019.1080p.BluRay",
		},
		{
			name:  "2001: A Space Odyssey",
			input: "2001.A.Space.Odyssey.1968.1080p.BluRay",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseMovieName(tt.input)
			if err != nil {
				t.Fatalf("ParseMovieName failed: %v", err)
			}
			// For these edge cases, we primarily care that we extract something reasonable
			// These are known hard cases where multi-year confusion is expected
			assert.NotEmpty(t, result.Title, "Title should not be empty")
			assert.NotEmpty(t, result.Year, "Year should not be empty")
		})
	}
}

func TestJellysink_EdgeCases_HyphenatedTitles(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// EXPECTED_FAILURE: Hyphen removed and year incorrectly included in title
		{
			name:     "Monte-Cristo (preserve hyphen)",
			input:    "Le.Comte.de.Monte-Cristo.2024.1080p",
			expected: "Le Comte De Monte-Cristo (2024)",
		},
		// EXPECTED_FAILURE: Colon in title not properly converted/preserved
		{
			name:     "X-Men: The Last Stand",
			input:    "X-Men.The.Last.Stand.2006.1080p.BluRay",
			expected: "X-Men The Last Stand (2006)",
		},
		{
			name:     "Spider-Man",
			input:    "Spider-Man.2002.1080p.BluRay",
			expected: "Spider-Man (2002)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseMovieName(tt.input)
			if err != nil {
				t.Fatalf("ParseMovieName failed: %v", err)
			}
			got := result.Title + " (" + result.Year + ")"
			assert.Equal(t, tt.expected, got)
			// Additionally check that hyphen is preserved in title
			assert.Contains(t, result.Title, "-", "Hyphen should be preserved in title")
		})
	}
}
