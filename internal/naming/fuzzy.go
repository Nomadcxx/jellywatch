package naming

import (
	"regexp"
	"strings"
)

var (
	// Streaming platform patterns (case-insensitive)
	streamingPlatformPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(NF|Netflix)\b`),
		regexp.MustCompile(`(?i)\b(AMZN|Amazon)\b`),
		regexp.MustCompile(`(?i)\b(DSNP|Disney\+?|DisneyPlus)\b`),
		regexp.MustCompile(`(?i)\b(HMAX|HBO\s?Max|HBO)\b`),
		regexp.MustCompile(`(?i)\b(HULU)\b`),
		regexp.MustCompile(`(?i)\b(ATVP|Apple\+?|ApplePlus|Apple\s?TV\+?)\b`),
		regexp.MustCompile(`(?i)\b(PCOK|Peacock)\b`),
		regexp.MustCompile(`(?i)\b(PMTP|Paramount\+?)\b`),
		regexp.MustCompile(`(?i)\b(CBS|CBSA)\b`),
		regexp.MustCompile(`(?i)\b(SHOC|Showtime)\b`),
		regexp.MustCompile(`(?i)\b(STAN)\b`),
		regexp.MustCompile(`(?i)\b(FX|FXP)\b`),
		regexp.MustCompile(`(?i)\b(Crunchyroll|CR)\b`),
		regexp.MustCompile(`(?i)\b(Funimation)\b`),
		regexp.MustCompile(`(?i)\b(Hulu|HuluPlus)\b`),
	}

	// Extended audio format patterns
	extendedAudioPatterns = []*regexp.Regexp{
		// DTS variants (most specific first)
		regexp.MustCompile(`(?i)\b(DTS-HD\s?MA)\b`),
		regexp.MustCompile(`(?i)\b(DTS-HD\s?HRA)\b`),
		regexp.MustCompile(`(?i)\b(DTS-HD)\b`),
		regexp.MustCompile(`(?i)\b(DTS-X)\b`),
		regexp.MustCompile(`(?i)\b(DTS-ES)\b`),
		regexp.MustCompile(`(?i)\b(DTS)\b`),

		// Dolby variants
		regexp.MustCompile(`(?i)\b(TrueHD)\b`),
		regexp.MustCompile(`(?i)\b(Atmos)\b`),
		regexp.MustCompile(`(?i)\b(Dolby\s?Atmos)\b`),
		regexp.MustCompile(`(?i)\b(Dolby\s?TrueHD)\b`),
		regexp.MustCompile(`(?i)\b(DDP|DD\+?|Dolby\s?Digital\s?Plus)\b`),
		regexp.MustCompile(`(?i)\b(EAC3|E-AC-3)\b`),
		regexp.MustCompile(`(?i)\b(AC3|AC-3)\b`),

		// Other lossless/lossy formats
		regexp.MustCompile(`(?i)\b(FLAC)\b`),
		regexp.MustCompile(`(?i)\b(PCM|LPCM)\b`),
		regexp.MustCompile(`(?i)\b(Opus)\b`),
		regexp.MustCompile(`(?i)\b(MP3)\b`),
		regexp.MustCompile(`(?i)\b(AAC)\b`),

		// Audio channel patterns (orphaned after codec removal)
		regexp.MustCompile(`(?i)\b\d\s\d\b`),     // "5 1", "7 1"
		regexp.MustCompile(`(?i)\b\d\.\d\b`),     // "5.1", "7.1"
	}

	// Edition/commentary markers
	editionMarkerPatterns = []*regexp.Regexp{
		// Edition markers
		regexp.MustCompile(`(?i)\b(Directors?\s?Cut|Director'?s\s?Cut|DC)\b`),
		regexp.MustCompile(`(?i)\b(IMAX\s?Enhanced|IMAX)\b`),
		regexp.MustCompile(`(?i)\b(UNCUT|Unrated|Uncensored)\b`),
		regexp.MustCompile(`(?i)\b(Extended|Extended\s?Edition)\b`),
		regexp.MustCompile(`(?i)\b(Theatrical|Theatrical\s?Cut|Theatrical\s?Edition)\b`),
		regexp.MustCompile(`(?i)\b(Criterion|Criterion\s?Collection)\b`),
		regexp.MustCompile(`(?i)\b(Remastered|REMASTERED)\b`),
		regexp.MustCompile(`(?i)\b(Alternate\s?Cut|Alt\s?Cut)\b`),
		regexp.MustCompile(`(?i)\b(Restored)\b`),
		regexp.MustCompile(`(?i)\b(Special\s?Edition|SE)\b`),
		regexp.MustCompile(`(?i)\b(Ultimate\s?Edition|UE)\b`),
		regexp.MustCompile(`(?i)\b(Collector'?s\s?Edition)\b`),
		regexp.MustCompile(`(?i)\b(LIMITED|LiMiTED)\b`),

		// Commentary markers
		regexp.MustCompile(`(?i)\b(Commentary|Audio\s?Commentary|With\s?Commentary)\b`),
		regexp.MustCompile(`(?i)\b(Plus\s?Commentary|Extended\s?Commentary)\b`),

		// Other content markers
		regexp.MustCompile(`(?i)\b(EXTRAS|Bonus\s?Features)\b`),

		// Standalone edition/cut words (after specific patterns)
		regexp.MustCompile(`(?i)\bEdition\b`),
		regexp.MustCompile(`(?i)\bCut\b`),
	}

	// Hyphen-suffix patterns (release groups attached with hyphen)
	hyphenSuffixPatterns = []*regexp.Regexp{
		regexp.MustCompile(`-[A-Za-z0-9]+$`),           // -GROUP, -x264, etc.
		regexp.MustCompile(`-\d+$`),                     // -1, -2 (version numbers)
		regexp.MustCompile(`-[A-Z]{2,5}$`),              // -YTS, -RARBG
		regexp.MustCompile(`-[a-z]{2,5}-[a-z]+$`),       // -web-dl, -bluray
		regexp.MustCompile(`-v\d+$`),                    // -v2, -v3
	}
)

// StripStreamingPlatforms removes streaming platform identifiers from name.
// Handles: NF, Netflix, AMZN, Amazon, DSNP, Disney+, HMAX, HBO Max, HULU, ATVP, Apple+, etc.
func StripStreamingPlatforms(name string) string {
	for _, re := range streamingPlatformPatterns {
		name = re.ReplaceAllString(name, " ")
	}
	return collapseSpaces(name)
}

// StripExtendedAudio removes extended audio format markers from name.
// Handles: DTS-HD MA, TrueHD Atmos, DDP, EAC3, FLAC, PCM, Opus, MP3, AAC, Atmos, etc.
func StripExtendedAudio(name string) string {
	for _, re := range extendedAudioPatterns {
		name = re.ReplaceAllString(name, " ")
	}
	return collapseSpaces(name)
}

// StripEditionMarkers removes edition and commentary markers from name.
// Handles: Director's Cut, IMAX Enhanced, UNCUT, Extended, Theatrical, Criterion, Commentary, etc.
func StripEditionMarkers(name string) string {
	for _, re := range editionMarkerPatterns {
		name = re.ReplaceAllString(name, " ")
	}
	return collapseSpaces(name)
}

// StripHyphenSuffixes removes release group names and other suffixes attached with hyphens.
// Example: "Movie.Name-Group" -> "Movie Name"
func StripHyphenSuffixes(name string) string {
	for _, re := range hyphenSuffixPatterns {
		name = re.ReplaceAllString(name, " ")
	}

	// Also handle cases where dots/underscores are used instead of hyphens
	// e.g., "Movie.Name.Group" -> "Movie Name"
	// Remove trailing known release group patterns
	name = stripTrailingReleaseGroup(name)

	return collapseSpaces(name)
}

// stripTrailingReleaseGroup removes trailing release group tokens
// This is a helper for StripHyphenSuffixes
func stripTrailingReleaseGroup(name string) string {
	// Common release group tokens that appear at the end
	trailingGroups := []string{
		"rarbg", "yts", "yify", "flux", "ethel", "kitsune", "ntb", "cmrg", "sparks", "fgt",
		"mag", "psychd", "mircrew", "mirc", "will1869", "aspide", "cinemix",
		"x264", "x265", "h264", "h265", "hevc", "avc",
		"group", "remux",
		"v2", "v3", "v4",
	}

	words := strings.Fields(name)
	for len(words) > 1 {
		last := strings.ToLower(words[len(words)-1])
		found := false
		for _, group := range trailingGroups {
			if last == group {
				words = words[:len(words)-1]
				found = true
				break
			}
		}
		if !found {
			break
		}
	}

	return strings.Join(words, " ")
}

// IsStreamingOnly checks if a name contains ONLY streaming platform markers
// (useful for detecting streaming-exclusive releases)
func IsStreamingOnly(name string) bool {
	// Extract the streaming platform first
	platform := ExtractStreamingPlatform(name)
	if platform == "" {
		return false
	}

	// Check if the name becomes empty or very short after removing the platform
	withoutPlatform := StripStreamingPlatforms(name)
	withoutPlatform = strings.TrimSpace(withoutPlatform)

	// If very short or empty, it's likely streaming-only
	return len(withoutPlatform) <= 3
}

// ExtractStreamingPlatform extracts the streaming platform name from filename.
// Returns the platform name if found, empty string otherwise.
func ExtractStreamingPlatform(name string) string {
	nameUpper := strings.ToUpper(name)

	// Check in order of specificity (most specific first)
	if strings.Contains(nameUpper, "DISNEY+") || strings.Contains(nameUpper, "DISNEYPLUS") || strings.Contains(nameUpper, "DSNP") {
		return "Disney+"
	}
	if strings.Contains(nameUpper, "HBO MAX") || strings.Contains(nameUpper, "HBOMAX") || strings.Contains(nameUpper, "HMAX") {
		return "HBO Max"
	}
	if strings.Contains(nameUpper, "APPLE+") || strings.Contains(nameUpper, "APPLEPLUS") || strings.Contains(nameUpper, "APPLE TV+") || strings.Contains(nameUpper, "ATVP") {
		return "Apple TV+"
	}
	if strings.Contains(nameUpper, "AMAZON") || strings.Contains(nameUpper, "AMZN") {
		return "Amazon Prime"
	}
	if strings.Contains(nameUpper, "PEACOCK") || strings.Contains(nameUpper, "PCOK") {
		return "Peacock"
	}
	if strings.Contains(nameUpper, "PARAMOUNT+") || strings.Contains(nameUpper, "PARAMOUNT") || strings.Contains(nameUpper, "PMTP") {
		return "Paramount+"
	}
	if strings.Contains(nameUpper, "NETFLIX") || strings.Contains(nameUpper, "NF") {
		return "Netflix"
	}
	if strings.Contains(nameUpper, "HULU") {
		return "Hulu"
	}
	if strings.Contains(nameUpper, "SHOWTIME") || strings.Contains(nameUpper, "SHOC") {
		return "Showtime"
	}
	if strings.Contains(nameUpper, "CBS") || strings.Contains(nameUpper, "CBSA") {
		return "CBS All Access"
	}
	if strings.Contains(nameUpper, "CRUNCHYROLL") || strings.Contains(nameUpper, "CR") {
		return "Crunchyroll"
	}
	if strings.Contains(nameUpper, "FUNIMATION") {
		return "Funimation"
	}

	return ""
}

// collapseSpaces collapses multiple spaces into a single space and trims
// Pre-compiled regex for performance (called 4x per filename parse)
var collapseSpacesFuzzyRegex = regexp.MustCompile(`\s+`)

func collapseSpaces(s string) string {
	return strings.TrimSpace(collapseSpacesFuzzyRegex.ReplaceAllString(s, " "))
}
