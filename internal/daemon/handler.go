package daemon

import (
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Nomadcxx/jellywatch/internal/naming"
	"github.com/Nomadcxx/jellywatch/internal/notify"
	"github.com/Nomadcxx/jellywatch/internal/organizer"
	"github.com/Nomadcxx/jellywatch/internal/transfer"
	"github.com/Nomadcxx/jellywatch/internal/watcher"
)

type MediaHandler struct {
	organizer     *organizer.Organizer
	notifyManager *notify.Manager
	tvLibraries   []string
	movieLibs     []string
	debounceTime  time.Duration
	pending       map[string]*time.Timer
	mu            sync.Mutex
	dryRun        bool
	stats         *Stats
}

type Stats struct {
	mu               sync.RWMutex
	MoviesProcessed  int64
	TVProcessed      int64
	BytesTransferred int64
	Errors           int64
	LastProcessed    time.Time
	StartTime        time.Time
}

func NewStats() *Stats {
	return &Stats{
		StartTime: time.Now(),
	}
}

func (s *Stats) RecordMovie(bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MoviesProcessed++
	s.BytesTransferred += bytes
	s.LastProcessed = time.Now()
}

func (s *Stats) RecordTV(bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TVProcessed++
	s.BytesTransferred += bytes
	s.LastProcessed = time.Now()
}

func (s *Stats) RecordError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Errors++
}

func (s *Stats) Snapshot() StatsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return StatsSnapshot{
		MoviesProcessed:  s.MoviesProcessed,
		TVProcessed:      s.TVProcessed,
		BytesTransferred: s.BytesTransferred,
		Errors:           s.Errors,
		LastProcessed:    s.LastProcessed,
		Uptime:           time.Since(s.StartTime),
	}
}

type StatsSnapshot struct {
	MoviesProcessed  int64
	TVProcessed      int64
	BytesTransferred int64
	Errors           int64
	LastProcessed    time.Time
	Uptime           time.Duration
}

type MediaHandlerConfig struct {
	TVLibraries   []string
	MovieLibs     []string
	DebounceTime  time.Duration
	DryRun        bool
	Timeout       time.Duration
	Backend       transfer.Backend
	NotifyManager *notify.Manager
}

func NewMediaHandler(cfg MediaHandlerConfig) *MediaHandler {
	if cfg.DebounceTime == 0 {
		cfg.DebounceTime = 10 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Minute
	}

	allLibs := append(cfg.TVLibraries, cfg.MovieLibs...)
	org := organizer.NewOrganizer(
		allLibs,
		organizer.WithDryRun(cfg.DryRun),
		organizer.WithTimeout(cfg.Timeout),
		organizer.WithBackend(cfg.Backend),
	)

	return &MediaHandler{
		organizer:     org,
		notifyManager: cfg.NotifyManager,
		tvLibraries:   cfg.TVLibraries,
		movieLibs:     cfg.MovieLibs,
		debounceTime:  cfg.DebounceTime,
		pending:       make(map[string]*time.Timer),
		dryRun:        cfg.DryRun,
		stats:         NewStats(),
	}
}

func (h *MediaHandler) HandleFileEvent(event watcher.FileEvent) error {
	if event.Type == watcher.EventDelete {
		return nil
	}

	if !h.isMediaFile(event.Path) {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if timer, exists := h.pending[event.Path]; exists {
		timer.Stop()
		delete(h.pending, event.Path)
	}

	h.pending[event.Path] = time.AfterFunc(h.debounceTime, func() {
		h.processFile(event.Path)
	})

	return nil
}

func (h *MediaHandler) processFile(path string) {
	h.mu.Lock()
	delete(h.pending, path)
	h.mu.Unlock()

	filename := filepath.Base(path)
	log.Printf("Processing: %s", filename)

	if h.dryRun {
		log.Printf("[dry-run] Would process: %s", filename)
		return
	}

	var result *organizer.OrganizationResult
	var err error
	var targetLib string
	var mediaType notify.MediaType

	if naming.IsTVEpisodeFilename(filename) {
		if len(h.tvLibraries) == 0 {
			log.Printf("No TV libraries configured, skipping: %s", filename)
			return
		}
		targetLib = h.tvLibraries[0]
		mediaType = notify.MediaTypeTVEpisode

		if !h.checkTargetHealth(targetLib) {
			log.Printf("Target library unhealthy, skipping: %s", filename)
			return
		}

		result, err = h.organizer.OrganizeTVEpisode(path, targetLib)
	} else if naming.IsMovieFilename(filename) {
		if len(h.movieLibs) == 0 {
			log.Printf("No movie libraries configured, skipping: %s", filename)
			return
		}
		targetLib = h.movieLibs[0]
		mediaType = notify.MediaTypeMovie

		if !h.checkTargetHealth(targetLib) {
			log.Printf("Target library unhealthy, skipping: %s", filename)
			return
		}

		result, err = h.organizer.OrganizeMovie(path, targetLib)
	} else {
		log.Printf("Cannot determine media type: %s", filename)
		return
	}

	if err != nil {
		log.Printf("Organization failed for %s: %v", filename, err)
		h.stats.RecordError()
		return
	}

	if result.Success {
		log.Printf("Organized: %s -> %s (%.2f MB in %s)",
			filepath.Base(result.SourcePath),
			result.TargetPath,
			float64(result.BytesCopied)/(1024*1024),
			result.Duration)

		if mediaType == notify.MediaTypeMovie {
			h.stats.RecordMovie(result.BytesCopied)
		} else {
			h.stats.RecordTV(result.BytesCopied)
		}

		h.sendNotifications(result, mediaType)
	} else if result.Skipped {
		log.Printf("Skipped: %s - %s", filename, result.SkipReason)
	} else {
		log.Printf("Organization failed: %s - %v", filename, result.Error)
		h.stats.RecordError()
	}
}

func (h *MediaHandler) sendNotifications(result *organizer.OrganizationResult, mediaType notify.MediaType) {
	if h.notifyManager == nil {
		return
	}

	event := notify.OrganizationEvent{
		MediaType:   mediaType,
		SourcePath:  result.SourcePath,
		TargetPath:  result.TargetPath,
		TargetDir:   filepath.Dir(result.TargetPath),
		BytesCopied: result.BytesCopied,
		Duration:    result.Duration,
	}

	h.notifyManager.Notify(event)
}

func (h *MediaHandler) checkTargetHealth(targetLib string) bool {
	err := transfer.CheckDiskHealthForTransfer("", targetLib, 5*time.Second, 0)
	if err != nil {
		log.Printf("Health check failed for %s: %v", targetLib, err)
		return false
	}
	return true
}

func (h *MediaHandler) isMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	mediaExts := map[string]bool{
		".mkv": true, ".mp4": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
		".mpg": true, ".mpeg": true, ".m2ts": true, ".ts": true,
	}
	return mediaExts[ext]
}

func (h *MediaHandler) Stats() StatsSnapshot {
	return h.stats.Snapshot()
}

func (h *MediaHandler) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for path, timer := range h.pending {
		timer.Stop()
		delete(h.pending, path)
	}

	if h.notifyManager != nil {
		h.notifyManager.Close()
	}
}
