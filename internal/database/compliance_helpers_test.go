package database

import (
	"testing"
)

func TestCheckCompliance_Movies(t *testing.T) {
	tests := []struct {
		name        string
		fullPath    string
		libraryRoot string
		wantValid   bool
	}{
		{
			name:        "Compliant movie",
			fullPath:    "/media/Movies/Interstellar (2014)/Interstellar (2014).mkv",
			libraryRoot: "/media/Movies",
			wantValid:   true,
		},
		{
			name:        "Non-compliant movie with markers",
			fullPath:    "/media/Movies/Interstellar (2014)/Interstellar.2014.1080p.BluRay.mkv",
			libraryRoot: "/media/Movies",
			wantValid:   false,
		},
		{
			name:        "Non-compliant movie missing year",
			fullPath:    "/media/Movies/Interstellar/Interstellar.mkv",
			libraryRoot: "/media/Movies",
			wantValid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isCompliant, issues := CheckCompliance(tt.fullPath, tt.libraryRoot)

			if isCompliant != tt.wantValid {
				t.Errorf("CheckCompliance() isCompliant = %v, want %v. Issues: %v",
					isCompliant, tt.wantValid, issues)
			}

			if tt.wantValid && len(issues) > 0 {
				t.Errorf("Expected no issues for compliant file, got: %v", issues)
			}

			if !tt.wantValid && len(issues) == 0 {
				t.Error("Expected issues for non-compliant file, got none")
			}
		})
	}
}

func TestCheckCompliance_Episodes(t *testing.T) {
	tests := []struct {
		name        string
		fullPath    string
		libraryRoot string
		wantValid   bool
	}{
		{
			name:        "Compliant episode",
			fullPath:    "/media/TV/Silo (2023)/Season 01/Silo (2023) S01E01.mkv",
			libraryRoot: "/media/TV",
			wantValid:   true,
		},
		{
			name:        "Non-compliant episode with markers",
			fullPath:    "/media/TV/Silo (2023)/Season 01/Silo.2023.S01E01.720p.WEB-DL.mkv",
			libraryRoot: "/media/TV",
			wantValid:   false,
		},
		{
			name:        "Non-compliant episode wrong season folder",
			fullPath:    "/media/TV/Silo (2023)/Season 1/Silo (2023) S01E01.mkv",
			libraryRoot: "/media/TV",
			wantValid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isCompliant, issues := CheckCompliance(tt.fullPath, tt.libraryRoot)

			if isCompliant != tt.wantValid {
				t.Errorf("CheckCompliance() isCompliant = %v, want %v. Issues: %v",
					isCompliant, tt.wantValid, issues)
			}

			if tt.wantValid && len(issues) > 0 {
				t.Errorf("Expected no issues for compliant file, got: %v", issues)
			}

			if !tt.wantValid && len(issues) == 0 {
				t.Error("Expected issues for non-compliant file, got none")
			}
		})
	}
}

func TestNormalizeTitleFromFilename_Movies(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{
			filename: "Interstellar (2014).mkv",
			expected: "interstellar",
		},
		{
			filename: "Her (2013).mkv",
			expected: "her",
		},
		{
			filename: "Top Gun (1986).mkv",
			expected: "topgun",
		},
		{
			filename: "Her.2013.MULTI.1080p.BluRay.x264-Goatlove.mkv",
			expected: "her",
		},
		{
			filename: "The Matrix (1999).mkv",
			expected: "thematrix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := NormalizeTitleFromFilename(tt.filename)
			if result != tt.expected {
				t.Errorf("NormalizeTitleFromFilename(%s) = %s, want %s", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestNormalizeTitleFromFilename_Episodes(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{
			filename: "Silo (2023) S01E01.mkv",
			expected: "silo",
		},
		{
			filename: "Breaking Bad (2008) S01E01.mkv",
			expected: "breakingbad",
		},
		{
			filename: "Silo.S02E05.720p.WEBRip.x264-XEN0N.mkv",
			expected: "silo",
		},
		{
			filename: "The Handmaid's Tale (2017) S01E03.mkv",
			expected: "thehandmaidstale",
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := NormalizeTitleFromFilename(tt.filename)
			if result != tt.expected {
				t.Errorf("NormalizeTitleFromFilename(%s) = %s, want %s", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestCheckMovieCompliance(t *testing.T) {
	fullPath := "/media/Movies/Her (2013)/Her (2013).mkv"
	libraryRoot := "/media/Movies"

	isCompliant, issues := CheckMovieCompliance(fullPath, libraryRoot)

	if !isCompliant {
		t.Errorf("Expected compliant movie, got issues: %v", issues)
	}

	if len(issues) != 0 {
		t.Errorf("Expected no issues, got: %v", issues)
	}
}

func TestCheckEpisodeCompliance(t *testing.T) {
	fullPath := "/media/TV/Silo (2023)/Season 01/Silo (2023) S01E01.mkv"
	libraryRoot := "/media/TV"

	isCompliant, issues := CheckEpisodeCompliance(fullPath, libraryRoot)

	if !isCompliant {
		t.Errorf("Expected compliant episode, got issues: %v", issues)
	}

	if len(issues) != 0 {
		t.Errorf("Expected no issues, got: %v", issues)
	}
}
