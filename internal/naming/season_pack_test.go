package naming

import (
	"testing"
)

func TestIsReleaseFormatFolder(t *testing.T) {
	tests := []struct {
		name       string
		folderName string
		want       bool
	}{
		{
			name:       "typical season pack folder",
			folderName: "The.Great.2020.S02.1080p.DS4K.HULU.Webrip.DV.HDR10+.DDP5.1.x265.-Vialle",
			want:       true,
		},
		{
			name:       "season pack with different markers",
			folderName: "Breaking.Bad.S05.COMPLETE.1080p.BluRay.x264-GROUP",
			want:       true,
		},
		{
			name:       "regular Jellyfin season folder",
			folderName: "Season 01",
			want:       false,
		},
		{
			name:       "show folder with year",
			folderName: "The Great (2020)",
			want:       false,
		},
		{
			name:       "no season marker",
			folderName: "The.Great.2020.1080p.HULU.Webrip",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsReleaseFormatFolder(tt.folderName)
			if got != tt.want {
				t.Errorf("IsReleaseFormatFolder(%q) = %v, want %v",
					tt.folderName, got, tt.want)
			}
		})
	}
}

func TestParseSeasonPackFolder(t *testing.T) {
	tests := []struct {
		name       string
		folderName string
		wantShow   string
		wantYear   string
		wantSeason int
		wantErr    bool
	}{
		{
			name:       "The Great season 2",
			folderName: "The.Great.2020.S02.1080p.DS4K.HULU.Webrip.DV.HDR10+.DDP5.1.x265.-Vialle",
			wantShow:   "The Great",
			wantYear:   "2020",
			wantSeason: 2,
			wantErr:    false,
		},
		{
			name:       "Breaking Bad season 5",
			folderName: "Breaking.Bad.S05.COMPLETE.1080p.BluRay.x264-GROUP",
			wantShow:   "Breaking Bad",
			wantYear:   "",
			wantSeason: 5,
			wantErr:    false,
		},
		{
			name:       "show with year in middle",
			folderName: "Show.2020.S01.1080p.WEB-DL",
			wantShow:   "Show",
			wantYear:   "2020",
			wantSeason: 1,
			wantErr:    false,
		},
		{
			name:       "no season marker",
			folderName: "The.Great.2020.1080p.HULU.Webrip",
			wantShow:   "",
			wantYear:   "",
			wantSeason: 0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSeasonPackFolder(tt.folderName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSeasonPackFolder(%q) error = %v, wantErr %v",
					tt.folderName, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if got.ShowName != tt.wantShow {
				t.Errorf("ParseSeasonPackFolder(%q).ShowName = %q, want %q",
					tt.folderName, got.ShowName, tt.wantShow)
			}
			if got.Year != tt.wantYear {
				t.Errorf("ParseSeasonPackFolder(%q).Year = %q, want %q",
					tt.folderName, got.Year, tt.wantYear)
			}
			if got.Season != tt.wantSeason {
				t.Errorf("ParseSeasonPackFolder(%q).Season = %d, want %d",
					tt.folderName, got.Season, tt.wantSeason)
			}
		})
	}
}
