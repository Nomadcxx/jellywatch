package consolidate

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/radarr"
	"github.com/Nomadcxx/jellywatch/internal/sonarr"
)

// Notifier handles notifications to Sonarr/Radarr after operations
type Notifier struct {
	sonarr *sonarr.Client
	radarr *radarr.Client
	logger *slog.Logger
	dryRun bool
}

// NewNotifier creates a notifier from config
func NewNotifier(cfg *config.Config, logger *slog.Logger, dryRun bool) *Notifier {
	n := &Notifier{
		logger: logger,
		dryRun: dryRun,
	}

	if cfg.Sonarr.Enabled {
		n.sonarr = sonarr.NewClient(sonarr.Config{
			URL:    cfg.Sonarr.URL,
			APIKey: cfg.Sonarr.APIKey,
		})
	}

	if cfg.Radarr.Enabled {
		n.radarr = radarr.NewClient(radarr.Config{
			URL:    cfg.Radarr.URL,
			APIKey: cfg.Radarr.APIKey,
		})
	}

	return n
}

// NotifyFileMove notifies the appropriate *arr service of a file move/rename
func (n *Notifier) NotifyFileMove(oldPath, newPath string, mediaType string) error {
	if n.dryRun {
		n.logger.Info("would notify", "old", oldPath, "new", newPath, "type", mediaType)
		return nil
	}

	switch mediaType {
	case "episode":
		return n.notifySonarrMove(oldPath, newPath)
	case "movie":
		return n.notifyRadarrMove(oldPath, newPath)
	default:
		return fmt.Errorf("unknown media type: %s", mediaType)
	}
}

// notifySonarrMove updates Sonarr with the new file location
func (n *Notifier) notifySonarrMove(oldPath, newPath string) error {
	if n.sonarr == nil {
		n.logger.Debug("sonarr not configured, skipping notification")
		return nil
	}

	// Get the show folder (parent of Season folder)
	showPath := filepath.Dir(filepath.Dir(newPath))

	// Find the series in Sonarr by path
	series, err := n.sonarr.GetSeriesByPath(showPath)
	if err != nil {
		return fmt.Errorf("failed to find series in Sonarr: %w", err)
	}

	if series == nil {
		n.logger.Warn("series not found in Sonarr", "path", showPath)
		return nil
	}

	// Trigger a refresh for this series
	n.logger.Info("triggering Sonarr refresh", "series", series.Title, "id", series.ID)
	_, err = n.sonarr.RefreshSeries(series.ID)
	if err != nil {
		return fmt.Errorf("failed to refresh series in Sonarr: %w", err)
	}

	return nil
}

// notifyRadarrMove updates Radarr with the new file location
func (n *Notifier) notifyRadarrMove(oldPath, newPath string) error {
	if n.radarr == nil {
		n.logger.Debug("radarr not configured, skipping notification")
		return nil
	}

	// Get the movie folder
	moviePath := filepath.Dir(newPath)

	// Find the movie in Radarr by path
	movie, err := n.radarr.GetMovieByPath(moviePath)
	if err != nil {
		return fmt.Errorf("failed to find movie in Radarr: %w", err)
	}

	if movie == nil {
		n.logger.Warn("movie not found in Radarr", "path", moviePath)
		return nil
	}

	// Trigger a refresh for this movie
	n.logger.Info("triggering Radarr refresh", "movie", movie.Title, "id", movie.ID)
	_, err = n.radarr.RefreshMovie(movie.ID)
	if err != nil {
		return fmt.Errorf("failed to refresh movie in Radarr: %w", err)
	}

	return nil
}

// NotifyBulkComplete triggers library scans after bulk operations
func (n *Notifier) NotifyBulkComplete(tvPaths, moviePaths []string) error {
	if n.dryRun {
		n.logger.Info("would trigger bulk scans", "tv_paths", len(tvPaths), "movie_paths", len(moviePaths))
		return nil
	}

	var errs []error

	// Trigger Sonarr rescan for affected series
	if n.sonarr != nil && len(tvPaths) > 0 {
		// Deduplicate to show-level paths
		showPaths := deduplicateShowPaths(tvPaths)
		for _, path := range showPaths {
			series, err := n.sonarr.GetSeriesByPath(path)
			if err != nil || series == nil {
				continue
			}
			if _, err := n.sonarr.RefreshSeries(series.ID); err != nil {
				errs = append(errs, fmt.Errorf("sonarr refresh %s: %w", series.Title, err))
			}
		}
	}

	// Trigger Radarr rescan for affected movies
	if n.radarr != nil && len(moviePaths) > 0 {
		for _, path := range moviePaths {
			movie, err := n.radarr.GetMovieByPath(path)
			if err != nil || movie == nil {
				continue
			}
			if _, err := n.radarr.RefreshMovie(movie.ID); err != nil {
				errs = append(errs, fmt.Errorf("radarr refresh %s: %w", movie.Title, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("some notifications failed: %v", errs)
	}

	return nil
}

// deduplicateShowPaths extracts unique show paths from file paths
func deduplicateShowPaths(filePaths []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, fp := range filePaths {
		// Go up to show folder (past Season folder)
		showPath := filepath.Dir(filepath.Dir(fp))
		if !seen[showPath] {
			seen[showPath] = true
			result = append(result, showPath)
		}
	}

	return result
}
