package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Nomadcxx/jellywatch/internal/ai"
	"github.com/Nomadcxx/jellywatch/internal/logging"
	"github.com/Nomadcxx/jellywatch/internal/naming"
	"github.com/Nomadcxx/jellywatch/internal/notify"
	"github.com/Nomadcxx/jellywatch/internal/organizer"
	"github.com/Nomadcxx/jellywatch/internal/sonarr"
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
	logger        *logging.Logger
	sonarrClient  *sonarr.Client
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
	Logger        *logging.Logger
	TargetUID     int
	TargetGID     int
	FileMode      os.FileMode
	DirMode       os.FileMode
	SonarrClient  *sonarr.Client
	AIIntegrator  *ai.Integrator
}

func NewMediaHandler(cfg MediaHandlerConfig) *MediaHandler {
	if cfg.DebounceTime == 0 {
		cfg.DebounceTime = 10 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Minute
	}
	if cfg.Logger == nil {
		cfg.Logger = logging.Nop()
	}

	allLibs := append(cfg.TVLibraries, cfg.MovieLibs...)
	orgOpts := []func(*organizer.Organizer){
		organizer.WithDryRun(cfg.DryRun),
		organizer.WithTimeout(cfg.Timeout),
		organizer.WithBackend(cfg.Backend),
	}
	if cfg.SonarrClient != nil {
		orgOpts = append(orgOpts, organizer.WithSonarrClient(cfg.SonarrClient))
	}
	if cfg.TargetUID >= 0 || cfg.TargetGID >= 0 || cfg.FileMode != 0 || cfg.DirMode != 0 {
		orgOpts = append(orgOpts, organizer.WithPermissions(cfg.TargetUID, cfg.TargetGID, cfg.FileMode, cfg.DirMode))
	}
	if cfg.AIIntegrator != nil {
		orgOpts = append(orgOpts, organizer.WithAIIntegrator(cfg.AIIntegrator))
	}
	org := organizer.NewOrganizer(allLibs, orgOpts...)

	return &MediaHandler{
		organizer:     org,
		notifyManager: cfg.NotifyManager,
		tvLibraries:   cfg.TVLibraries,
		movieLibs:     cfg.MovieLibs,
		debounceTime:  cfg.DebounceTime,
		pending:       make(map[string]*time.Timer),
		dryRun:        cfg.DryRun,
		stats:         NewStats(),
		logger:        cfg.Logger,
		sonarrClient:  cfg.SonarrClient,
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
	h.logger.Info("handler", "Processing file", logging.F("filename", filename), logging.F("path", path))

	if h.dryRun {
		h.logger.Info("handler", "Dry run - would process", logging.F("filename", filename))
		return
	}

	var result *organizer.OrganizationResult
	var err error
	var targetLib string
	var mediaType notify.MediaType

	isObfuscated := naming.IsObfuscatedFilename(filename)
	if isObfuscated {
		h.logger.Info("handler", "Detected obfuscated filename, using folder name", logging.F("filename", filename))
	}

	isTVEpisode := naming.IsTVEpisodeFromPath(path)

	if isTVEpisode {
		if len(h.tvLibraries) == 0 {
			h.logger.Warn("handler", "No TV libraries configured, skipping", logging.F("filename", filename))
			return
		}
		mediaType = notify.MediaTypeTVEpisode

		// Use auto-selection (queries Sonarr + filesystem)
		result, err = h.organizer.OrganizeTVEpisodeAuto(path, func(p string) (int64, error) {
			info, err := os.Stat(p)
			if err != nil {
				return 0, err
			}
			return info.Size(), nil
		})

		// Extract target library from result for health check logging
		if result != nil && result.TargetPath != "" {
			// TargetPath format: /mnt/STORAGE1/TVSHOWS/Show Name (Year)/Season 01/episode.mkv
			// Extract library: /mnt/STORAGE1/TVSHOWS
			targetLib = filepath.Dir(filepath.Dir(filepath.Dir(result.TargetPath)))
		}
	} else {
		if len(h.movieLibs) == 0 {
			h.logger.Warn("handler", "No movie libraries configured, skipping", logging.F("filename", filename))
			return
		}
		targetLib = h.movieLibs[0]
		mediaType = notify.MediaTypeMovie

		if !h.checkTargetHealth(targetLib) {
			h.logger.Warn("handler", "Target library unhealthy, skipping", logging.F("filename", filename), logging.F("target", targetLib))
			return
		}

		result, err = h.organizer.OrganizeMovie(path, targetLib)
	}

	if err != nil {
		h.logger.Error("handler", "Organization failed", err, logging.F("filename", filename))
		h.stats.RecordError()
		return
	}

	if result.Success {
		h.logger.Info("handler", "Organized successfully",
			logging.F("source", filepath.Base(result.SourcePath)),
			logging.F("target", result.TargetPath),
			logging.F("size_mb", float64(result.BytesCopied)/(1024*1024)),
			logging.F("duration", result.Duration.String()))

		if mediaType == notify.MediaTypeMovie {
			h.stats.RecordMovie(result.BytesCopied)
		} else {
			h.stats.RecordTV(result.BytesCopied)
		}

		h.sendNotifications(result, mediaType)
	} else if result.Skipped {
		h.logger.Info("handler", "Skipped", logging.F("filename", filename), logging.F("reason", result.SkipReason))
	} else {
		h.logger.Error("handler", "Organization failed", result.Error, logging.F("filename", filename))
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
		h.logger.Warn("handler", "Health check failed", logging.F("target", targetLib), logging.F("error", err.Error()))
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
