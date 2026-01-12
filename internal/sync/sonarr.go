package sync

import (
	"context"
	"path/filepath"
	"time"

	"github.com/Nomadcxx/jellywatch/internal/database"
)

const sonarrSourcePriority = 25

// SyncFromSonarr imports series data from Sonarr API
func (s *SyncService) SyncFromSonarr(ctx context.Context) error {
	s.logger.Info("syncing from Sonarr")

	logID, err := s.db.StartSyncLog("sonarr")
	if err != nil {
		return err
	}

	// Get all series from Sonarr with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	series, err := s.sonarr.GetAllSeries()
	if err != nil {
		s.db.CompleteSyncLog(logID, "failed", 0, 0, 0, err.Error())
		return err
	}

	var processed, added, updated int

	for _, show := range series {
		select {
		case <-ctx.Done():
			s.db.CompleteSyncLog(logID, "failed", processed, added, updated, "context cancelled")
			return ctx.Err()
		default:
		}

		processed++

		// Extract episode count from statistics
		episodeCount := 0
		if show.Statistics != nil {
			episodeCount = show.Statistics.EpisodeFileCount
		}

		record := &database.Series{
			Title:          show.Title,
			Year:           show.Year,
			TvdbID:         &show.TvdbID,
			SonarrID:       &show.ID,
			CanonicalPath:  show.Path,
			LibraryRoot:    filepath.Dir(show.Path),
			Source:         "sonarr",
			SourcePriority: sonarrSourcePriority,
			EpisodeCount:   episodeCount,
		}

		// Set IMDB ID if available
		if show.ImdbID != "" {
			record.ImdbID = &show.ImdbID
		}

		// Check if this is new
		existing, _ := s.db.GetSeriesByTitle(show.Title, show.Year)
		isNew := (existing == nil)

		// UpsertSeries respects source priority - won't overwrite jellywatch paths
		_, err := s.db.UpsertSeries(record)
		if err != nil {
			s.logger.Warn("failed to upsert series", "title", show.Title, "error", err)
			continue
		}

		if isNew {
			added++
		} else {
			updated++
		}
	}

	s.db.CompleteSyncLog(logID, "success", processed, added, updated, "")
	s.logger.Info("sonarr sync completed", "processed", processed, "added", added, "updated", updated)

	return nil
}
