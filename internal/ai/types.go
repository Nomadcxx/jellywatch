package ai

import (
	"sync/atomic"
	"time"
)

// ParseSource indicates which parsing method produced the result
type ParseSource int

const (
	SourceCache ParseSource = iota
	SourceRegex
	SourceAI
)

// String returns the string representation of the parse source
func (s ParseSource) String() string {
	switch s {
	case SourceCache:
		return "cache"
	case SourceRegex:
		return "regex"
	case SourceAI:
		return "ai"
	default:
		return "unknown"
	}
}

// Metrics tracks AI integration usage statistics
type Metrics struct {
	TotalParsings  atomic.Int64
	CacheHits      atomic.Int64
	RegexUsed      atomic.Int64
	AIUsed         atomic.Int64
	AIFallbacks    atomic.Int64 // AI called but fell back to regex
	AITimeouts     atomic.Int64
	AIErrors       atomic.Int64
	TotalAILatency atomic.Int64 // Cumulative AI call time (ms)
}

// RecordParse records a parsing operation with its source and latency
func (m *Metrics) RecordParse(source ParseSource, aiLatency time.Duration) {
	m.TotalParsings.Add(1)
	switch source {
	case SourceCache:
		m.CacheHits.Add(1)
	case SourceRegex:
		m.RegexUsed.Add(1)
	case SourceAI:
		m.AIUsed.Add(1)
		m.TotalAILatency.Add(aiLatency.Milliseconds())
	}
}

// RecordAIFallback records when AI was called but fell back to regex
func (m *Metrics) RecordAIFallback() { m.AIFallbacks.Add(1) }

// RecordAITimeout records an AI timeout
func (m *Metrics) RecordAITimeout() { m.AITimeouts.Add(1) }

// RecordAIError records an AI error
func (m *Metrics) RecordAIError() { m.AIErrors.Add(1) }

// Summary returns a map of metrics for display
func (m *Metrics) Summary() map[string]interface{} {
	total := m.TotalParsings.Load()
	if total == 0 {
		return map[string]interface{}{"total": int64(0)}
	}

	aiCalls := m.AIUsed.Load() + m.AIFallbacks.Load()
	avgLatency := int64(0)
	if aiCalls > 0 {
		avgLatency = m.TotalAILatency.Load() / aiCalls
	}

	fallbackRate := float64(0)
	if aiCalls > 0 {
		fallbackRate = float64(m.AIFallbacks.Load()) / float64(aiCalls) * 100
	}

	return map[string]interface{}{
		"total":             total,
		"cache_hit_rate":    float64(m.CacheHits.Load()) / float64(total) * 100,
		"regex_rate":        float64(m.RegexUsed.Load()) / float64(total) * 100,
		"ai_rate":           float64(m.AIUsed.Load()) / float64(total) * 100,
		"ai_fallback_rate":  fallbackRate,
		"ai_avg_latency_ms": avgLatency,
		"ai_timeouts":       m.AITimeouts.Load(),
		"ai_errors":         m.AIErrors.Load(),
	}
}
