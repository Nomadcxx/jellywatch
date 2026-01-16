package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultSourcePath is the default path to archivedFiles
	DefaultSourcePath = "/home/nomadx/Documents/jellysink/archivedFiles"
	// DefaultOutputPath is the default output directory
	DefaultOutputPath = "internal/naming/testdata"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "generate":
		runGenerate()
	case "validate":
		runValidate()
	case "coverage":
		runCoverage()
	case "confidence":
		runConfidence()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`JellyWatch Test Data Generator

Usage: testgen <command> [options]

Commands:
    generate    Generate test data from archivedFiles CSV
    validate    Validate generated test data
    coverage    Show coverage metrics for test data
    confidence  Generate confidence calibration data
    help        Show this help message

Generate Options:
    --source <path>     Source CSV file (default: %s)
    --output <path>     Output directory (default: %s)
    --samples <n>       Number of samples per category (default: 100)
    --categories        List all categories
    --quick             Quick mode: smaller sample sizes for testing

Validate Options:
    --output <path>     Output directory to validate (default: %s)

Coverage Options:
    --output <path>     Output directory to analyze (default: %s)
    --verbose           Show detailed per-category breakdown

Confidence Options:
    --source <path>     Source CSV file (default: %s)
    --output <path>     Output directory (default: %s)
    --samples <n>       Number of samples to generate (default: 1000)

Categories:
    quality           - Quality markers (1080p, 2160p, REMUX, WEB-DL, etc.)
    streaming         - Streaming platforms (NF, AMZN, Hulu, MAX, etc.)
    release_groups    - Release group identifiers
    date_episodes     - Date-based episodes (The Daily Show, talk shows)
    year_edge         - Edge cases with years (multi-year, parentheses, etc.)
    special_chars     - Special characters and punctuation
    tv_formats        - TV episode formats (S01E01, 1x01, etc.)
    multi_part        - Multi-part releases (CD1, CD2, Part1, etc.)
    foreign           - Foreign language releases
    obfuscated        - Obfuscated or unusual naming patterns

Examples:
    # Generate small sample set for testing
    testgen generate --samples 10

    # Generate full test dataset
    testgen generate --samples 100

    # Generate from custom source
    testgen generate --source /path/to/archivedFiles --output custom/output

    # Validate generated data
    testgen validate

    # Check coverage
    testgen coverage --verbose
`, DefaultSourcePath, DefaultOutputPath, DefaultOutputPath, DefaultOutputPath, DefaultSourcePath, DefaultOutputPath)
}

// ensureOutputDir creates the output directory if it doesn't exist
func ensureOutputDir(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	return nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
