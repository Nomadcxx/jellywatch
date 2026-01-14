package consolidate

import (
	"fmt"
	"syscall"

	"github.com/Nomadcxx/jellywatch/internal/database"
)

// TargetSelector determines the best consolidation target for media
type TargetSelector struct {
	db        *database.MediaDB
	libraries []string
}

// NewTargetSelector creates a new target selector
func NewTargetSelector(db *database.MediaDB, libraries []string) *TargetSelector {
	return &TargetSelector{
		db:        db,
		libraries: libraries,
	}
}

// SelectTVTarget selects the best library for a TV series
// Priority: Drive with most existing episodes for this show
// Fallback: Drive with most free space
func (t *TargetSelector) SelectTVTarget(normalizedTitle string, year int) (string, error) {
	// Find all locations for this series
	locations, err := t.db.FindSeriesLocations(normalizedTitle, year)
	if err != nil {
		return "", fmt.Errorf("failed to find series locations: %w", err)
	}

	if len(locations) == 0 {
		// New series - pick library with most free space
		return t.selectByFreeSpace()
	}

	// Find library with most episodes
	var bestLibrary string
	var maxEpisodes int

	for _, loc := range locations {
		count, err := t.db.CountEpisodesInLibrary(loc, normalizedTitle, year)
		if err != nil {
			continue
		}
		if count > maxEpisodes {
			maxEpisodes = count
			bestLibrary = loc
		}
	}

	if bestLibrary == "" {
		return t.selectByFreeSpace()
	}

	return bestLibrary, nil
}

// SelectMovieTarget selects the best library for a movie
// Priority: Drive where best quality file already exists
// Fallback: Drive with most free space
func (t *TargetSelector) SelectMovieTarget(normalizedTitle string, year int) (string, error) {
	// Find best quality file for this movie
	bestFile, err := t.db.GetBestMovieFile(normalizedTitle, year)
	if err != nil {
		return "", fmt.Errorf("failed to find best movie file: %w", err)
	}

	if bestFile != nil && bestFile.LibraryRoot != "" {
		return bestFile.LibraryRoot, nil
	}

	// No existing file - pick library with most free space
	return t.selectByFreeSpace()
}

// selectByFreeSpace returns the library with the most available space
func (t *TargetSelector) selectByFreeSpace() (string, error) {
	var bestLibrary string
	var maxFree uint64

	for _, lib := range t.libraries {
		free, err := getFreeSpace(lib)
		if err != nil {
			continue
		}
		if free > maxFree {
			maxFree = free
			bestLibrary = lib
		}
	}

	if bestLibrary == "" {
		return "", fmt.Errorf("no accessible libraries found")
	}

	return bestLibrary, nil
}

// getFreeSpace returns available bytes on the filesystem containing path
func getFreeSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}

// LibrarySpace contains free space information for a library
type LibrarySpace struct {
	Path      string
	FreeBytes uint64
	FreeGB    float64
	UsedPct   float64
}

// GetLibrarySpaces returns space information for all configured libraries
func (t *TargetSelector) GetLibrarySpaces() ([]LibrarySpace, error) {
	var spaces []LibrarySpace

	for _, lib := range t.libraries {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(lib, &stat); err != nil {
			continue
		}

		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize)
		used := total - free
		usedPct := float64(used) / float64(total) * 100

		spaces = append(spaces, LibrarySpace{
			Path:      lib,
			FreeBytes: free,
			FreeGB:    float64(free) / (1024 * 1024 * 1024),
			UsedPct:   usedPct,
		})
	}

	return spaces, nil
}
