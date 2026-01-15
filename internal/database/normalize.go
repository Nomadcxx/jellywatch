package database

import (
	"fmt"
	"regexp"
	"strings"
)

// yearPattern matches any parenthesized year (not anchored to end)
// This finds ALL years in parentheses so we can pick the FIRST one
var yearPattern = regexp.MustCompile(`\((\d{4})\)`)

// yearSuffixPattern matches year at the end (for backward compatibility in normalization)
var yearSuffixPattern = regexp.MustCompile(`\s*\((\d{4})\)\s*$`)

// NormalizeTitle converts a title to a normalized form for matching
// "For All Mankind (2019)" -> "forallmankind"
// "M*A*S*H" -> "mash"
func NormalizeTitle(title string) string {
	// Remove year suffix like "(2024)"
	title = yearSuffixPattern.ReplaceAllString(title, "")

	// Lowercase
	title = strings.ToLower(title)

	// Remove common separators and punctuation
	replacements := []string{
		" ", "", ".", "", "-", "", "_", "",
		"'", "", ":", "", "&", "", "*", "",
		",", "", "!", "", "?", "",
		"(", "", ")", "",
		"[", "", "]", "",
	}

	replacer := strings.NewReplacer(replacements...)
	title = replacer.Replace(title)

	return title
}

// ExtractYear attempts to extract a year from a title string using smart logic:
//
// 1. Multiple years: return FIRST (premiere year) - "Show (2015) (2025)" -> 2015
// 2. Single year at END: return it - "Fallout (2024)" -> 2024
// 3. Single year NOT at end: return 0 - "Star Trek (2009) Remastered" -> 0
//
// The third case handles titles where the year is part of the name, not a suffix.
//
// Bug fix: JELLYWATCH_BUG_REPORT_OverAggressive_Year_Stripping_TV_Show_Names.md
func ExtractYear(title string) int {
	// Find ALL parenthesized years
	matches := yearPattern.FindAllStringSubmatch(title, -1)
	if len(matches) == 0 {
		return 0
	}

	// Multiple years: extract the FIRST one (premiere year)
	if len(matches) > 1 {
		for _, match := range matches {
			if len(match) >= 2 {
				var year int
				fmt.Sscanf(match[1], "%d", &year)
				if year >= 1900 && year <= 2100 {
					return year
				}
			}
		}
		return 0
	}

	// Single year: only extract if it's at the END of the string
	// This handles cases like "Star Trek (2009) Remastered" where the year is part of the title
	if yearSuffixPattern.MatchString(title) {
		match := yearSuffixPattern.FindStringSubmatch(title)
		if len(match) >= 2 {
			var year int
			fmt.Sscanf(match[1], "%d", &year)
			if year >= 1900 && year <= 2100 {
				return year
			}
		}
	}
	return 0
}

// StripYear removes a year from a title using smart logic:
//
// 1. Multiple years: strip the FIRST one - "Show (2015) (2025)" -> "Show (2025)"
// 2. Single year at END: strip it - "For All Mankind (2019)" -> "For All Mankind"
// 3. Single year NOT at end: don't strip - "Star Trek (2009) Remastered" -> unchanged
//
// Bug fix: JELLYWATCH_BUG_REPORT_OverAggressive_Year_Stripping_TV_Show_Names.md
func StripYear(title string) string {
	// Find ALL parenthesized years
	matches := yearPattern.FindAllStringSubmatchIndex(title, -1)
	if len(matches) == 0 {
		return title
	}

	// Multiple years: strip the FIRST one
	if len(matches) > 1 {
		match := matches[0]
		yearStr := title[match[2]:match[3]]
		var year int
		fmt.Sscanf(yearStr, "%d", &year)
		if year < 1900 || year > 2100 {
			return title
		}

		// Remove the first year occurrence (including surrounding spaces)
		before := strings.TrimRight(title[:match[0]], " ")
		after := strings.TrimLeft(title[match[1]:], " ")

		if after == "" {
			return before
		}
		return before + " " + after
	}

	// Single year: only strip if it's at the END of the string
	if !yearSuffixPattern.MatchString(title) {
		return title // Year is not at end, don't strip (it's part of the title)
	}

	return yearSuffixPattern.ReplaceAllString(title, "")
}
