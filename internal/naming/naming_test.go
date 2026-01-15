package naming

import (
	"testing"
)

// TestIsTVEpisodeFilename_DateBased tests detection of date-based episodes
// like The Daily Show, Conan, Late Night shows that use YYYY.MM.DD format
// instead of standard S##E## format.
// Bug report: JELLYWATCH_BUG_REPORT_DailyShow_Misclassification.md
func TestIsTVEpisodeFilename_DateBased(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantTV   bool
	}{
		// Date-based episodes (SHOULD be detected as TV)
		{
			name:     "Daily Show with dot separators",
			filename: "The.Daily.Show.2026.01.13.Joachim.Trier.1080p.WEB.h264-EDITH.mkv",
			wantTV:   true,
		},
		{
			name:     "Daily Show different guest",
			filename: "The.Daily.Show.2026.01.14.Stephen.J.Dubner.1080p.WEB.h264-EDITH.mkv",
			wantTV:   true,
		},
		{
			name:     "Conan with dash separators",
			filename: "Conan.2024-03-15.Guest.Name.720p.HDTV.mkv",
			wantTV:   true,
		},
		{
			name:     "Late Show date format",
			filename: "The.Late.Show.2024.01.15.1080p.WEB.mkv",
			wantTV:   true,
		},
		{
			name:     "Tonight Show date format",
			filename: "The.Tonight.Show.Starring.Jimmy.Fallon.2024.02.28.1080p.WEB.mkv",
			wantTV:   true,
		},

		// Standard S##E## episodes (should still work)
		{
			name:     "Standard SxxExx format",
			filename: "Show.S01E01.1080p.mkv",
			wantTV:   true,
		},
		{
			name:     "Standard XxXX format",
			filename: "Show.1x01.1080p.mkv",
			wantTV:   true,
		},

		// Movies (should NOT be detected as TV)
		{
			name:     "Movie with year",
			filename: "Movie.2024.1080p.BluRay.mkv",
			wantTV:   false,
		},
		{
			name:     "Movie with year in title",
			filename: "The.Matrix.1999.mkv",
			wantTV:   false,
		},
		{
			name:     "Movie with full date in middle - no episode context",
			filename: "2012.2009.1080p.BluRay.mkv",
			wantTV:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTVEpisodeFilename(tt.filename)
			if got != tt.wantTV {
				t.Errorf("IsTVEpisodeFilename(%q) = %v, want %v", tt.filename, got, tt.wantTV)
			}
		})
	}
}

func TestParseMovieName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTitle string
		wantYear  string
		wantErr   bool
	}{
		{
			name:      "Standard format",
			input:     "Dune Part Two (2024).mkv",
			wantTitle: "Dune Part Two",
			wantYear:  "2024",
			wantErr:   false,
		},
		{
			name:      "With release markers",
			input:     "Dune.Part.Two.2024.1080p.BluRay.x264-GROUP.mkv",
			wantTitle: "Dune Part Two",
			wantYear:  "2024",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMovieName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMovieName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.Title != tt.wantTitle {
				t.Errorf("ParseMovieName() title = %v, want %v", got.Title, tt.wantTitle)
			}
			if got.Year != tt.wantYear {
				t.Errorf("ParseMovieName() year = %v, want %v", got.Year, tt.wantYear)
			}
		})
	}
}

func TestParseTVShowName(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantTitle  string
		wantSeason int
		wantEp     int
		wantErr    bool
	}{
		{
			name:       "Standard format",
			input:      "Breaking Bad S01E01.mkv",
			wantTitle:  "Breaking Bad",
			wantSeason: 1,
			wantEp:     1,
			wantErr:    false,
		},
		{
			name:       "With release markers",
			input:      "Breaking.Bad.S01E01.1080p.WEB-DL.x264.mkv",
			wantTitle:  "Breaking Bad",
			wantSeason: 1,
			wantEp:     1,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTVShowName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTVShowName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.Title != tt.wantTitle {
				t.Errorf("ParseTVShowName() title = %v, want %v", got.Title, tt.wantTitle)
			}
			if got.Season != tt.wantSeason {
				t.Errorf("ParseTVShowName() season = %v, want %v", got.Season, tt.wantSeason)
			}
			if got.Episode != tt.wantEp {
				t.Errorf("ParseTVShowName() episode = %v, want %v", got.Episode, tt.wantEp)
			}
		})
	}
}
