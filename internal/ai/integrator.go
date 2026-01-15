package ai

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	maxConcurrentAICalls = 4
	maxConcurrentWarms   = 8
)

// SQLDatabase is an interface for types that provide a *sql.DB connection
type SQLDatabase interface {
	DB() *sql.DB
}

// Integrator connects AI, cache, and regex parsing
type Integrator struct {
	enabled         bool
	matcher         *Matcher
	cache           *Cache
	regexConfidence *ConfidenceCalculator
	config          Config
	metrics         *Metrics

	sfGroup    singleflight.Group
	aiSem      chan struct{}
	warmWg     sync.WaitGroup
	warmSem    chan struct{}
	shutdownCh chan struct{}
}

// NewIntegrator creates a new AI integrator
func NewIntegrator(cfg Config, db interface{}) (*Integrator, error) {
	shutdownCh := make(chan struct{})

	if !cfg.Enabled {
		return &Integrator{
			enabled:         false,
			regexConfidence: NewConfidenceCalculator(),
			metrics:         &Metrics{},
			shutdownCh:      shutdownCh,
		}, nil
	}

	matcher, err := NewMatcher(cfg)
	if err != nil {
		log.Printf("[AI] Failed to initialize matcher: %v (degrading to regex-only)", err)
		return &Integrator{
			enabled:         false,
			regexConfidence: NewConfidenceCalculator(),
			metrics:         &Metrics{},
			shutdownCh:      shutdownCh,
		}, nil
	}

	var cache *Cache
	if cfg.CacheEnabled {
		if sqlDB, ok := db.(SQLDatabase); ok {
			cache = NewCache(sqlDB.DB())
		}
	}

	return &Integrator{
		enabled:         true,
		matcher:         matcher,
		cache:           cache,
		regexConfidence: NewConfidenceCalculator(),
		config:          cfg,
		metrics:         &Metrics{},
		aiSem:           make(chan struct{}, maxConcurrentAICalls),
		warmSem:         make(chan struct{}, maxConcurrentWarms),
		warmWg:          sync.WaitGroup{},
		shutdownCh:      shutdownCh,
	}, nil
}

type parseResult struct {
	title  string
	source ParseSource
}

// Close gracefully shuts down integrator
func (i *Integrator) Close() error {
	close(i.shutdownCh)
	i.warmWg.Wait()
	return nil
}

// GetMetrics returns the integrator's metrics
func (i *Integrator) GetMetrics() *Metrics {
	return i.metrics
}

// IsEnabled returns whether AI is enabled
func (i *Integrator) IsEnabled() bool {
	return i.enabled
}

// GetConfidenceCalculator returns the confidence calculator for external use
func (i *Integrator) GetConfidenceCalculator() *ConfidenceCalculator {
	return i.regexConfidence
}

// EnhanceTitle uses AI to enhance a regex-parsed title if needed.
// It accepts the regex-parsed title and original filename, and returns
// the enhanced title, parse source, and any error.
// This method does NOT import the naming package - callers pass in the regex result.
func (i *Integrator) EnhanceTitle(regexTitle, filename string, mediaType string) (string, ParseSource, error) {
	if !i.enabled {
		i.metrics.RecordParse(SourceRegex, 0)
		return regexTitle, SourceRegex, nil
	}

	key := mediaType + ":" + filename
	result, err, _ := i.sfGroup.Do(key, func() (interface{}, error) {
		return i.enhanceTitleInternal(regexTitle, filename, mediaType)
	})

	if err != nil {
		return regexTitle, SourceRegex, err
	}

	parsed := result.(*parseResult)
	return parsed.title, parsed.source, nil
}

func (i *Integrator) enhanceTitleInternal(regexTitle, filename, mediaType string) (*parseResult, error) {
	normalized := NormalizeForCache(filename)

	// 1. Check cache
	if i.cache != nil {
		cached, err := i.cache.Get(normalized, mediaType, i.config.Model)
		if err == nil && cached != nil {
			i.metrics.RecordParse(SourceCache, 0)
			return &parseResult{cached.Title, SourceCache}, nil
		}
	}

	// 2. Calculate confidence for regex result
	var regexConf float64
	if mediaType == "tv" {
		regexConf = i.regexConfidence.CalculateTV(regexTitle, filename)
	} else {
		regexConf = i.regexConfidence.CalculateMovie(regexTitle, filename)
	}

	// 3. High confidence - use regex result
	if regexConf >= i.config.ConfidenceThreshold {
		i.metrics.RecordParse(SourceRegex, 0)
		i.scheduleWarmCache(normalized, mediaType, regexTitle)
		return &parseResult{regexTitle, SourceRegex}, nil
	}

	// 4. Low confidence - try AI with concurrency limit
	select {
	case i.aiSem <- struct{}{}:
		defer func() { <-i.aiSem }()
	case <-i.shutdownCh:
		i.metrics.RecordParse(SourceRegex, 0)
		return &parseResult{regexTitle, SourceRegex}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), i.config.Timeout)
	defer cancel()

	start := time.Now()
	aiResult, err := i.matcher.Parse(ctx, filename)
	latency := time.Since(start)

	if err != nil {
		i.metrics.RecordAIError()
		i.metrics.RecordAIFallback()
		i.metrics.RecordParse(SourceRegex, latency)
		return &parseResult{regexTitle, SourceRegex}, nil
	}

	if aiResult.Confidence < i.config.ConfidenceThreshold {
		i.metrics.RecordAIFallback()
		i.metrics.RecordParse(SourceRegex, latency)
		return &parseResult{regexTitle, SourceRegex}, nil
	}

	// 5. Cache AI result
	if i.cache != nil {
		result := &Result{
			Title:      aiResult.Title,
			Type:       mediaType,
			Confidence: aiResult.Confidence,
		}
		_ = i.cache.Put(normalized, mediaType, i.config.Model, result, latency)
	}

	i.metrics.RecordParse(SourceAI, latency)
	return &parseResult{aiResult.Title, SourceAI}, nil
}

func (i *Integrator) scheduleWarmCache(normalized, mediaType, title string) {
	if i.cache == nil {
		return
	}

	select {
	case i.warmSem <- struct{}{}:
		i.warmWg.Add(1)
		go func() {
			defer func() {
				<-i.warmSem
				i.warmWg.Done()
			}()
			select {
			case <-i.shutdownCh:
				return
			default:
				result := &Result{
					Title:      title,
					Type:       mediaType,
					Confidence: 1.0, // Regex result considered high confidence
				}
				_ = i.cache.Put(normalized, mediaType, i.config.Model, result, 0)
			}
		}()
	default:
		// At concurrency limit, skip warming
	}
}
