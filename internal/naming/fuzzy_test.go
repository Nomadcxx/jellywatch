package naming

import (
	"testing"
)

func TestStripStreamingPlatforms(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Netflix marker",
			input:    "Movie Name 2024 NF 1080p BluRay",
			expected: "Movie Name 2024 1080p BluRay",
		},
		{
			name:     "Amazon marker",
			input:    "Show Name S01E01 AMZN WEB-DL",
			expected: "Show Name S01E01 WEB-DL",
		},
		{
			name:     "Disney+ marker",
			input:    "Series Name 2023 DSNP 2160p",
			expected: "Series Name 2023 2160p",
		},
		{
			name:     "HBO Max marker",
			input:    "Movie 2023 HMAX 1080p",
			expected: "Movie 2023 1080p",
		},
		{
			name:     "Apple TV+ marker",
			input:    "Show S01 ATVP WEBRip",
			expected: "Show S01 WEBRip",
		},
		{
			name:     "Multiple platforms",
			input:    "Movie NF AMZN 2024",
			expected: "Movie 2024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripStreamingPlatforms(tt.input)
			if result != tt.expected {
				t.Errorf("StripStreamingPlatforms(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStripExtendedAudio(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "DTS-HD MA marker",
			input:    "Movie 2024 1080p BluRay DTS-HD MA",
			expected: "Movie 2024 1080p BluRay",
		},
		{
			name:     "TrueHD Atmos marker",
			input:    "Movie 2023 TrueHD Atmos 7 1",
			expected: "Movie 2023",
		},
		{
			name:     "DDP 5.1 marker",
			input:    "Show Name S01E01 DDP 5 1",
			expected: "Show Name S01E01",
		},
		{
			name:     "EAC3 marker",
			input:    "Movie 2024 EAC3 5 1",
			expected: "Movie 2024",
		},
		{
			name:     "FLAC marker",
			input:    "Movie 2024 FLAC 2 0",
			expected: "Movie 2024",
		},
		{
			name:     "Atmos only",
			input:    "Movie Atmos 2024",
			expected: "Movie 2024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripExtendedAudio(tt.input)
			if result != tt.expected {
				t.Errorf("StripExtendedAudio(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStripEditionMarkers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Director's Cut",
			input:    "Movie Name 2024 Directors Cut 1080p",
			expected: "Movie Name 2024 1080p",
		},
		{
			name:     "IMAX Enhanced",
			input:    "Movie 2023 IMAX Enhanced 2160p",
			expected: "Movie 2023 2160p",
		},
		{
			name:     "Commentary",
			input:    "Movie Name 2024 Commentary BluRay",
			expected: "Movie Name 2024 BluRay",
		},
		{
			name:     "UNCUT",
			input:    "Movie 2024 UNCUT 1080p",
			expected: "Movie 2024 1080p",
		},
		{
			name:     "Extended Edition",
			input:    "Movie 2023 Extended Edition",
			expected: "Movie 2023",
		},
		{
			name:     "Theatrical Cut",
			input:    "Movie 2024 Theatrical Cut",
			expected: "Movie 2024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripEditionMarkers(tt.input)
			if result != tt.expected {
				t.Errorf("StripEditionMarkers(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStripHyphenSuffixes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Hyphen release group",
			input:    "Movie Name Group",
			expected: "Movie Name",
		},
		{
			name:     "Hyphen x264",
			input:    "Movie Name x264",
			expected: "Movie Name",
		},
		{
			name:     "Hyphen version",
			input:    "Movie Name v2",
			expected: "Movie Name",
		},
		{
			name:     "Multiple hyphen suffixes",
			input:    "Movie Name Group REMUX",
			expected: "Movie Name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripHyphenSuffixes(tt.input)
			if result != tt.expected {
				t.Errorf("StripHyphenSuffixes(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractStreamingPlatform(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Netflix",
			input:    "Movie.2024.NF.1080p",
			expected: "Netflix",
		},
		{
			name:     "Amazon",
			input:    "Movie.2023.AMZN.WEB-DL",
			expected: "Amazon Prime",
		},
		{
			name:     "Disney+",
			input:    "Movie.2024.DSNP.2160p",
			expected: "Disney+",
		},
		{
			name:     "HBO Max",
			input:    "Movie.2023.HMAX.1080p",
			expected: "HBO Max",
		},
		{
			name:     "Apple TV+",
			input:    "Show.S01.ATVP.WEBRip",
			expected: "Apple TV+",
		},
		{
			name:     "Hulu",
			input:    "Show.2024.HULU.1080p",
			expected: "Hulu",
		},
		{
			name:     "No platform",
			input:    "Movie.2024.1080p.BluRay.x264",
			expected: "",
		},
		{
			name:     "Peacock",
			input:    "Show.2024.PCOK.WEB-DL",
			expected: "Peacock",
		},
		{
			name:     "Paramount+",
			input:    "Movie.2024.PMTP.1080p",
			expected: "Paramount+",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractStreamingPlatform(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractStreamingPlatform(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsStreamingOnly(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Streaming only with year",
			input:    "Movie.2024.NF",
			expected: false, // Has year and title
		},
		{
			name:     "Short streaming marker",
			input:    "NF",
			expected: true, // Just the marker
		},
		{
			name:     "Not streaming only",
			input:    "Movie.2024.1080p.BluRay",
			expected: false,
		},
		{
			name:     "Title with streaming marker",
			input:    "MyMovie.NF.2024",
			expected: false, // Has title and year
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsStreamingOnly(tt.input)
			if result != tt.expected {
				t.Errorf("IsStreamingOnly(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestCollapseSpaces tests the helper function
func TestCollapseSpaces(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Multiple spaces",
			input:    "Movie    Name    2024",
			expected: "Movie Name 2024",
		},
		{
			name:     "Leading/trailing spaces",
			input:    "   Movie Name 2024   ",
			expected: "Movie Name 2024",
		},
		{
			name:     "Tabs and spaces mixed",
			input:    "Movie\t\tName  \t 2024",
			expected: "Movie Name 2024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collapseSpaces(tt.input)
			if result != tt.expected {
				t.Errorf("collapseSpaces(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
