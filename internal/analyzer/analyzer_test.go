package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsVideoFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"mkv file", "movie.mkv", true},
		{"mp4 file", "movie.mp4", true},
		{"avi file", "movie.avi", true},
		{"m4v file", "movie.m4v", true},
		{"m2ts file", "movie.m2ts", true},
		{"ts file", "movie.ts", true},
		{"txt file", "readme.txt", false},
		{"nfo file", "movie.nfo", false},
		{"rar file", "movie.rar", false},
		{"srt file", "movie.srt", false},
		{"uppercase MKV", "MOVIE.MKV", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isVideoFile(tt.filename); got != tt.want {
				t.Errorf("isVideoFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestIsRARFile(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		want bool
	}{
		{"rar extension", ".rar", true},
		{"r00 extension", ".r00", true},
		{"r01 extension", ".r01", true},
		{"r99 extension", ".r99", true},
		{"zip extension", ".zip", false},
		{"mkv extension", ".mkv", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRARFile(tt.ext); got != tt.want {
				t.Errorf("isRARFile(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestIsSubtitleFile(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		want bool
	}{
		{"srt extension", ".srt", true},
		{"sub extension", ".sub", true},
		{"ass extension", ".ass", true},
		{"vtt extension", ".vtt", true},
		{"idx extension", ".idx", true},
		{"txt extension", ".txt", false},
		{"mkv extension", ".mkv", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSubtitleFile(tt.ext); got != tt.want {
				t.Errorf("isSubtitleFile(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestIsJunkFile(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		file string
		want bool
	}{
		{"txt extension", ".txt", "readme.txt", true},
		{"nfo extension", ".nfo", "movie.nfo", true},
		{"url extension", ".url", "website.url", true},
		{"sfv extension", ".sfv", "movie.sfv", true},
		{"rarbg in name", ".jpg", "RARBG.jpg", true},
		{"yify in name", ".jpg", "YIFY.jpg", true},
		{"readme in name", ".md", "README.md", true},
		{"regular jpg", ".jpg", "poster.jpg", false},
		{"video file", ".mkv", "movie.mkv", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isJunkFile(tt.ext, tt.file); got != tt.want {
				t.Errorf("isJunkFile(%q, %q) = %v, want %v", tt.ext, tt.file, got, tt.want)
			}
		})
	}
}

func TestIsSampleFile(t *testing.T) {
	tests := []struct {
		name string
		file FileInfo
		want bool
	}{
		{
			"small sample file",
			FileInfo{Name: "movie-sample.mkv", Size: 50 * 1024 * 1024},
			true,
		},
		{
			"sample in middle",
			FileInfo{Name: "The.Movie.2024.Sample.mkv", Size: 80 * 1024 * 1024},
			true,
		},
		{
			"large file with sample name",
			FileInfo{Name: "movie-sample.mkv", Size: 200 * 1024 * 1024},
			false,
		},
		{
			"regular movie file",
			FileInfo{Name: "The.Movie.2024.1080p.BluRay.mkv", Size: 8 * 1024 * 1024 * 1024},
			false,
		},
		{
			"small but no sample in name",
			FileInfo{Name: "movie.mkv", Size: 50 * 1024 * 1024},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSampleFile(tt.file); got != tt.want {
				t.Errorf("isSampleFile(%v) = %v, want %v", tt.file, got, tt.want)
			}
		})
	}
}

func TestIsExtraFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"trailer", "Movie.2024-trailer.mkv", true},
		{"featurette", "Movie.2024.featurette.mkv", true},
		{"behind the scenes", "Movie.2024.behind-the-scenes.mkv", true},
		{"deleted scene", "Movie.2024.deleted-scene.mkv", true},
		{"interview", "Movie.2024.interview.mkv", true},
		{"making of", "Movie.2024.making-of.mkv", true},
		{"regular movie", "Movie.2024.1080p.BluRay.mkv", false},
		{"sample file", "Movie.2024-sample.mkv", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isExtraFile(tt.filename); got != tt.want {
				t.Errorf("isExtraFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestMediaTypeString(t *testing.T) {
	tests := []struct {
		mt   MediaType
		want string
	}{
		{MediaTypeUnknown, "Unknown"},
		{MediaTypeMovie, "Movie"},
		{MediaTypeTVEpisode, "TV Episode"},
		{MediaTypeTVSeason, "TV Season"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mt.String(); got != tt.want {
				t.Errorf("MediaType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeFolder(t *testing.T) {
	// Create temp directory structure for testing
	tmpDir := t.TempDir()

	// Create a movie folder structure
	movieDir := filepath.Join(tmpDir, "Movie.2024.1080p.BluRay")
	if err := os.MkdirAll(movieDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create main movie file (large)
	movieFile := filepath.Join(movieDir, "Movie.2024.1080p.BluRay.mkv")
	if err := createFileWithSize(movieFile, 800*1024*1024); err != nil {
		t.Fatal(err)
	}

	// Create sample file (small)
	sampleFile := filepath.Join(movieDir, "Movie.2024-sample.mkv")
	if err := createFileWithSize(sampleFile, 50*1024*1024); err != nil {
		t.Fatal(err)
	}

	// Create junk files
	nfoFile := filepath.Join(movieDir, "Movie.nfo")
	if err := os.WriteFile(nfoFile, []byte("info"), 0644); err != nil {
		t.Fatal(err)
	}

	txtFile := filepath.Join(movieDir, "RARBG.txt")
	if err := os.WriteFile(txtFile, []byte("rarbg"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create subtitle file
	srtFile := filepath.Join(movieDir, "Movie.2024.srt")
	if err := os.WriteFile(srtFile, []byte("subtitles"), 0644); err != nil {
		t.Fatal(err)
	}

	// Analyze the folder
	analysis, err := AnalyzeFolder(movieDir)
	if err != nil {
		t.Fatalf("AnalyzeFolder() error = %v", err)
	}

	// Verify results
	if analysis.MediaType != MediaTypeMovie {
		t.Errorf("MediaType = %v, want Movie", analysis.MediaType)
	}

	if len(analysis.MediaFiles) != 1 {
		t.Errorf("MediaFiles count = %d, want 1", len(analysis.MediaFiles))
	}

	if len(analysis.SampleFiles) != 1 {
		t.Errorf("SampleFiles count = %d, want 1", len(analysis.SampleFiles))
	}

	if len(analysis.JunkFiles) != 2 {
		t.Errorf("JunkFiles count = %d, want 2", len(analysis.JunkFiles))
	}

	if len(analysis.SubtitleFiles) != 1 {
		t.Errorf("SubtitleFiles count = %d, want 1", len(analysis.SubtitleFiles))
	}

	if analysis.MainMediaFile == nil {
		t.Error("MainMediaFile is nil")
	} else if analysis.MainMediaFile.Name != "Movie.2024.1080p.BluRay.mkv" {
		t.Errorf("MainMediaFile.Name = %q, want Movie.2024.1080p.BluRay.mkv", analysis.MainMediaFile.Name)
	}

	if !analysis.HasUsableMedia() {
		t.Error("HasUsableMedia() = false, want true")
	}
}

func TestAnalyzeFolderWithRAR(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a folder with only RAR files (incomplete)
	rarDir := filepath.Join(tmpDir, "Movie.2024.RAR")
	if err := os.MkdirAll(rarDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create RAR files
	for _, ext := range []string{".rar", ".r00", ".r01", ".r02"} {
		rarFile := filepath.Join(rarDir, "movie"+ext)
		if err := os.WriteFile(rarFile, []byte("rar content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	analysis, err := AnalyzeFolder(rarDir)
	if err != nil {
		t.Fatalf("AnalyzeFolder() error = %v", err)
	}

	if len(analysis.RARFiles) != 4 {
		t.Errorf("RARFiles count = %d, want 4", len(analysis.RARFiles))
	}

	if !analysis.IsIncomplete {
		t.Error("IsIncomplete = false, want true")
	}

	if analysis.HasUsableMedia() {
		t.Error("HasUsableMedia() = true, want false for incomplete archive")
	}
}

func TestAnalyzeFolderTVEpisode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a TV episode folder
	tvDir := filepath.Join(tmpDir, "Show.Name.S01E01")
	if err := os.MkdirAll(tvDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create episode file
	episodeFile := filepath.Join(tvDir, "Show.Name.S01E01.1080p.WEB-DL.mkv")
	if err := createFileWithSize(episodeFile, 500*1024*1024); err != nil {
		t.Fatal(err)
	}

	analysis, err := AnalyzeFolder(tvDir)
	if err != nil {
		t.Fatalf("AnalyzeFolder() error = %v", err)
	}

	if analysis.MediaType != MediaTypeTVEpisode {
		t.Errorf("MediaType = %v, want TV Episode", analysis.MediaType)
	}

	if analysis.DetectedTitle == "" {
		t.Error("DetectedTitle is empty")
	}
}

func TestAnalyzeFolderTVSeason(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a TV season folder with multiple episodes
	tvDir := filepath.Join(tmpDir, "Show.Name.S01")
	if err := os.MkdirAll(tvDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create multiple episode files
	for i := 1; i <= 3; i++ {
		episodeFile := filepath.Join(tvDir, "Show.Name.S01E0"+string(rune('0'+i))+".mkv")
		if err := createFileWithSize(episodeFile, 500*1024*1024); err != nil {
			t.Fatal(err)
		}
	}

	analysis, err := AnalyzeFolder(tvDir)
	if err != nil {
		t.Fatalf("AnalyzeFolder() error = %v", err)
	}

	if analysis.MediaType != MediaTypeTVSeason {
		t.Errorf("MediaType = %v, want TV Season", analysis.MediaType)
	}

	if len(analysis.MediaFiles) != 3 {
		t.Errorf("MediaFiles count = %d, want 3", len(analysis.MediaFiles))
	}
}

func TestGetCleanupFiles(t *testing.T) {
	analysis := &FolderAnalysis{
		JunkFiles: []FileInfo{
			{Path: "/path/to/junk1.txt"},
			{Path: "/path/to/junk2.nfo"},
		},
		SampleFiles: []FileInfo{
			{Path: "/path/to/sample.mkv"},
		},
	}

	cleanup := analysis.GetCleanupFiles()

	if len(cleanup) != 3 {
		t.Errorf("GetCleanupFiles() returned %d files, want 3", len(cleanup))
	}
}

func TestFolderAnalysisString(t *testing.T) {
	analysis := &FolderAnalysis{
		Path:      "/test/path",
		MediaType: MediaTypeMovie,
		MainMediaFile: &FileInfo{
			Name: "movie.mkv",
		},
		MediaFiles:  make([]FileInfo, 1),
		SampleFiles: make([]FileInfo, 2),
		RARFiles:    make([]FileInfo, 0),
		JunkFiles:   make([]FileInfo, 3),
	}

	str := analysis.String()

	if str == "" {
		t.Error("String() returned empty string")
	}

	// Verify it contains expected info
	if !contains(str, "/test/path") {
		t.Error("String() doesn't contain path")
	}
	if !contains(str, "Movie") {
		t.Error("String() doesn't contain media type")
	}
	if !contains(str, "movie.mkv") {
		t.Error("String() doesn't contain main file")
	}
}

func TestAnalyzeSingleFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a single movie file
	movieFile := filepath.Join(tmpDir, "Movie.2024.1080p.BluRay.mkv")
	if err := createFileWithSize(movieFile, 800*1024*1024); err != nil {
		t.Fatal(err)
	}

	// Analyze single file (not folder)
	analysis, err := AnalyzeFolder(movieFile)
	if err != nil {
		t.Fatalf("AnalyzeFolder() error = %v", err)
	}

	if analysis.MediaType != MediaTypeMovie {
		t.Errorf("MediaType = %v, want Movie", analysis.MediaType)
	}

	if analysis.MainMediaFile == nil {
		t.Error("MainMediaFile is nil")
	}
}

// Helper function to create a file with specific size (sparse file)
func createFileWithSize(path string, size int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Use truncate to create a sparse file (doesn't actually allocate disk space)
	return f.Truncate(size)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
