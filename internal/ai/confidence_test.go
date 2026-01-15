package ai

import (
	"testing"
)

func TestConfidenceCalculator(t *testing.T) {
	calc := NewConfidenceCalculator()

	tests := []struct {
		title    string
		original string
		desc     string
	}{
		{"Test Show", "Test Show (2023)", "normal title with year"},
		{"x264", "x264.1080p.mkv", "release pattern"},
		{"ABC123", "ABC123.mkv", "all caps >4 chars"},
		{"short", "short.mp4", "short word"},
		{"www.test", "www.test.mkv", "garbage prefix"},
		{"Lost", "Lost.S01E01.mkv", "known title"},
		{"unknown", "unknown.mkv", "unknown single word"},
		{"House", "House.S01E01.mkv", "known title"},
		{"Dexter", "Dexter (2006)", "known title with year"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := calc.CalculateTV(tt.title, tt.original)
			t.Logf("%s (%q, %q) = %.2f", tt.desc, tt.title, tt.original, result)

			if result < 0.0 || result > 1.0 {
				t.Errorf("confidence %.2f outside valid range [0.0, 1.0]", result)
			}
		})
	}
}
