package daemon

import (
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Nomadcxx/jellywatch/internal/naming"
	"github.com/Nomadcxx/jellywatch/internal/organizer"
	"github.com/Nomadcxx/jellywatch/internal/radarr"
	"github.com/Nomadcxx/jellywatch/internal/sonarr"
	"github.com/Nomadcxx/jellywatch/internal/transfer"
	"github.com/Nomadcxx/jellywatch/internal/watcher"
)

type MediaHandler struct {
	organizer    *organizer.Organizer
	sonarrClient *sonarr.Client
	radarrClient *radarr.Client
	tvLibraries  []string
	movieLibs    []string
	debounceTime time.Duration
	pending      map[string]*time.Timer
	mu           sync.Mutex
	dryRun       bool
	notifySonarr bool
	notifyRadarr bool
}

type MediaHandlerConfig struct {
	TVLibraries  []string
	MovieLibs    []string
	DebounceTime time.Duration
	DryRun       bool
	Timeout      time.Duration
	SonarrClient *sonarr.Client
	RadarrClient *radarr.Client
	NotifySonarr bool
	NotifyRadarr bool
	Backend      transfer.Backend
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
		organizer:    org,
		sonarrClient: cfg.SonarrClient,
		radarrClient: cfg.RadarrClient,
		tvLibraries:  cfg.TVLibraries,
		movieLibs:    cfg.MovieLibs,
		debounceTime: cfg.DebounceTime,
		pending:      make(map[string]*time.Timer),
		dryRun:       cfg.DryRun,
		notifySonarr: cfg.NotifySonarr,
		notifyRadarr: cfg.NotifyRadarr,
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

	if naming.IsTVEpisodeFilename(filename) {
		if len(h.tvLibraries) == 0 {
			log.Printf("No TV libraries configured, skipping: %s", filename)
			return
		}
		targetLib = h.tvLibraries[0]

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
		return
	}

	if result.Success {
		log.Printf("Organized: %s -> %s (%.2f MB in %s)",
			filepath.Base(result.SourcePath),
			result.TargetPath,
			float64(result.BytesCopied)/(1024*1024),
			result.Duration)

		if h.notifySonarr && h.sonarrClient != nil && naming.IsTVEpisodeFilename(filename) {
			h.notifySonarrImport(result.TargetPath)
		}
		if h.notifyRadarr && h.radarrClient != nil && naming.IsMovieFilename(filename) {
			h.notifyRadarrImport(result.TargetPath)
		}
	} else {
		log.Printf("Organization failed: %s - %v", filename, result.Error)
	}
}

func (h *MediaHandler) checkTargetHealth(targetLib string) bool {
	err := transfer.CheckDiskHealthForTransfer("", targetLib, 5*time.Second, 0)
	if err != nil {
		log.Printf("Health check failed for %s: %v", targetLib, err)
		return false
	}
	return true
}

func (h *MediaHandler) notifySonarrImport(targetPath string) {
	targetDir := filepath.Dir(targetPath)

	resp, err := h.sonarrClient.TriggerDownloadedEpisodesScan(targetDir)
	if err != nil {
		log.Printf("Failed to notify Sonarr: %v", err)
		return
	}

	log.Printf("Sonarr import triggered (command ID: %d)", resp.ID)
}

func (h *MediaHandler) notifyRadarrImport(targetPath string) {
	targetDir := filepath.Dir(targetPath)

	resp, err := h.radarrClient.TriggerDownloadedMoviesScan(targetDir)
	if err != nil {
		log.Printf("Failed to notify Radarr: %v", err)
		return
	}

	log.Printf("Radarr import triggered (command ID: %d)", resp.ID)
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

func (h *MediaHandler) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for path, timer := range h.pending {
		timer.Stop()
		delete(h.pending, path)
	}
}
