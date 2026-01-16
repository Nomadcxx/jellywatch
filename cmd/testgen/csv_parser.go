package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

// ReleaseInfo represents a single release from the archivedFiles CSV
type ReleaseInfo struct {
	ReleaseName string
	Filename    string
	Size        int64
	Timestamp   string
	Hash        string
}

// ParseReleaseInfo parses a single line from the archivedFiles CSV
// The format is: ReleaseName,Filename,Size,Timestamp,Hash
// Note: The CSV has variable length records and some lines may be invalid
func ParseReleaseInfo(line []string) (*ReleaseInfo, error) {
	if len(line) < 5 {
		return nil, fmt.Errorf("insufficient fields: %d", len(line))
	}

	info := &ReleaseInfo{
		ReleaseName: strings.TrimSpace(line[0]),
		Filename:    strings.TrimSpace(line[1]),
		Timestamp:   strings.TrimSpace(line[3]),
		Hash:        strings.TrimSpace(line[4]),
	}

	// Parse size
	var err error
	info.Size, err = parseInt64(strings.TrimSpace(line[2]))
	if err != nil {
		return nil, fmt.Errorf("invalid size: %w", err)
	}

	// Validate
	if info.ReleaseName == "" {
		return nil, fmt.Errorf("empty release name")
	}
	if info.Filename == "" {
		return nil, fmt.Errorf("empty filename")
	}
	if info.Size <= 0 {
		return nil, fmt.Errorf("invalid size: %d", info.Size)
	}

	return info, nil
}

// parseInt64 parses an int64 from a string
func parseInt64(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	var result int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid digit: %c", ch)
		}
		result = result*10 + int64(ch-'0')
	}
	return result, nil
}

// ReleaseCallback is called for each valid release info found
type ReleaseCallback func(info *ReleaseInfo) error

// ReadArchivedFiles reads the archivedFiles CSV and calls the callback for each valid release
// It handles:
// - Variable length records
// - Skipping invalid lines
// - Large files efficiently
func ReadArchivedFiles(path string, callback ReleaseCallback) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable length records

	lineNum := 0
	skipped := 0

	for {
		lineNum++
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading line %d: %w", lineNum, err)
		}

		// Skip empty lines
		if len(line) == 0 || (len(line) == 1 && strings.TrimSpace(line[0]) == "") {
			skipped++
			continue
		}

		// Parse the line
		info, err := ParseReleaseInfo(line)
		if err != nil {
			skipped++
			// Don't log every error - too noisy
			if lineNum <= 10 {
				fmt.Printf("Warning: skipping line %d: %v\n", lineNum, err)
			}
			continue
		}

		// Call the callback
		if err := callback(info); err != nil {
			return fmt.Errorf("callback error at line %d: %w", lineNum, err)
		}
	}

	if skipped > 0 {
		fmt.Printf("Skipped %d invalid lines\n", skipped)
	}

	return nil
}

// CountReleases counts the total number of valid releases in the file
func CountReleases(path string) (int, error) {
	count := 0
	err := ReadArchivedFiles(path, func(info *ReleaseInfo) error {
		count++
		return nil
	})
	return count, err
}
