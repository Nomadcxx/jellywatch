// ⚠️  EXPERIMENTAL - AI parsing functions in this file are NOT YET INTEGRATED
// These functions (ParseMovieNameWithAI, ParseTVShowNameWithAI) are prototypes
// and are not currently called by any production code. All parsing uses
// ParseMovieName/ParseTVShowName (regex-based) in naming.go.
package naming

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Nomadcxx/jellywatch/internal/ai"
)

// OptionalMatcher is an optional AI matcher
type OptionalMatcher struct {
	matcher *ai.Matcher
	cache   *ai.Cache
	enabled bool
}

// NewAIMatcher creates an optional AI matcher for naming operations
func NewAIMatcher(matcher *ai.Matcher, cache *ai.Cache, enabled bool) *OptionalMatcher {
	return &OptionalMatcher{
		matcher: matcher,
		cache:   cache,
		enabled: enabled,
	}
}

// ParseMovieNameWithAI parses a movie name using AI with regex fallback
func ParseMovieNameWithAI(filename string, aiMatcher *OptionalMatcher) (*MovieInfo, error) {
	if aiMatcher == nil || !aiMatcher.enabled || aiMatcher.matcher == nil {
		return ParseMovieName(filename)
	}

	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))

	// Try cache first
	if aiMatcher.cache != nil {
		normalized := ai.NormalizeInput(baseName)
		if cached, err := aiMatcher.cache.Get(normalized, "movie", aiMatcher.matcher.GetConfig().Model); err == nil && cached != nil {
			return aiResultToMovieInfo(cached), nil
		}
	}

	// Try AI
	ctx := context.Background()
	result, err := aiMatcher.matcher.Parse(ctx, baseName)
	if err != nil {
		return nil, fmt.Errorf("AI parse failed: %w (falling back to regex)", err)
	}

	// Check confidence threshold
	if result.Confidence < aiMatcher.matcher.GetConfig().ConfidenceThreshold {
		return ParseMovieName(filename)
	}

	// Cache the result
	if aiMatcher.cache != nil {
		normalized := ai.NormalizeInput(baseName)
		aiMatcher.cache.Put(normalized, "movie", aiMatcher.matcher.GetConfig().Model, result, 0)
	}

	return aiResultToMovieInfo(result), nil
}

// ParseTVShowNameWithAI parses a TV show name using AI with regex fallback
func ParseTVShowNameWithAI(filename string, aiMatcher *OptionalMatcher) (*TVShowInfo, error) {
	if aiMatcher == nil || !aiMatcher.enabled || aiMatcher.matcher == nil {
		return ParseTVShowName(filename)
	}

	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))

	// Try cache first
	if aiMatcher.cache != nil {
		normalized := ai.NormalizeInput(baseName)
		if cached, err := aiMatcher.cache.Get(normalized, "tv", aiMatcher.matcher.GetConfig().Model); err == nil && cached != nil {
			return aiResultToTVShowInfo(cached), nil
		}
	}

	// Try AI
	ctx := context.Background()
	result, err := aiMatcher.matcher.Parse(ctx, baseName)
	if err != nil {
		return nil, fmt.Errorf("AI parse failed: %w (falling back to regex)", err)
	}

	// Check confidence threshold
	if result.Confidence < aiMatcher.matcher.GetConfig().ConfidenceThreshold {
		return ParseTVShowName(filename)
	}

	// Cache the result
	if aiMatcher.cache != nil {
		normalized := ai.NormalizeInput(baseName)
		aiMatcher.cache.Put(normalized, "tv", aiMatcher.matcher.GetConfig().Model, result, 0)
	}

	return aiResultToTVShowInfo(result), nil
}

// aiResultToMovieInfo converts AI result to naming.MovieInfo
func aiResultToMovieInfo(result *ai.Result) *MovieInfo {
	year := ""
	if result.Year != nil {
		year = strconv.Itoa(*result.Year)
	}

	return &MovieInfo{
		Title: result.Title,
		Year:  year,
	}
}

// aiResultToTVShowInfo converts AI result to naming.TVShowInfo
func aiResultToTVShowInfo(result *ai.Result) *TVShowInfo {
	year := ""
	if result.Year != nil {
		year = strconv.Itoa(*result.Year)
	}

	season := 0
	if result.Season != nil {
		season = *result.Season
	}

	episode := 0
	if len(result.Episodes) > 0 {
		episode = result.Episodes[0]
	} else if result.AbsoluteEpisode != nil {
		episode = *result.AbsoluteEpisode
	}

	return &TVShowInfo{
		Title:   result.Title,
		Year:    year,
		Season:  season,
		Episode: episode,
	}
}
