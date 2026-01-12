package sync

import (
	"context"
	"path/filepath"
	"time"

	"github.com/Nomadcxx/jellywatch/internal/database"
)

const radarrSourcePriority = 25

// SyncFromRadarr imports movie data from Radarr API
func (s *SyncService) SyncFromRadarr(ctx context.Context) error {
	s.logger.Info("syncing from Radarr")

	logID, err := s.db.StartSyncLog("radarr")
	if err != nil {
		return err
	}

	// Get all movies from Radarr with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	movies, err := s.radarr.GetMovies()
	if err != nil {
		s.db.CompleteSyncLog(logID, "failed", 0, 0, 0, err.Error())
		return err
	}

	var processed, added, updated int

	for _, movie := range movies {
		select {
		case <-ctx.Done():
			s.db.CompleteSyncLog(logID, "failed", processed, added, updated, "context cancelled")
			return ctx.Err()
		default:
		}

		processed++

		record := &database.Movie{
			Title:          movie.Title,
			Year:           movie.Year,
			TmdbID:         &movie.TmdbID,
			RadarrID:       &movie.ID,
			CanonicalPath:  movie.Path,
			LibraryRoot:    filepath.Dir(movie.Path),
			Source:         "radarr",
			SourcePriority: radarrSourcePriority,
		}

		// Set IMDB ID if available
		if movie.ImdbID != "" {
			record.ImdbID = &movie.ImdbID
		}

		// Check if this is new
		existing, _ := s.db.GetMovieByTitle(movie.Title, movie.Year)
		isNew := (existing == nil)

		// UpsertMovie respects source priority - won't overwrite jellywatch paths
		_, err := s.db.UpsertMovie(record)
		if err != nil {
			s.logger.Warn("failed to upsert movie", "title", movie.Title, "error", err)
			continue
		}

		if isNew {
			added++
		} else {
			updated++
		}
	}

	s.db.CompleteSyncLog(logID, "success", processed, added, updated, "")
	s.logger.Info("radarr sync completed", "processed", processed, "added", added, "updated", updated)

	return nil
}
