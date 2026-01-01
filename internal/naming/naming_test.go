package naming

import (
	"testing"
)

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
