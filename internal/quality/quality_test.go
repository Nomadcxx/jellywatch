package quality

import (
	"testing"
)

func TestParseResolution(t *testing.T) {
	tests := []struct {
		filename string
		want     Resolution
	}{
		{"Movie.2024.2160p.BluRay.mkv", Resolution2160p},
		{"Movie.2024.4K.UHD.mkv", Resolution2160p},
		{"Movie.2024.1080p.WEB-DL.mkv", Resolution1080p},
		{"Movie.2024.720p.HDTV.mkv", Resolution720p},
		{"Movie.2024.480p.DVDRip.mkv", Resolution480p},
		{"Movie.2024.576p.mkv", Resolution576p},
		{"Movie.2024.8K.mkv", Resolution4320p},
		{"Movie.2024.mkv", ResolutionUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := Parse(tt.filename)
			if got.Resolution != tt.want {
				t.Errorf("Parse(%q).Resolution = %v, want %v", tt.filename, got.Resolution, tt.want)
			}
		})
	}
}

func TestParseSource(t *testing.T) {
	tests := []struct {
		filename string
		want     Source
	}{
		{"Movie.2024.2160p.REMUX.mkv", SourceREMUX},
		{"Movie.2024.1080p.BluRay.x264.mkv", SourceBluRay},
		{"Movie.2024.1080p.Blu-Ray.mkv", SourceBluRay},
		{"Movie.2024.1080p.BDRip.mkv", SourceBluRay},
		{"Movie.2024.1080p.WEB-DL.mkv", SourceWEBDL},
		{"Movie.2024.1080p.WEBDL.mkv", SourceWEBDL},
		{"Movie.2024.1080p.WEBRip.mkv", SourceWEBRip},
		{"Movie.2024.720p.HDTV.mkv", SourceHDTV},
		{"Movie.2024.DVDRip.mkv", SourceDVDRip},
		{"Movie.2024.DVDScr.mkv", SourceDVDScr},
		{"Movie.2024.TC.mkv", SourceTC},
		{"Movie.2024.TS.mkv", SourceTS},
		{"Movie.2024.CAM.mkv", SourceCAM},
		{"Movie.2024.mkv", SourceUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := Parse(tt.filename)
			if got.Source != tt.want {
				t.Errorf("Parse(%q).Source = %v, want %v", tt.filename, got.Source, tt.want)
			}
		})
	}
}

func TestParseHDR(t *testing.T) {
	tests := []struct {
		filename string
		want     HDRFormat
	}{
		{"Movie.2024.2160p.DV.mkv", DolbyVision},
		{"Movie.2024.2160p.DoVi.mkv", DolbyVision},
		{"Movie.2024.2160p.Dolby.Vision.mkv", DolbyVision},
		{"Movie.2024.2160p.HDR10+.mkv", HDR10Plus},
		{"Movie.2024.2160p.HDR10Plus.mkv", HDR10Plus},
		{"Movie.2024.2160p.HDR10.mkv", HDR10},
		{"Movie.2024.2160p.HDR.mkv", HDR10},
		{"Movie.2024.2160p.HLG.mkv", HLG},
		{"Movie.2024.1080p.BluRay.mkv", HDRNone},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := Parse(tt.filename)
			if got.HDR != tt.want {
				t.Errorf("Parse(%q).HDR = %v, want %v", tt.filename, got.HDR, tt.want)
			}
		})
	}
}

func TestParseAudio(t *testing.T) {
	tests := []struct {
		filename string
		want     AudioCodec
	}{
		{"Movie.2024.TrueHD.Atmos.mkv", AudioAtmos},
		{"Movie.2024.TrueHD.mkv", AudioTrueHD},
		{"Movie.2024.DTS-X.mkv", AudioDTSX},
		{"Movie.2024.DTS-HD.MA.mkv", AudioDTSHDMA},
		{"Movie.2024.DTS-HD.mkv", AudioDTSHD},
		{"Movie.2024.DTS.mkv", AudioDTS},
		{"Movie.2024.DD+.mkv", AudioEAC3},
		{"Movie.2024.DDP5.1.mkv", AudioEAC3},
		{"Movie.2024.AC3.mkv", AudioAC3},
		{"Movie.2024.DD.mkv", AudioAC3},
		{"Movie.2024.AAC.mkv", AudioAAC},
		{"Movie.2024.mkv", AudioUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := Parse(tt.filename)
			if got.Audio != tt.want {
				t.Errorf("Parse(%q).Audio = %v, want %v", tt.filename, got.Audio, tt.want)
			}
		})
	}
}

func TestQualityScoreHierarchy(t *testing.T) {
	// REMUX should beat everything
	remux := Parse("Movie.2024.2160p.REMUX.TrueHD.Atmos.mkv")
	bluray := Parse("Movie.2024.2160p.BluRay.TrueHD.mkv")
	webdl := Parse("Movie.2024.2160p.WEB-DL.DDP5.1.mkv")
	webrip := Parse("Movie.2024.1080p.WEBRip.mkv")
	hdtv := Parse("Movie.2024.720p.HDTV.mkv")

	if !remux.IsBetterThan(bluray) {
		t.Errorf("REMUX should beat BluRay: %d vs %d", remux.Score, bluray.Score)
	}
	if !bluray.IsBetterThan(webdl) {
		t.Errorf("BluRay should beat WEB-DL: %d vs %d", bluray.Score, webdl.Score)
	}
	if !webdl.IsBetterThan(webrip) {
		t.Errorf("WEB-DL should beat WEBRip: %d vs %d", webdl.Score, webrip.Score)
	}
	if !webrip.IsBetterThan(hdtv) {
		t.Errorf("WEBRip should beat HDTV: %d vs %d", webrip.Score, hdtv.Score)
	}
}

func TestResolutionHierarchy(t *testing.T) {
	uhd := Parse("Movie.2024.2160p.WEB-DL.mkv")
	fhd := Parse("Movie.2024.1080p.WEB-DL.mkv")
	hd := Parse("Movie.2024.720p.WEB-DL.mkv")
	sd := Parse("Movie.2024.480p.WEB-DL.mkv")

	if !uhd.IsBetterThan(fhd) {
		t.Errorf("2160p should beat 1080p: %d vs %d", uhd.Score, fhd.Score)
	}
	if !fhd.IsBetterThan(hd) {
		t.Errorf("1080p should beat 720p: %d vs %d", fhd.Score, hd.Score)
	}
	if !hd.IsBetterThan(sd) {
		t.Errorf("720p should beat 480p: %d vs %d", hd.Score, sd.Score)
	}
}

func TestHDRBonus(t *testing.T) {
	hdr := Parse("Movie.2024.2160p.WEB-DL.HDR10.mkv")
	sdr := Parse("Movie.2024.2160p.WEB-DL.mkv")

	if !hdr.IsBetterThan(sdr) {
		t.Errorf("HDR should beat SDR at same resolution: %d vs %d", hdr.Score, sdr.Score)
	}

	dv := Parse("Movie.2024.2160p.WEB-DL.DV.mkv")
	if !dv.IsBetterThan(hdr) {
		t.Errorf("DolbyVision should beat HDR10: %d vs %d", dv.Score, hdr.Score)
	}
}

func TestCompareFiles(t *testing.T) {
	tests := []struct {
		file1 string
		file2 string
		want  int
	}{
		{"Movie.2024.2160p.REMUX.mkv", "Movie.2024.1080p.BluRay.mkv", 1},
		{"Movie.2024.1080p.WEB-DL.mkv", "Movie.2024.2160p.REMUX.mkv", -1},
		{"Movie.2024.1080p.WEB-DL.mkv", "Movie.2024.1080p.WEB-DL.mkv", 0},
	}

	for _, tt := range tests {
		t.Run(tt.file1+" vs "+tt.file2, func(t *testing.T) {
			got := CompareFiles(tt.file1, tt.file2)
			if got != tt.want {
				t.Errorf("CompareFiles(%q, %q) = %d, want %d", tt.file1, tt.file2, got, tt.want)
			}
		})
	}
}

func TestIsBetterFile(t *testing.T) {
	if !IsBetterFile("Movie.2024.2160p.REMUX.mkv", "Movie.2024.1080p.WEB-DL.mkv") {
		t.Error("REMUX 2160p should be better than WEB-DL 1080p")
	}
	if IsBetterFile("Movie.2024.720p.HDTV.mkv", "Movie.2024.1080p.BluRay.mkv") {
		t.Error("HDTV 720p should NOT be better than BluRay 1080p")
	}
}

func TestQualityString(t *testing.T) {
	tests := []struct {
		filename string
		contains []string
	}{
		{"Movie.2024.2160p.REMUX.DV.mkv", []string{"2160p", "REMUX", "DV"}},
		{"Movie.2024.1080p.BluRay.mkv", []string{"1080p", "BluRay"}},
		{"Movie.2024.720p.WEB-DL.HDR10.mkv", []string{"720p", "WEB-DL", "HDR10"}},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := GetQualityString(tt.filename)
			for _, want := range tt.contains {
				if !containsString(got, want) {
					t.Errorf("GetQualityString(%q) = %q, should contain %q", tt.filename, got, want)
				}
			}
		})
	}
}

func TestProperAndExtended(t *testing.T) {
	proper := Parse("Movie.2024.1080p.BluRay.PROPER.mkv")
	regular := Parse("Movie.2024.1080p.BluRay.mkv")

	if !proper.IsProper {
		t.Error("PROPER release should be detected")
	}
	if !proper.IsBetterThan(regular) {
		t.Errorf("PROPER should beat regular: %d vs %d", proper.Score, regular.Score)
	}

	extended := Parse("Movie.2024.1080p.BluRay.EXTENDED.mkv")
	if !extended.IsExtended {
		t.Error("EXTENDED release should be detected")
	}
}

func TestRealWorldFilenames(t *testing.T) {
	tests := []struct {
		filename   string
		wantSource Source
		wantRes    Resolution
	}{
		{"The.Matrix.1999.2160p.UHD.BluRay.REMUX.HDR.HEVC.DTS-HD.MA.7.1-GROUP", SourceREMUX, Resolution2160p},
		{"Inception.2010.1080p.BluRay.x264-SPARKS", SourceBluRay, Resolution1080p},
		{"Dune.Part.Two.2024.2160p.AMZN.WEB-DL.DDP5.1.Atmos.H.265-FLUX", SourceWEBDL, Resolution2160p},
		{"Silo.S02E05.720p.WEBRip.x264-XEN0N", SourceWEBRip, Resolution720p},
		{"Breaking.Bad.S01E01.Pilot.1080p.BluRay.x265.HEVC.10bit.AAC.5.1-GROUP", SourceBluRay, Resolution1080p},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := Parse(tt.filename)
			if got.Source != tt.wantSource {
				t.Errorf("Source: got %v, want %v", got.Source, tt.wantSource)
			}
			if got.Resolution != tt.wantRes {
				t.Errorf("Resolution: got %v, want %v", got.Resolution, tt.wantRes)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
