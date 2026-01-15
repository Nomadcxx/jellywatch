package ai

import (
	"testing"
)

func TestNormalizeForCache(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Basic normalization
		{"Show.Name.S01E01.1080p.mkv", "show name s01e01 1080p"},
		{"show_name_s01e01_1080p.mkv", "show name s01e01 1080p"},
		{"Show-Name-S01E01-1080p.mkv", "show name s01e01 1080p"},
		{"Show_Name_S01E01.mkv", "show name s01e01"},

		// Path handling
		{"/path/to/Show.Name.S01E01.mkv", "show name s01e01"},
		{"Show.Name.S01E01", "show name s01e01"},

		// Multiple separators
		{"Show..Name__S01E01--1080p.mkv", "show name s01e01 1080p"},
		{"Show_Name___S01E01.mkv", "show name s01e01"},

		// Whitespace handling
		{"  Show  Name  .mkv", "show name"},
		{"Show Name  .mkv", "show name"},

		// No extension
		{"Show.Name.S01E01", "show name s01e01"},

		// Already normalized
		{"show name s01e01", "show name s01e01"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeForCache(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeForCache(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func BenchmarkNormalizeForCache(b *testing.B) {
	filename := "The.Show.Name.2024.S01E01.1080p.WEB-DL.DDP5.1.H.264-GROUP.mkv"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NormalizeForCache(filename)
	}
	// Target: < 5μs
}
