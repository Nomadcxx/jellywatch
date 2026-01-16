package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TestRecord represents a test record in JSON format
type TestRecord struct {
	Input    string                 `json:"input"`
	Features map[string]interface{} `json:"features"`
	Expected map[string]interface{} `json:"expected"` // Empty for manual labeling
}

// GoldenFile represents a golden test file
type GoldenFile struct {
	Description string       `json:"description"`
	Category    Category     `json:"category"`
	CreatedAt   string       `json:"created_at"`
	Records     []TestRecord `json:"records"`
}

// runGenerate implements the generate command
func runGenerate() {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	sourcePath := fs.String("source", DefaultSourcePath, "Source CSV file path")
	outputPath := fs.String("output", DefaultOutputPath, "Output directory path")
	sampleCount := fs.Int("samples", 100, "Number of samples per category")
	listCategories := fs.Bool("categories", false, "List all categories")
	quickMode := fs.Bool("quick", false, "Quick mode: smaller sample sizes")

	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Printf("Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if *listCategories {
		PrintCategoryList()
		return
	}

	// Quick mode reduces sample sizes
	if *quickMode {
		*sampleCount = 10
	}

	// Validate source file exists
	if !fileExists(*sourcePath) {
		fmt.Printf("Error: source file not found: %s\n", *sourcePath)
		os.Exit(1)
	}

	// Create output directory
	if err := ensureOutputDir(*outputPath); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generating test data...\n")
	fmt.Printf("  Source: %s\n", *sourcePath)
	fmt.Printf("  Output: %s\n", *outputPath)
	fmt.Printf("  Samples per category: %d\n", *sampleCount)

	// Count total lines first (for progress)
	fmt.Printf("Counting total releases...\n")
	total, err := CountReleases(*sourcePath)
	if err != nil {
		fmt.Printf("Error counting releases: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Total releases in source: %d\n", total)

	// Create sampler
	sampler := NewSampler(*sampleCount, time.Now().UnixNano())

	// Process the file
	fmt.Printf("Processing releases...\n")
	processed := 0
	lastUpdate := time.Now()

	if err := ReadArchivedFiles(*sourcePath, func(info *ReleaseInfo) error {
		processed++
		sampler.ClassifyAndSample(info)

		// Progress update every second
		if time.Since(lastUpdate) > time.Second {
			percent := float64(processed) / float64(total) * 100
			fmt.Printf("  Progress: %d/%d (%.1f%%)\n", processed, total, percent)
			lastUpdate = time.Now()
		}

		return nil
	}); err != nil {
		fmt.Printf("Error processing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Processed %d releases\n", processed)

	// Generate output files
	if err := generateOutput(sampler, *outputPath); err != nil {
		fmt.Printf("Error generating output: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	printSummary(sampler)

	fmt.Printf("\nTest data generated successfully!\n")
	fmt.Printf("Output location: %s\n", *outputPath)
}

// generateOutput creates the golden, samples, confidence, and benchmark directories
func generateOutput(sampler *Sampler, outputPath string) error {
	// Create subdirectories
	dirs := []string{
		filepath.Join(outputPath, "golden"),
		filepath.Join(outputPath, "samples"),
		filepath.Join(outputPath, "confidence"),
		filepath.Join(outputPath, "benchmark"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Get all samples
	allSamples := sampler.GetAllSamples()

	// Generate golden files for each category
	for cat, samples := range allSamples {
		if len(samples) == 0 {
			continue
		}

		golden := GoldenFile{
			Description: CategoryDescription(cat),
			Category:    cat,
			CreatedAt:   time.Now().Format(time.RFC3339),
			Records:     make([]TestRecord, 0, len(samples)),
		}

		for _, sample := range samples {
			record := TestRecord{
				Input: sample.Info.ReleaseName,
				Features: map[string]interface{}{
					"filename":  sample.Info.Filename,
					"size":      sample.Info.Size,
					"timestamp": sample.Info.Timestamp,
					"hash":      sample.Info.Hash,
				},
				Expected: map[string]interface{}{
					"type":         nil, // To be filled manually
					"title":        nil,
					"year":         nil,
					"season":       nil,
					"episode":      nil,
					"quality":      extractResolution(sample.Info.ReleaseName),
					"source":       extractSource(sample.Info.ReleaseName),
					"codec":        extractCodec(sample.Info.ReleaseName),
					"platform":     extractPlatform(sample.Info.ReleaseName),
					"date_pattern": extractDatePattern(sample.Info.ReleaseName),
				},
			}
			golden.Records = append(golden.Records, record)
		}

		// Write golden file
		goldenPath := filepath.Join(outputPath, "golden", fmt.Sprintf("%s.json", cat))
		data, err := json.MarshalIndent(golden, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal golden file: %w", err)
		}
		if err := os.WriteFile(goldenPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write golden file: %w", err)
		}
	}

	// Generate plain text sample files
	for cat, samples := range allSamples {
		if len(samples) == 0 {
			continue
		}

		samplePath := filepath.Join(outputPath, "samples", fmt.Sprintf("%s.txt", cat))
		file, err := os.Create(samplePath)
		if err != nil {
			return fmt.Errorf("failed to create sample file: %w", err)
		}
		defer file.Close()

		for _, sample := range samples {
			fmt.Fprintf(file, "%s\n", sample.Info.ReleaseName)
		}
	}

	// Generate benchmark files (small, medium, large)
	if err := generateBenchmarkFiles(sampler, outputPath); err != nil {
		return fmt.Errorf("failed to generate benchmark files: %w", err)
	}

	return nil
}

// generateBenchmarkFiles creates benchmark input files of different sizes
func generateBenchmarkFiles(sampler *Sampler, outputPath string) error {
	// Collect all unique release names
	allReleases := make(map[string]bool)
	allSamples := sampler.GetAllSamples()
	for _, samples := range allSamples {
		for _, sample := range samples {
			allReleases[sample.Info.ReleaseName] = true
		}
	}

	// Convert to slice and shuffle
	releases := make([]string, 0, len(allReleases))
	for release := range allReleases {
		releases = append(releases, release)
	}
	// Sort for reproducibility
	sort.Strings(releases)

	// Small benchmark: 100 items
	smallCount := 100
	if len(releases) < smallCount {
		smallCount = len(releases)
	}
	smallPath := filepath.Join(outputPath, "benchmark", "small_bench.txt")
	if err := writeBenchmarkFile(smallPath, releases[:smallCount]); err != nil {
		return err
	}

	// Medium benchmark: 1000 items
	mediumCount := 1000
	if len(releases) < mediumCount {
		mediumCount = len(releases)
	}
	mediumPath := filepath.Join(outputPath, "benchmark", "medium_bench.txt")
	if err := writeBenchmarkFile(mediumPath, releases[:mediumCount]); err != nil {
		return err
	}

	// Large benchmark: 10000 items or all if fewer
	largeCount := 10000
	if len(releases) < largeCount {
		largeCount = len(releases)
	}
	largePath := filepath.Join(outputPath, "benchmark", "large_bench.txt")
	if err := writeBenchmarkFile(largePath, releases[:largeCount]); err != nil {
		return err
	}

	return nil
}

func writeBenchmarkFile(path string, releases []string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create benchmark file: %w", err)
	}
	defer file.Close()

	for _, release := range releases {
		fmt.Fprintln(file, release)
	}

	return nil
}

// runValidate implements the validate command
func runValidate() {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	outputPath := fs.String("output", DefaultOutputPath, "Output directory path")

	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Printf("Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Validating test data in: %s\n", *outputPath)

	// Check if output directory exists
	if !fileExists(*outputPath) {
		fmt.Printf("Error: output directory not found: %s\n", *outputPath)
		os.Exit(1)
	}

	// Validate golden files
	goldenDir := filepath.Join(*outputPath, "golden")
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		fmt.Printf("Error reading golden directory: %v\n", err)
		os.Exit(1)
	}

	validFiles := 0
	totalRecords := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(goldenDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", entry.Name(), err)
			continue
		}

		var golden GoldenFile
		if err := json.Unmarshal(data, &golden); err != nil {
			fmt.Printf("Error parsing JSON in %s: %v\n", entry.Name(), err)
			continue
		}

		validFiles++
		totalRecords += len(golden.Records)
		fmt.Printf("  %s: %d records\n", entry.Name(), len(golden.Records))
	}

	fmt.Printf("\nValidation complete:\n")
	fmt.Printf("  Valid files: %d\n", validFiles)
	fmt.Printf("  Total records: %d\n", totalRecords)
}

// runCoverage implements the coverage command
func runCoverage() {
	fs := flag.NewFlagSet("coverage", flag.ExitOnError)
	outputPath := fs.String("output", DefaultOutputPath, "Output directory path")
	verbose := fs.Bool("verbose", false, "Show detailed breakdown")

	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Printf("Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Coverage analysis for: %s\n", *outputPath)

	// Check if output directory exists
	if !fileExists(*outputPath) {
		fmt.Printf("Error: output directory not found: %s\n", *outputPath)
		os.Exit(1)
	}

	// Analyze sample files
	sampleDir := filepath.Join(*outputPath, "samples")
	entries, err := os.ReadDir(sampleDir)
	if err != nil {
		fmt.Printf("Error reading samples directory: %v\n", err)
		os.Exit(1)
	}

	categoryCounts := make(map[Category]int)
	totalSamples := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}

		catName := strings.TrimSuffix(entry.Name(), ".txt")
		path := filepath.Join(sampleDir, entry.Name())

		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", entry.Name(), err)
			continue
		}

		lines := strings.Split(string(data), "\n")
		count := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				count++
			}
		}

		categoryCounts[Category(catName)] = count
		totalSamples += count

		if *verbose {
			fmt.Printf("  %s: %d samples\n", catName, count)
		}
	}

	fmt.Printf("\nCoverage Summary:\n")
	fmt.Printf("  Categories covered: %d/%d\n", len(categoryCounts), len(AllCategories))
	fmt.Printf("  Total samples: %d\n", totalSamples)

	// Show missing categories
	missing := []Category{}
	for _, cat := range AllCategories {
		if categoryCounts[cat] == 0 {
			missing = append(missing, cat)
		}
	}
	if len(missing) > 0 {
		fmt.Printf("  Missing categories: %s\n", missing)
	}
}

// runConfidence implements the confidence command
func runConfidence() {
	fs := flag.NewFlagSet("confidence", flag.ExitOnError)
	sourcePath := fs.String("source", DefaultSourcePath, "Source CSV file path")
	outputPath := fs.String("output", DefaultOutputPath, "Output directory path")
	sampleCount := fs.Int("samples", 1000, "Number of samples to generate")

	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Printf("Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Validate source file exists
	if !fileExists(*sourcePath) {
		fmt.Printf("Error: source file not found: %s\n", *sourcePath)
		os.Exit(1)
	}

	// Create output directory
	confidenceDir := filepath.Join(*outputPath, "confidence")
	if err := ensureOutputDir(confidenceDir); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generating confidence calibration data...\n")
	fmt.Printf("  Target samples: %d\n", *sampleCount)

	// Collect samples for confidence testing
	// We want a mix of easy, medium, and hard cases
	sampler := NewSampler(*sampleCount*3, time.Now().UnixNano()) // Oversample

	if err := ReadArchivedFiles(*sourcePath, func(info *ReleaseInfo) error {
		sampler.ClassifyAndSample(info)
		return nil
	}); err != nil {
		fmt.Printf("Error processing file: %v\n", err)
		os.Exit(1)
	}

	// Get all samples and select a diverse set
	allSamples := sampler.GetAllSamples()

	// Write confidence calibration file
	confPath := filepath.Join(confidenceDir, "calibration.json")
	file, err := os.Create(confPath)
	if err != nil {
		fmt.Printf("Error creating confidence file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	fmt.Fprintln(file, `# Confidence Calibration Data`)
	fmt.Fprintln(file, `# Generated:`, time.Now().Format(time.RFC3339))
	fmt.Fprintln(file, `#`)
	fmt.Fprintln(file, `# This file contains samples for confidence calibration.`)
	fmt.Fprintln(file, `# Format: release_name | expected_type | expected_title | expected_year | difficulty`)
	fmt.Fprintln(file, `#`)
	fmt.Fprintln(file, `# difficulty: 1=easy (obvious patterns), 2=medium (some ambiguity), 3=hard (edge cases)`)

	count := 0
	for _, samples := range allSamples {
		for _, sample := range samples {
			if count >= *sampleCount {
				break
			}
			// Estimate difficulty based on category
			difficulty := "2" // default medium
			for _, cat := range sample.Categories {
				if cat == CategoryObfuscated || cat == CategoryYearEdge || cat == CategorySpecialChars {
					difficulty = "3" // hard
				} else if cat == CategoryQuality || cat == CategoryStreaming {
					difficulty = "1" // easy
				}
			}

			fmt.Fprintf(file, "%s | %s | %s | %s | %s\n",
				sample.Info.ReleaseName,
				"", // To be filled manually
				"", // To be filled manually
				"", // To be filled manually
				difficulty)
			count++
		}
	}

	fmt.Printf("Generated %d confidence calibration samples\n", count)
	fmt.Printf("Output: %s\n", confPath)
}

// printSummary prints a summary of the sampling results
func printSummary(sampler *Sampler) {
	summary := sampler.GetSummary()

	fmt.Printf("\nSampling Summary:\n")

	// Sort categories by count
	cats := make([]Category, 0, len(summary))
	for cat := range summary {
		cats = append(cats, cat)
	}
	sort.Slice(cats, func(i, j int) bool {
		return summary[cats[i]] > summary[cats[j]]
	})

	for _, cat := range cats {
		fmt.Printf("  %-20s %6d samples\n", cat, summary[cat])
	}

	fmt.Printf("\n  %-20s %6d total\n", "Total", countTotalSamples(summary))
}

func countTotalSamples(summary map[Category]int) int {
	total := 0
	for _, count := range summary {
		total += count
	}
	return total
}
