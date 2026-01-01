package radarr

import "time"

type SystemStatus struct {
	AppName           string `json:"appName"`
	InstanceName      string `json:"instanceName"`
	Version           string `json:"version"`
	BuildTime         string `json:"buildTime"`
	IsDebug           bool   `json:"isDebug"`
	IsProduction      bool   `json:"isProduction"`
	IsAdmin           bool   `json:"isAdmin"`
	IsUserInteractive bool   `json:"isUserInteractive"`
	StartupPath       string `json:"startupPath"`
	AppData           string `json:"appData"`
	OsName            string `json:"osName"`
	OsVersion         string `json:"osVersion"`
	IsNetCore         bool   `json:"isNetCore"`
	IsLinux           bool   `json:"isLinux"`
	IsOsx             bool   `json:"isOsx"`
	IsWindows         bool   `json:"isWindows"`
	IsDocker          bool   `json:"isDocker"`
	Branch            string `json:"branch"`
	Authentication    string `json:"authentication"`
	MigrationVersion  int    `json:"migrationVersion"`
	UrlBase           string `json:"urlBase"`
	RuntimeVersion    string `json:"runtimeVersion"`
	RuntimeName       string `json:"runtimeName"`
}

type Movie struct {
	ID                    int              `json:"id"`
	Title                 string           `json:"title"`
	OriginalTitle         string           `json:"originalTitle"`
	SortTitle             string           `json:"sortTitle"`
	SizeOnDisk            int64            `json:"sizeOnDisk"`
	Overview              string           `json:"overview"`
	InCinemas             string           `json:"inCinemas,omitempty"`
	PhysicalRelease       string           `json:"physicalRelease,omitempty"`
	DigitalRelease        string           `json:"digitalRelease,omitempty"`
	Year                  int              `json:"year"`
	HasFile               bool             `json:"hasFile"`
	Path                  string           `json:"path"`
	QualityProfileID      int              `json:"qualityProfileId"`
	Monitored             bool             `json:"monitored"`
	MinimumAvailability   string           `json:"minimumAvailability"`
	IsAvailable           bool             `json:"isAvailable"`
	FolderName            string           `json:"folderName"`
	Runtime               int              `json:"runtime"`
	CleanTitle            string           `json:"cleanTitle"`
	ImdbID                string           `json:"imdbId"`
	TmdbID                int              `json:"tmdbId"`
	TitleSlug             string           `json:"titleSlug"`
	Certification         string           `json:"certification"`
	Genres                []string         `json:"genres"`
	Tags                  []int            `json:"tags"`
	Added                 time.Time        `json:"added"`
	Status                string           `json:"status"`
	MovieFile             *MovieFile       `json:"movieFile,omitempty"`
	Collection            *MovieCollection `json:"collection,omitempty"`
	OriginalLanguage      *Language        `json:"originalLanguage,omitempty"`
	AlternateTitles       []AlternateTitle `json:"alternateTitles,omitempty"`
	SecondaryYearSourceId int              `json:"secondaryYearSourceId,omitempty"`
}

type MovieFile struct {
	ID               int        `json:"id"`
	MovieID          int        `json:"movieId"`
	RelativePath     string     `json:"relativePath"`
	Path             string     `json:"path"`
	Size             int64      `json:"size"`
	DateAdded        time.Time  `json:"dateAdded"`
	SceneName        string     `json:"sceneName"`
	ReleaseGroup     string     `json:"releaseGroup"`
	Edition          string     `json:"edition"`
	Quality          Quality    `json:"quality"`
	MediaInfo        *MediaInfo `json:"mediaInfo,omitempty"`
	OriginalFilePath string     `json:"originalFilePath,omitempty"`
}

type MovieCollection struct {
	Name   string `json:"name"`
	TmdbID int    `json:"tmdbId"`
}

type Language struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type AlternateTitle struct {
	SourceType string    `json:"sourceType"`
	MovieID    int       `json:"movieId"`
	Title      string    `json:"title"`
	SourceID   int       `json:"sourceId"`
	Votes      int       `json:"votes"`
	VoteCount  int       `json:"voteCount"`
	Language   *Language `json:"language,omitempty"`
}

type Quality struct {
	Quality  QualityDetail `json:"quality"`
	Revision Revision      `json:"revision"`
}

type QualityDetail struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Source     string `json:"source"`
	Resolution int    `json:"resolution"`
	Modifier   string `json:"modifier"`
}

type Revision struct {
	Version  int  `json:"version"`
	Real     int  `json:"real"`
	IsRepack bool `json:"isRepack"`
}

type MediaInfo struct {
	AudioBitrate          int     `json:"audioBitrate"`
	AudioChannels         float64 `json:"audioChannels"`
	AudioCodec            string  `json:"audioCodec"`
	AudioLanguages        string  `json:"audioLanguages"`
	AudioStreamCount      int     `json:"audioStreamCount"`
	VideoBitDepth         int     `json:"videoBitDepth"`
	VideoBitrate          int     `json:"videoBitrate"`
	VideoCodec            string  `json:"videoCodec"`
	VideoFps              float64 `json:"videoFps"`
	VideoDynamicRange     string  `json:"videoDynamicRange"`
	VideoDynamicRangeType string  `json:"videoDynamicRangeType"`
	Resolution            string  `json:"resolution"`
	RunTime               string  `json:"runTime"`
	ScanType              string  `json:"scanType"`
	Subtitles             string  `json:"subtitles"`
}

type QueueItem struct {
	ID                      int             `json:"id"`
	MovieID                 int             `json:"movieId"`
	Movie                   *Movie          `json:"movie,omitempty"`
	Title                   string          `json:"title"`
	Status                  string          `json:"status"`
	TrackedDownloadStatus   string          `json:"trackedDownloadStatus"`
	TrackedDownloadState    string          `json:"trackedDownloadState"`
	StatusMessages          []StatusMessage `json:"statusMessages"`
	DownloadID              string          `json:"downloadId"`
	Protocol                string          `json:"protocol"`
	DownloadClient          string          `json:"downloadClient"`
	OutputPath              string          `json:"outputPath"`
	ErrorMessage            string          `json:"errorMessage"`
	Size                    int64           `json:"size"`
	Sizeleft                int64           `json:"sizeleft"`
	Timeleft                string          `json:"timeleft"`
	EstimatedCompletionTime *time.Time      `json:"estimatedCompletionTime"`
	Added                   time.Time       `json:"added"`
	Quality                 Quality         `json:"quality"`
	CustomFormats           []CustomFormat  `json:"customFormats,omitempty"`
	CustomFormatScore       int             `json:"customFormatScore"`
}

type StatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

type CustomFormat struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type QueueResponse struct {
	Page          int         `json:"page"`
	PageSize      int         `json:"pageSize"`
	SortKey       string      `json:"sortKey"`
	SortDirection string      `json:"sortDirection"`
	TotalRecords  int         `json:"totalRecords"`
	Records       []QueueItem `json:"records"`
}

type BulkQueueRequest struct {
	IDs              []int `json:"ids"`
	RemoveFromClient bool  `json:"removeFromClient,omitempty"`
	Blocklist        bool  `json:"blocklist,omitempty"`
}

type Command struct {
	Name       string `json:"name"`
	Path       string `json:"path,omitempty"`
	ImportMode string `json:"importMode,omitempty"`
	MovieID    int    `json:"movieId,omitempty"`
	MovieIDs   []int  `json:"movieIds,omitempty"`
	Files      []int  `json:"files,omitempty"`
}

type CommandResponse struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	CommandName string    `json:"commandName"`
	Message     string    `json:"message,omitempty"`
	Priority    string    `json:"priority"`
	Status      string    `json:"status"`
	Queued      time.Time `json:"queued"`
	Started     time.Time `json:"started,omitempty"`
	Ended       time.Time `json:"ended,omitempty"`
	Duration    string    `json:"duration,omitempty"`
	Trigger     string    `json:"trigger"`
	StateChange string    `json:"stateChangeTime,omitempty"`
}
