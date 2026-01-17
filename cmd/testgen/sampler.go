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
	seen      map[Category]map[string]bool // O(1) duplicate check
	maxPerCat int
	rand      *rand.Rand
}

// NewSampler creates a new sampler with the given configuration
func NewSampler(maxPerCat int, seed int64) *Sampler {
	return &Sampler{
		samples: make(map[Category][]*Sample),
		seen:    make(map[Category]map[string]bool),
		maxPerCat: maxPerCat,
		rand:      rand.New(rand.NewSource(seed)),
	}
}

// ClassifyAndSample classifies a release and adds it to samples if appropriate
func (s *Sampler) ClassifyAndSample(info *ReleaseInfo) {
	// Filter out non-movie/TV content
	if isSoftwareRelease(info) {
		return
	}
	if isPornRelease(info) {
		return
	}

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
		// Initialize seen map for this category if needed
		if s.seen[cat] == nil {
			s.seen[cat] = make(map[string]bool)
		}

		// O(1) duplicate check
		if !s.seen[cat][info.Hash] {
			s.seen[cat][info.Hash] = true
			s.samples[cat] = append(s.samples[cat], sample)
		}
	}
}

// Precompiled regex patterns for classification
// These are combined into a single regex for efficient one-pass matching
var (
	// Combined regex for fast classification - matches multiple patterns in one pass
	// Using alternation so we can check all common patterns at once
	fastClassifyRegex = regexp.MustCompile(`(?i)(2160p|4K|1080p|720p|480p|REMUX|WEB-DL|WEBDL|WEBRip|BluRay|BDRip|DVDRip|HDTV|NF|AMZN|ATVP|HULU|DSNP|MAX|PMTP|HBO|CRiT|iTUNES|STAN|20\d{2}[\.\-]\d{2}[\.\-]\d{2}|\(?\d{4}[ \-–/&]\d{4}\)?|\d{4}\d{4}|\(19\d{2}\)|\(20\d{2}\)|\bS\d{1,2}E\d{1,2}\b|\b\d{1,2}x\d{1,2}\b|\bPt\d{1,2}\b|[\-. ]CD\d[\-. ]|[\-. ]Part\d{1,2}[\-. ]|[\-. ]Disk\d[\-. ]|[\-. ]Disc\d[\-. ])`)
)

// Release group regex - kept separate as it's more specific
var releaseGroupRegex = regexp.MustCompile(`(?i)-(RARBG|YTS|YIFY|FLUX|ETHEL|Kitsune|NTb|CMRG|SPARKS|FGT|DIMENSION|SiGMA|CONTROL|GECKOS|FLiCKSiCK|iNCiTE|DiViDi|HoRnEtS|NBS|DONDAN|FmE|DFE|c0nFuSed|CDC|aAF|ALLiANCE|LiNE|PRoDJi|SFiNH|xOr|DEiTY|DoNE|DMT|TDF|BMD|LTER|WB|FoV|LOL|Sys|EVOLVE|QCF|ASAP|IMMERSE|BATV|2HD|KILLERS|REMARKABLE|OVERMATCH|WAFFLES|ANON|TLA|AFO|CAA|AOC|AOU|AFG|AJP)[\-.]`)

// Foreign language regex
var foreignRegex = regexp.MustCompile(`(?i)\.(German|French|Spanish|Italian|Japanese|Korean|Russian|Polish|Hindi|Thai|Chinese|Danish|Dutch|Swedish|Norwegian|Finnish|Czech|Greek|Hungarian|Portuguese|Hebrew|Arabic|Turkish|Vietnamese|Bulgarian|Romanian|Slovak|Croatian|Serbian|Ukrainian)\.|\b(German|French|Spanish|Italian|Japanese|Korean|Russian|Polish|Hindi|Thai|Chinese|Danish|Dutch|Swedish|Norwegian|Finnish|Czech|Greek|Hungarian|Portuguese|Hebrew|Arabic|Turkish|Vietnamese|Bulgarian|Romanian|Slovak|Croatian|Serbian|Ukrainian)\.(AC3|Dubbed|DUB)\b`)

// classify determines which categories a release belongs to (optimized single-pass)
func (s *Sampler) classify(releaseName, filename string) []Category {
	var categories []Category

	// Combined check for most patterns (90%+ of classifications)
	combined := releaseName + " " + filename
	if fastClassifyRegex.MatchString(combined) {
		lower := strings.ToLower(combined)

		// Quick substring checks for quality (faster than regex)
		if strings.Contains(lower, "2160p") || strings.Contains(lower, "4k") ||
			strings.Contains(lower, "1080p") || strings.Contains(lower, "720p") ||
			strings.Contains(lower, "480p") || strings.Contains(lower, "remux") ||
			strings.Contains(lower, "web-dl") || strings.Contains(lower, "webdl") ||
			strings.Contains(lower, "webrip") || strings.Contains(lower, "bluray") ||
			strings.Contains(lower, "bdrip") || strings.Contains(lower, "dvdrip") ||
			strings.Contains(lower, "hdtv") {
			categories = append(categories, CategoryQuality)
		}

		// Streaming platforms
		if strings.Contains(lower, ".nf.") || strings.Contains(lower, ".amzn.") ||
			strings.Contains(lower, ".atvp.") || strings.Contains(lower, ".hulu.") ||
			strings.Contains(lower, ".dsnp.") || strings.Contains(lower, ".max.") ||
			strings.Contains(lower, ".pmtp.") || strings.Contains(lower, ".hbo.") ||
			strings.Contains(lower, ".crit.") || strings.Contains(lower, ".itunes.") ||
			strings.Contains(lower, ".stan.") {
			categories = append(categories, CategoryStreaming)
		}

		// Date patterns (YYYY.MM.DD or YYYY-MM-DD)
		if fastDatePattern(lower) {
			categories = append(categories, CategoryDateEpisodes)
		}

		// Year edge cases
		if fastYearEdgePattern(lower) {
			categories = append(categories, CategoryYearEdge)
		}

		// TV formats
		if fastTVPattern(lower) {
			categories = append(categories, CategoryTVFormats)
		}

		// Multi-part
		if strings.Contains(lower, ".cd1") || strings.Contains(lower, ".cd2") ||
			strings.Contains(lower, ".part1.") || strings.Contains(lower, ".part2.") ||
			strings.Contains(lower, ".disk1.") || strings.Contains(lower, ".disc1.") ||
			strings.Contains(lower, "-cd1") || strings.Contains(lower, "-cd2") {
			categories = append(categories, CategoryMultiPart)
		}
	}

	// Release group check (separate regex - more expensive)
	if releaseGroupRegex.MatchString(releaseName) {
		categories = append(categories, CategoryReleaseGroups)
	}

	// Foreign language check (separate regex)
	if foreignRegex.MatchString(releaseName) {
		categories = append(categories, CategoryForeign)
	}

	// Special characters check (manual scan is faster than regex for this case)
	if hasSpecialChars(releaseName) || hasSpecialChars(filename) {
		categories = append(categories, CategorySpecialChars)
	}

	// Obfuscated pattern check
	if isObfuscated(releaseName) {
		categories = append(categories, CategoryObfuscated)
	}

	return categories
}

// Fast inline checks for patterns (avoiding regex where possible)

func fastDatePattern(s string) bool {
	// Look for 20XX.XX.XX or 20XX-XX-XX pattern
	for i := 0; i < len(s)-9; i++ {
		if s[i] == '2' && s[i+1] >= '0' && s[i+1] <= '9' &&
			s[i+2] >= '0' && s[i+2] <= '9' &&
			(s[i+3] == '.' || s[i+3] == '-') &&
			s[i+4] >= '0' && s[i+4] <= '9' &&
			s[i+5] >= '0' && s[i+5] <= '9' &&
			(s[i+6] == '.' || s[i+6] == '-') &&
			s[i+7] >= '0' && s[i+7] <= '9' &&
			s[i+8] >= '0' && s[i+8] <= '9' {
			return true
		}
	}
	return false
}

func fastYearEdgePattern(s string) bool {
	// Look for (19XX) or (20XX) or YYYY/YYYY or YYYY-YYYY
	for i := 0; i < len(s)-6; i++ {
		// Parenthesized years
		if s[i] == '(' && s[i+5] == ')' &&
			((s[i+1] == '1' && s[i+2] == '9') || (s[i+1] == '2' && s[i+2] == '0')) &&
			s[i+3] >= '0' && s[i+3] <= '9' &&
			s[i+4] >= '0' && s[i+4] <= '9' {
			return true
		}
		// Year ranges: YYYY-YYYY or YYYY/YYYY
		if s[i] >= '0' && s[i] <= '9' && s[i+4] >= '0' && s[i+4] <= '9' &&
			(s[i+5] == '-' || s[i+5] == '/' || s[i+5] == '&' || s[i+5] == ' ') {
			return true
		}
	}
	return false
}

func fastTVPattern(s string) bool {
	// Look for S##E## or ##x## patterns
	for i := 0; i < len(s)-5; i++ {
		// S##E##
		if (s[i] == 's' || s[i] == 'S') &&
			s[i+1] >= '0' && s[i+1] <= '9' &&
			s[i+2] >= '0' && s[i+2] <= '9' &&
			(s[i+3] == 'e' || s[i+3] == 'E') &&
			s[i+4] >= '0' && s[i+4] <= '9' &&
			s[i+5] >= '0' && s[i+5] <= '9' {
			return true
		}
		// ##x##
		if i+4 < len(s) &&
			s[i] >= '0' && s[i] <= '9' &&
			s[i+1] >= '0' && s[i+1] <= '9' &&
			(s[i+2] == 'x' || s[i+2] == 'X') &&
			s[i+3] >= '0' && s[i+3] <= '9' &&
			s[i+4] >= '0' && s[i+4] <= '9' {
			return true
		}
	}
	// Pt## pattern
	for i := 0; i < len(s)-3; i++ {
		if (s[i] == 'p' || s[i] == 'P') &&
			(s[i+1] == 't' || s[i+1] == 'T') &&
			s[i+2] >= '0' && s[i+2] <= '9' &&
			s[i+3] >= '0' && s[i+3] <= '9' {
			return true
		}
	}
	return false
}

func hasSpecialChars(s string) bool {
	for _, ch := range s {
		switch ch {
		case '<', '>', ':', '"', '|', '?', '*', '[', ']', '{', '}', '\'', '`':
			return true
		}
	}
	return false
}

func isObfuscated(s string) bool {
	if len(s) < 10 {
		return false
	}
	// Check for very high alphanumeric ratio
	alphanum := 0
	for _, ch := range s {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			alphanum++
		}
	}
	if float64(alphanum)/float64(len(s)) > 0.95 && len(s) > 15 {
		return true
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
	return false
}

// Extract functions for detailed analysis (used by generator)

// extractResolution extracts resolution from a release name
func extractResolution(s string) string {
	lower := strings.ToLower(s)
	if strings.Contains(lower, "2160p") || strings.Contains(lower, "4k") {
		return "2160p"
	}
	if strings.Contains(lower, "1080p") {
		return "1080p"
	}
	if strings.Contains(lower, "720p") {
		return "720p"
	}
	if strings.Contains(lower, "480p") {
		return "480p"
	}
	return ""
}

// extractSource extracts the source from a release name
func extractSource(s string) string {
	lower := strings.ToLower(s)
	sources := []string{"remux", "web-dl", "webdl", "webrip", "bluray", "bdrip", "dvdrip", "hdtv"}
	for _, source := range sources {
		if strings.Contains(lower, source) {
			return source
		}
	}
	return ""
}

// extractCodec extracts codec information
func extractCodec(s string) string {
	lower := strings.ToLower(s)
	if strings.Contains(lower, "x264") || strings.Contains(lower, "h.264") {
		return "h264"
	}
	if strings.Contains(lower, "x265") || strings.Contains(lower, "h.265") || strings.Contains(lower, "hevc") {
		return "hevc"
	}
	if strings.Contains(lower, "av1") {
		return "av1"
	}
	return ""
}

// extractPlatform extracts streaming platform
func extractPlatform(s string) string {
	lower := strings.ToLower(s)
	platforms := []string{"nf", "amzn", "atvp", "hulu", "dsnp", "max", "pmtp", "hbo", "crit", "itunes", "stan"}
	for _, p := range platforms {
		if strings.Contains(lower, "."+p+".") {
			return strings.ToUpper(p)
		}
	}
	return ""
}

// extractDatePattern extracts date patterns for episodes
func extractDatePattern(s string) string {
	if fastDatePattern(s) {
		// Find and return the actual pattern
		for i := 0; i < len(s)-9; i++ {
			if s[i] == '2' && s[i+1] >= '0' && s[i+1] <= '9' &&
				s[i+2] >= '0' && s[i+2] <= '9' &&
				(s[i+3] == '.' || s[i+3] == '-') &&
				s[i+4] >= '0' && s[i+4] <= '9' &&
				s[i+5] >= '0' && s[i+5] <= '9' &&
				(s[i+6] == '.' || s[i+6] == '-') &&
				s[i+7] >= '0' && s[i+7] <= '9' &&
				s[i+8] >= '0' && s[i+8] <= '9' {
				return s[i : i+10]
			}
		}
	}
	return ""
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

// PrintCategoryList prints all available categories with descriptions
func PrintCategoryList() {
	fmt.Println("Available categories:")
	for _, cat := range AllCategories {
		fmt.Printf("  %-20s - %s\n", cat, CategoryDescription(cat))
	}
}

// isSoftwareRelease detects software/0day/course releases that should be excluded
func isSoftwareRelease(info *ReleaseInfo) bool {
	release := strings.ToUpper(info.ReleaseName)
	filename := strings.ToUpper(info.Filename)

	// Check file extensions for software
	softwareExts := []string{".EXE", ".ISO", ".DAA", ".UHA", ".REV", ".RAR", ".ZIP", ".7Z", ".TAR", ".GZ"}
	for _, ext := range softwareExts {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}

	// Software/course keywords in release name
	softwareKeywords := []string{
		"-TUTOR", "-BOOKWARE", "-TUTORiAL", "-COURSE", "-TRAINING",
		"-INSTALL", "-KEYGEN", "-PATCH", "-CRACKED", "-ACTIVATED",
		"-PLUGINS", "-VST", "-AU", "-AAX", "-RTAS", "-VSTI",
		"ISO", "DVD-ISO", "CD-ISO",
		"0DAY", "0DAY",
		"SOFTWARE", "APPZ", "APPLICATION",
		"WINALL", "MACOSX", "LINUX", "UNIX", "BSD",
		"EBOOK", "AUDIOBOOK", "COMIC",
		"FONT", "SAMPLES", "PRESETS", "SAMPLEPACK",
		"TEMPLATE", "THEME", "SCRIPT",
	}

	for _, kw := range softwareKeywords {
		if strings.Contains(release, kw) {
			return true
		}
	}

	// Educational platforms
	eduPlatforms := []string{
		"PLURALSIGHT", "UDACITY", "COURSERA", "LYNDA", "LINKEDIN",
		"UDEMY", "SKILLSHARE", "CODECADEMY", "TREEHOUSE",
		"INFORMATORY", "CBT", "CBTNUGGETS", "LARACASTS",
		"FRONTENDMASTERS", "ACTIONS", "TESTDRIVEN", "PRAGMATIC",
	}

	for _, plat := range eduPlatforms {
		if strings.Contains(release, plat) {
			return true
		}
	}

	// Version patterns typical of software (v1.0.0, 2023.12, etc.)
	// but exclude video resolution patterns
	if strings.Contains(release, ".V") &&
		!strings.Contains(release, "1080P") &&
		!strings.Contains(release, "720P") &&
		!strings.Contains(release, "2160P") &&
		!strings.Contains(release, "480P") {
		// Check for version-like pattern after .V
		vIdx := strings.Index(release, ".V")
		if vIdx+4 < len(release) && release[vIdx+3] >= '0' && release[vIdx+3] <= '9' {
			return true
		}
	}

	// Adobe/Microsoft/Autodesk products
	adobeProducts := []string{"PHOTOSHOP", "ILLUSTRATOR", "PREMIERE", "AFTER", "DREAMWEAVER", "INDESIGN", "ACROBAT"}
	for _, prod := range adobeProducts {
		if strings.Contains(release, prod) && strings.Contains(release, "ADOBE") {
			return true
		}
	}

	microsoftProducts := []string{"OFFICE", "WINDOWS", "VISUAL.STUDIO", "SQL.SERVER", "EXCHANGE", "SHAREPOINT"}
	for _, prod := range microsoftProducts {
		if strings.Contains(release, prod) && strings.Contains(release, "MICROSOFT") {
			return true
		}
	}

	autodeskProducts := []string{"AUTOCAD", "MAYA", "3DS.MAX", "REVIT", "INVENTOR"}
	for _, prod := range autodeskProducts {
		if strings.Contains(release, prod) && strings.Contains(release, "AUTODESK") {
			return true
		}
	}

	return false
}

// isPornRelease detects adult content releases that should be excluded
func isPornRelease(info *ReleaseInfo) bool {
	release := strings.ToUpper(info.ReleaseName)

	// XXX markers (adult content indicator)
	if strings.Contains(release, ".XXX.") || strings.Contains(release, " XXX.") ||
		strings.Contains(release, "-XXX.") || strings.HasPrefix(release, "XXX.") {
		return true
	}

	// JAV (Japanese Adult Video) markers
	if strings.Contains(release, ".JAV.") || strings.Contains(release, ".CENSORED.") ||
		strings.Contains(release, ".UNCENSORED.") {
		// But only if combined with adult indicators (JAV is ambiguous)
		if strings.Contains(release, "JAV") || strings.Contains(release, "EBOD-") ||
			strings.Contains(release, "PPPD-") || strings.Contains(release, "SSIS-") {
			return true
		}
	}

	// Specific adult content studios (very specific patterns)
	adultStudios := []string{
		"TEENFIDELITY.", "KELLYMADISON.", "YOUNGLEGALPORN.",
		"SILOVESME.", "SISLOVESME.", "BROCRUSH.", "MOMLOVES.",
		"FAMILYSTROKES.", "PASSION-HD.", "WEAREHAIRY.", "LOVEHERFEET.",
	}

	for _, studio := range adultStudios {
		if strings.Contains(release, studio) {
			return true
		}
	}

	return false
}
