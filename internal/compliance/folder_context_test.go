package compliance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractShowFromFolderPath_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
		expectedShow string
		expectedYear string
	}{
		{
			name:        "empty path",
			path:        "",
			expectError: true,
		},
		{
			name:        "root path",
			path:        "/",
			expectError: true,
		},
		{
			name:        "filename only",
			path:        "file.mkv",
			expectError: true,
		},
		{
			name:        "one level deep",
			path:        "/file.mkv",
			expectError: true,
		},
		{
			name:        "two levels deep",
			path:        "/season/file.mkv",
			expectError: true,
		},
		{
			name:          "valid path with year",
			path:          "/tv/Show Name (1989)/Season 01/file.mkv",
			expectError:   false,
			expectedShow:  "Show Name",
			expectedYear:  "1989",
		},
		{
			name:          "valid path without year",
			path:          "/tv/Show Name/Season 01/file.mkv",
			expectError:   false,
			expectedShow:  "Show Name",
			expectedYear:  "",
		},
		{
			name:          "year with spaces",
			path:          "/tv/Show Name ( 1989 )/Season 01/file.mkv",
			expectError:   false,
			expectedShow:  "Show Name ( 1989 )", // Regex doesn't match spaces, so no year extracted
			expectedYear:  "",
		},
		{
			name:        "invalid year format (too short)",
			path:        "/tv/Show Name (89)/Season 01/file.mkv",
			expectError: false, // Currently extracts "89" - regex only matches 4 digits
			expectedShow: "Show Name (89)",
			expectedYear: "",
		},
		{
			name:        "year out of range (too old)",
			path:        "/tv/Show Name (1800)/Season 01/file.mkv",
			expectError: true, // Should fail validation
		},
		{
			name:        "year out of range (too new)",
			path:        "/tv/Show Name (2200)/Season 01/file.mkv",
			expectError: true, // Should fail validation
		},
		{
			name:          "multiple years (takes last)",
			path:          "/tv/Show Name (1989) (2025)/Season 01/file.mkv",
			expectError:   false,
			expectedShow:  "Show Name (1989)",
			expectedYear:  "2025", // Takes last year
		},
		{
			name:          "show name with parentheses",
			path:          "/tv/Show (Name) (1989)/Season 01/file.mkv",
			expectError:   false,
			expectedShow:  "Show (Name)",
			expectedYear:  "1989",
		},
		{
			name:          "show name with special characters",
			path:          "/tv/M*A*S*H (1972)/Season 01/file.mkv",
			expectError:   false,
			expectedShow:  "M*A*S*H",
			expectedYear:  "1972",
		},
		{
			name:        "invalid folder name (dots)",
			path:        "/tv/../Season 01/file.mkv",
			expectError: true,
		},
		{
			name:        "show folder same as season folder",
			path:        "/tv/file.mkv",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			show, year, err := ExtractShowFromFolderPath(tt.path)

			if tt.expectError {
				assert.Error(t, err, "expected error for path: %s", tt.path)
			} else {
				require.NoError(t, err, "unexpected error for path: %s", tt.path)
				assert.Equal(t, tt.expectedShow, show, "show name mismatch")
				assert.Equal(t, tt.expectedYear, year, "year mismatch")
			}
		})
	}
}

func TestTitleMatchesFolderContext_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		parsedTitle string
		folderTitle string
		expected    bool
	}{
		{
			name:        "exact match",
			parsedTitle: "The Simpsons",
			folderTitle: "The Simpsons",
			expected:    true,
		},
		{
			name:        "substring match",
			parsedTitle: "Simpsons",
			folderTitle: "The Simpsons",
			expected:    true,
		},
		{
			name:        "case insensitive",
			parsedTitle: "the simpsons",
			folderTitle: "The Simpsons",
			expected:    true,
		},
		{
			name:        "contains match",
			parsedTitle: "The Simpsons Movie",
			folderTitle: "The Simpsons",
			expected:    true,
		},
		{
			name:        "prefix removal",
			parsedTitle: "Simpsons",
			folderTitle: "The Simpsons",
			expected:    true,
		},
		{
			name:        "no match",
			parsedTitle: "Family Guy",
			folderTitle: "The Simpsons",
			expected:    false,
		},
		{
			name:        "empty parsed title",
			parsedTitle: "",
			folderTitle: "The Simpsons",
			expected:    false,
		},
		{
			name:        "empty folder title",
			parsedTitle: "The Simpsons",
			folderTitle: "",
			expected:    false,
		},
		{
			name:        "both empty",
			parsedTitle: "",
			folderTitle: "",
			expected:    false,
		},
		{
			name:        "whitespace only parsed",
			parsedTitle: "   ",
			folderTitle: "The Simpsons",
			expected:    false,
		},
		{
			name:        "whitespace only folder",
			parsedTitle: "The Simpsons",
			folderTitle: "   ",
			expected:    false,
		},
		{
			name:        "partial word match",
			parsedTitle: "Simp",
			folderTitle: "The Simpsons",
			expected:    true, // Contains match
		},
		{
			name:        "reverse substring",
			parsedTitle: "The Simpsons",
			folderTitle: "Simpsons",
			expected:    true, // Contains match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := FolderContext{ShowName: tt.folderTitle}
			result := TitleMatchesFolderContext(tt.parsedTitle, ctx)
			assert.Equal(t, tt.expected, result,
				"TitleMatchesFolderContext(%q, %q) = %v, want %v",
				tt.parsedTitle, tt.folderTitle, result, tt.expected)
		})
	}
}

func TestClassifySuggestion_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		currentPath   string
		suggestedPath string
		expectError   bool
		expectedClass Classification
	}{
		{
			name:          "same folder rename",
			currentPath:   "/tv/Show (1989)/Season 01/old.mkv",
			suggestedPath: "/tv/Show (1989)/Season 01/new.mkv",
			expectError:   false,
			expectedClass: ClassificationSafe,
		},
		{
			name:          "different show folder",
			currentPath:   "/tv/Show1 (1989)/Season 01/file.mkv",
			suggestedPath: "/tv/Show2 (1989)/Season 01/file.mkv",
			expectError:   false,
			expectedClass: ClassificationRisky,
		},
		{
			name:          "different library",
			currentPath:   "/tv1/Show (1989)/Season 01/file.mkv",
			suggestedPath: "/tv2/Show (1989)/Season 01/file.mkv",
			expectError:   false,
			expectedClass: ClassificationRisky,
		},
		{
			name:          "invalid current path",
			currentPath:   "invalid",
			suggestedPath: "/tv/Show (1989)/Season 01/file.mkv",
			expectError:   true,
		},
		{
			name:          "invalid suggested path",
			currentPath:   "/tv/Show (1989)/Season 01/file.mkv",
			suggestedPath: "invalid",
			expectError:   true,
		},
		{
			name:          "same show different season",
			currentPath:   "/tv/Show (1989)/Season 01/file.mkv",
			suggestedPath: "/tv/Show (1989)/Season 02/file.mkv",
			expectError:   false,
			expectedClass: ClassificationRisky, // Different season folder - RISKY per current logic
		},
		{
			name:          "case difference in show name",
			currentPath:   "/tv/the simpsons (1989)/Season 01/file.mkv",
			suggestedPath: "/tv/The Simpsons (1989)/Season 01/file.mkv",
			expectError:   false,
			expectedClass: ClassificationRisky, // Show folder doesn't exist yet - RISKY per current logic
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification, err := ClassifySuggestion(tt.currentPath, tt.suggestedPath)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedClass, classification)
			}
		})
	}
}

func TestExtractFolderContext_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "empty path",
			path:        "",
			expectError: true,
		},
		{
			name:        "invalid path structure",
			path:        "/file.mkv",
			expectError: true,
		},
		{
			name:        "valid path",
			path:        "/tv/Show (1989)/Season 01/file.mkv",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := ExtractFolderContext(tt.path)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, FolderContext{}, ctx)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, ctx.ShowName)
			}
		})
	}
}
