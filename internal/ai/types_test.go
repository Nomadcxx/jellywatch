package ai

import (
	"testing"
	"time"
)

func TestParseSource_String(t *testing.T) {
	tests := []struct {
		source   ParseSource
		expected string
	}{
		{SourceCache, "cache"},
		{SourceRegex, "regex"},
		{SourceAI, "ai"},
	}

	for _, tt := range tests {
		if got := tt.source.String(); got != tt.expected {
			t.Errorf("ParseSource(%d).String() = %q, want %q", tt.source, got, tt.expected)
		}
	}
}

func TestMetrics_RecordParse(t *testing.T) {
	m := &Metrics{}

	m.RecordParse(SourceCache, 0)
	m.RecordParse(SourceRegex, 0)
	m.RecordParse(SourceAI, 150*time.Millisecond)
	m.RecordParse(SourceAI, 100*time.Millisecond)

	if got := m.TotalParsings.Load(); got != 4 {
		t.Errorf("TotalParsings = %d, want 4", got)
	}
	if got := m.CacheHits.Load(); got != 1 {
		t.Errorf("CacheHits = %d, want 1", got)
	}
	if got := m.RegexUsed.Load(); got != 1 {
		t.Errorf("RegexUsed = %d, want 1", got)
	}
	if got := m.AIUsed.Load(); got != 2 {
		t.Errorf("AIUsed = %d, want 2", got)
	}
	if got := m.TotalAILatency.Load(); got != 250 {
		t.Errorf("TotalAILatency = %d, want 250", got)
	}
}

func TestMetrics_Summary(t *testing.T) {
	m := &Metrics{}

	// Empty metrics
	summary := m.Summary()
	if summary["total"].(int64) != 0 {
		t.Error("Empty metrics should return total=0")
	}

	// With data
	m.RecordParse(SourceCache, 0)
	m.RecordParse(SourceCache, 0)
	m.RecordParse(SourceRegex, 0)
	m.RecordParse(SourceAI, 100*time.Millisecond)

	summary = m.Summary()
	if summary["total"].(int64) != 4 {
		t.Errorf("total = %v, want 4", summary["total"])
	}
	if summary["cache_hit_rate"].(float64) != 50.0 {
		t.Errorf("cache_hit_rate = %v, want 50.0", summary["cache_hit_rate"])
	}
}
