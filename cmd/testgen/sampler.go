package main

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// Category represents a classification category for releases
type Category string

const (
	CategoryQuality       Category = "quality"
	CategoryStreaming     Category = "streaming"
	CategoryReleaseGroups Category = "release_groups"
	CategoryDateEpisodes  Category = "date_episodes"
	CategoryYearEdge      Category = "year_edge"
	CategorySpecialChars  Category = "special_chars"
	CategoryTVFormats     Category = "tv_formats"
	CategoryMultiPart     Category = "multi_part"
	CategoryForeign       Category = "foreign"
	CategoryObfuscated    Category = "obfuscated"
)

// AllCategories is a list of all defined categories
var AllCategories = []Category{
	CategoryQuality,
	CategoryStreaming,
	CategoryReleaseGroups,
	CategoryDateEpisodes,
	CategoryYearEdge,
	CategorySpecialChars,
	CategoryTVFormats,
	CategoryMultiPart,
	CategoryForeign,
	CategoryObfuscated,
}

// CategoryDescription returns a human-readable description for a category
func CategoryDescription(cat Category) string {
	switch cat {
	case CategoryQuality:
		return "Quality markers (1080p, 2160p, REMUX, WEB-DL, etc.)"
	case CategoryStreaming:
		return "Streaming platforms (NF, AMZN, Hulu, MAX, etc.)"
	case CategoryReleaseGroups:
		return "Release group identifiers"
	case CategoryDateEpisodes:
		return "Date-based episodes (The Daily Show, talk shows)"
	case CategoryYearEdge:
		return "Edge cases with years (multi-year, parentheses, etc.)"
	case CategorySpecialChars:
		return "Special characters and punctuation"
	case CategoryTVFormats:
		return "TV episode formats (S01E01, 1x01, etc.)"
	case CategoryMultiPart:
		return "Multi-part releases (CD1, CD2, Part1, etc.)"
	case CategoryForeign:
		return "Foreign language releases"
	case CategoryObfuscated:
		return "Obfuscated or unusual naming patterns"
	default:
		return "Unknown category"
	}
}

// Sample represents a sampled release
type Sample struct {
	Info       *ReleaseInfo
	Categories []Category
}

// Sampler handles stratified sampling of releases
type Sampler struct {
	mu        sync.RWMutex
	samples   map[Category][]*Sample
	maxPerCat int
	rand      *rand.Rand
}

// NewSampler creates a new sampler with the given configuration
func NewSampler(maxPerCat int, seed int64) *Sampler {
	return &Sampler{
		samples:   make(map[Category][]*Sample),
		maxPerCat: maxPerCat,
		rand:      rand.New(rand.NewSource(seed)),
	}
}

// ClassifyAndSample classifies a release and adds it to samples if appropriate
func (s *Sampler) ClassifyAndSample(info *ReleaseInfo) {
	categories := s.classify(info.ReleaseName, info.Filename)
	if len(categories) == 0 {
		return
	}

	sample := &Sample{
		Info:       info,
		Categories: categories,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, cat := range categories {
		existing := s.samples[cat]
		// Check if we already have this release (by hash)
		alreadySampled := false
		for _, s := range existing {
			if s.Info.Hash == info.Hash {
				alreadySampled = true
				break
			}
		}
		if !alreadySampled {
			s.samples[cat] = append(existing, sample)
		}
	}
}

// classify determines which categories a release belongs to
func (s *Sampler) classify(releaseName, filename string) []Category {
	var categories []Category

	// Check each category
	if containsQualityPattern(releaseName) || containsQualityPattern(filename) {
		categories = append(categories, CategoryQuality)
	}
	if containsStreamingPlatform(releaseName) {
		categories = append(categories, CategoryStreaming)
	}
	if containsReleaseGroup(releaseName) {
		categories = append(categories, CategoryReleaseGroups)
	}
	if containsDatePattern(releaseName) || containsDatePattern(filename) {
		categories = append(categories, CategoryDateEpisodes)
	}
	if containsYearEdgeCase(releaseName) {
		categories = append(categories, CategoryYearEdge)
	}
	if containsSpecialChars(releaseName) || containsSpecialChars(filename) {
		categories = append(categories, CategorySpecialChars)
	}
	if containsTVEpisodePattern(releaseName) || containsTVEpisodePattern(filename) {
		categories = append(categories, CategoryTVFormats)
	}
	if containsMultiPart(releaseName) || containsMultiPart(filename) {
		categories = append(categories, CategoryMultiPart)
	}
	if containsForeign(releaseName) {
		categories = append(categories, CategoryForeign)
	}
	if containsObfuscatedPattern(releaseName) {
		categories = append(categories, CategoryObfuscated)
	}

	return categories
}

// GetSamples returns samples for a specific category, limited to maxPerCat
func (s *Sampler) GetSamples(cat Category) []*Sample {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := s.samples[cat]
	if len(all) <= s.maxPerCat {
		return all
	}

	// Shuffle and return maxPerCat samples
	shuffled := make([]*Sample, len(all))
	copy(shuffled, all)
	s.rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	return shuffled[:s.maxPerCat]
}

// GetAllSamples returns all samples organized by category
func (s *Sampler) GetAllSamples() map[Category][]*Sample {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[Category][]*Sample)
	for cat := range s.samples {
		result[cat] = s.GetSamples(cat)
	}
	return result
}

// GetSummary returns a summary of samples per category
func (s *Sampler) GetSummary() map[Category]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := make(map[Category]int)
	for cat, samples := range s.samples {
		summary[cat] = len(samples)
	}
	return summary
}

// Helper functions

var (
	// Quality patterns
	qualityRegex = regexp.MustCompile(`(?i)\b(2160p|4K|1080p|720p|480p|REMUX|WEB-DL|WEBDL|WEBRip|BluRay|BDRip|DVDRip|HDTV)\b`)
	// Streaming platforms
	streamingRegex = regexp.MustCompile(`(?i)\b(NF|AMZN|ATVP|HULU|DSNP|MAX|PMTP|HBO|CRiT|iTUNES|STAN)\b`)
	// Release groups (common ones)
	releaseGroupRegex = regexp.MustCompile(`(?i)-(RARBG|YTS|YIFY|FLUX|ETHEL|Kitsune|NTb|CMRG|SPARKS|FGT|DIMENSION|SiGMA|NTb|CONTROL|GECKOS|FLiCKSiCK|RARBG|iNCiTE|DiViDi|HoRnEtS|NBS|DONDAN|FmE|DFE|c0nFuSed|CDC|aAF|ALLiANCE|LiNE|PRoDJi|SFiNH|xOr|DEiTY|DoNE|DMT|TDF|BMD|LTER|WB|FoV|LOL|Sys|EVOLVE|QCF|ASAP|IMMERSE|BATV|2HD|KILLERS|REMARKABLE|OVERMATCH|WAFFLES|ANON|TLA|TLA|AFO|CAA|AOC|AOU|AFG|AJP)[\-.]`)
	// Date-based episode pattern (YYYY.MM.DD or YYYY-MM-DD)
	datePatternRegex = regexp.MustCompile(`\b20\d{2}[\.\-]\d{2}[\.\-]\d{2}\b`)
	// Year edge cases
	yearEdgeRegex = regexp.MustCompile(`(?i)(\(?\d{4}[ \-–/&]\d{4}\)?|\d{4}\d{4}|\(19\d{2}\)|\(20\d{2}\))`)
	// Special characters
	specialCharsRegex = regexp.MustCompile(`[<>:"|?*\[\]{}'` + "`" + `]`)
	// TV episode patterns
	tvSEPattern   = regexp.MustCompile(`(?i)\bS\d{1,2}E\d{1,2}\b`)
	tvXPattern    = regexp.MustCompile(`(?i)\b\d{1,2}x\d{1,2}\b`)
	tvPartPattern = regexp.MustCompile(`(?i)\bPt\d{1,2}\b`)
	// Multi-part patterns
	multiPartRegex = regexp.MustCompile(`(?i)[\-. ](CD\d|Part\d{1,2}|Disk\d|Disc\d)[\-. ]`)
	// Foreign language indicators
	foreignRegex = regexp.MustCompile(`(?i)\.(German|French|Spanish|Italian|Japanese|Korean|Russian|Polish|Hindi|Thai|Chinese|Danish|Dutch|Swedish|Norwegian|Finnish|Czech|Greek|Hungarian|Portuguese|Hebrew|Arabic|Turkish|Vietnamese|Bulgarian|Romanian|Slovak|Croatian|Serbian|Ukrainian|Greek|Korean)\.|\b(German|French|Spanish|Italian|Japanese|Korean|Russian|Polish|Hindi|Thai|Chinese|Danish|Dutch|Swedish|Norwegian|Finnish|Czech|Greek|Hungarian|Portuguese|Hebrew|Arabic|Turkish|Vietnamese|Bulgarian|Romanian|Slovak|Croatian|Serbian|Ukrainian)\.(AC3|Dubbed|DUB)\b`)
	// Obfuscated patterns
	obfuscatedRegex = regexp.MustCompile(`(?i)^[A-Z0-9]{8,}$|^[a-z0-9]{8,}$|^[A-Z]{2,5}\d{2,}`)
)

func containsQualityPattern(s string) bool {
	return qualityRegex.MatchString(s)
}

func containsStreamingPlatform(s string) bool {
	return streamingRegex.MatchString(s)
}

func containsReleaseGroup(s string) bool {
	return releaseGroupRegex.MatchString(s)
}

func containsDatePattern(s string) bool {
	return datePatternRegex.MatchString(s)
}

func containsYearEdgeCase(s string) bool {
	return yearEdgeRegex.MatchString(s)
}

func containsSpecialChars(s string) bool {
	return specialCharsRegex.MatchString(s)
}

func containsTVEpisodePattern(s string) bool {
	return tvSEPattern.MatchString(s) || tvXPattern.MatchString(s) || tvPartPattern.MatchString(s)
}

func containsMultiPart(s string) bool {
	return multiPartRegex.MatchString(s)
}

func containsForeign(s string) bool {
	return foreignRegex.MatchString(s)
}

func containsObfuscatedPattern(s string) bool {
	// Check if the release name is very short and alphanumeric (potential obfuscation)
	if len(s) < 10 {
		return false
	}
	// High ratio of non-alphanumeric characters might indicate obfuscation
	specialCount := 0
	for _, ch := range s {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '.' && ch != '-' && ch != '_' {
			specialCount++
		}
	}
	if float64(specialCount)/float64(len(s)) > 0.3 {
		return true
	}
	return obfuscatedRegex.MatchString(s)
}

// Extract functions for detailed analysis

// extractResolution extracts resolution from a release name
func extractResolution(s string) string {
	matches := qualityRegex.FindAllString(s, -1)
	for _, m := range matches {
		if strings.Contains(strings.ToLower(m), "p") {
			return strings.ToLower(m)
		}
		if m == "4K" || m == "2160p" {
			return "2160p"
		}
	}
	return ""
}

// extractSource extracts the source from a release name
func extractSource(s string) string {
	s = strings.ToLower(s)
	sources := []string{"remux", "web-dl", "webdl", "webrip", "bluray", "bdrip", "dvdrip", "hdtv"}
	for _, source := range sources {
		if strings.Contains(s, source) {
			return source
		}
	}
	return ""
}

// extractCodec extracts codec information
func extractCodec(s string) string {
	s = strings.ToLower(s)
	if strings.Contains(s, "x264") || strings.Contains(s, "h.264") {
		return "h264"
	}
	if strings.Contains(s, "x265") || strings.Contains(s, "h.265") || strings.Contains(s, "hevc") {
		return "hevc"
	}
	if strings.Contains(s, "av1") {
		return "av1"
	}
	return ""
}

// extractPlatform extracts streaming platform
func extractPlatform(s string) string {
	matches := streamingRegex.FindAllString(s, -1)
	if len(matches) > 0 {
		return strings.ToUpper(matches[0])
	}
	return ""
}

// extractDatePattern extracts date patterns for episodes
func extractDatePattern(s string) string {
	matches := datePatternRegex.FindAllString(s, -1)
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// extractYearPattern extracts year patterns
func extractYearPattern(s string) []string {
	matches := yearEdgeRegex.FindAllString(s, -1)
	return matches
}

// containsAny checks if the string contains any of the substrings
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(strings.ToLower(s), strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// PrintCategoryList prints all available categories with descriptions
func PrintCategoryList() {
	fmt.Println("Available categories:")
	for _, cat := range AllCategories {
		fmt.Printf("  %-20s - %s\n", cat, CategoryDescription(cat))
	}
}
