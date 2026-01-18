package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/Nomadcxx/jellywatch/internal/database"
	"github.com/Nomadcxx/jellywatch/internal/scanner"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Theme colors - Jellyfin
var (
	Primary      = lipgloss.Color("#8b0000") // Dark red
	Secondary    = lipgloss.Color("#cc7722") // Orange/copper
	Accent       = lipgloss.Color("#f6aa1c") // Bright orange/gold
	BgBase       = lipgloss.Color("#2d0e0e") // Very dark red/brown
	FgPrimary    = lipgloss.Color("#FFFFFF") // White text
	FgMuted      = lipgloss.Color("#888888") // Muted text
	ErrorColor   = lipgloss.Color("#FF5555") // Bright red for errors
	SuccessColor = lipgloss.Color("#e1ad01") // Yellow/gold for success
)

// Styles
var (
	headerStyle = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	checkMark   = lipgloss.NewStyle().Foreground(SuccessColor).SetString("[OK]")
	failMark    = lipgloss.NewStyle().Foreground(ErrorColor).SetString("[FAIL]")
	skipMark    = lipgloss.NewStyle().Foreground(FgMuted).SetString("[SKIP]")
)

// Installation steps
type installStep int

const (
	stepWelcome installStep = iota
	stepUpdateNotice  // NEW: shown when existing DB detected
	stepUninstallConfirm // NEW: shown to confirm what to remove during uninstall
	stepWatchDirs
	stepLibraryPaths
	stepSonarr
	stepRadarr
	stepAI           // NEW: AI configuration step
	stepPermissions
	stepConfirm
	stepInstalling
	stepInitialScan   // NEW: runs scanner with progress
	stepScanEducation // NEW: shows results and commands
	stepComplete
)

// Task status
type taskStatus int

const (
	statusPending taskStatus = iota
	statusRunning
	statusComplete
	statusFailed
	statusSkipped
)

// Installation task
type installTask struct {
	name        string
	description string
	execute     func(*model) error
	optional    bool
	status      taskStatus
}

// Main model
type model struct {
	step             installStep
	tasks            []installTask
	currentTaskIndex int
	width            int
	height           int
	spinner          spinner.Model
	errors           []string
	uninstallMode    bool
	selectedOption   int // 0 = Install, 1 = Uninstall

	// Text inputs
	inputs       []textinput.Model
	focusedInput int

	// User configuration
	tvWatchDir      string
	movieWatchDir   string
	tvLibraryDir    string
	movieLibraryDir string
	sonarrEnabled   bool
	sonarrURL       string
	sonarrAPIKey    string
	sonarrTested    bool
	sonarrVersion   string
	radarrEnabled   bool
	radarrURL       string
	radarrAPIKey    string
	radarrTested    bool
	radarrVersion   string
	testingAPI      bool
	testError       string

	// Permissions configuration
	permUser     string
	permGroup    string
	permFileMode string
	permDirMode  string

	// AI configuration
	aiEnabled         bool
	aiOllamaURL       string
	aiModel           string
	aiModels          []string     // Available models from Ollama
	aiModelIndex      int          // Currently selected model index
	aiModelDropdownOpen bool        // Is model dropdown open?
	aiTestResult       string       // Result of testing Ollama connection
	aiTesting         bool         // Currently testing connection
	aiOllamaInstalled bool         // Whether Ollama is detected as installed
	aiInstallingOllama bool         // Currently installing Ollama
	aiInstallResult   string       // Result of Ollama installation attempt

	// Installation mode detection
	existingDBDetected bool
	existingDBPath     string
	existingDBModTime  time.Time
	updateWithRefresh  bool       // true = update + rescan, false = update only
	forceWizard        bool       // 'W' key pressed to run full wizard
	daemonWasRunning   bool       // true if daemon was running before update (to restart after)
	keepDatabase       bool       // true = keep database during uninstall, false = remove it

	// Scan state
	scanProgress    ScanProgress
	scanResult      *ScanResult
	scanStats       *database.ConsolidationStats
	exampleDupe     *database.DuplicateGroup
	program         *tea.Program // for sending messages from goroutines
	scanCancel      context.CancelFunc // for cancelling scan
	episodeCount    int // actual count from database
	movieCount      int // actual count from database
}

// ScanProgress mirrors scanner.ScanProgress for TUI
type ScanProgress struct {
	FilesScanned   int
	CurrentPath    string
	LibrariesDone  int
	LibrariesTotal int
}

// ScanResult mirrors scanner.ScanResult for TUI
type ScanResult struct {
	FilesScanned int
	FilesAdded   int
	Duration     time.Duration
	Errors       []error
}

// Task completion message
type taskCompleteMsg struct {
	index   int
	success bool
	error   string
}

// API test messages
type apiTestStartMsg struct {
	service string // "sonarr" or "radarr"
}

type apiTestResultMsg struct {
	service string
	success bool
	version string
	err     error
}

// Ollama test result message
type ollamaTestResultMsg struct {
	success bool
	models  []string
	err     error
}

// Ollama install result message
type ollamaInstallResultMsg struct {
	success bool
	err     error
}

// Scan start message (to store cancel function)
type scanStartMsg struct {
	cancel context.CancelFunc
}

// Scan progress message
type scanProgressMsg struct {
	progress ScanProgress
}

// Scan complete message
type scanCompleteMsg struct {
	result       *ScanResult
	stats        *database.ConsolidationStats
	dupe         *database.DuplicateGroup
	err          error
	episodeCount int
	movieCount   int
}

// detectExistingInstall checks if jellywatch is already installed
// getConfigDir returns the actual user's config dir, respecting SUDO_USER
func getConfigDir() (string, error) {
	// If running with sudo, get the original user's home directory
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		// Running with sudo - get original user's home
		userInfo, err := user.Lookup(sudoUser)
		if err == nil {
			return filepath.Join(userInfo.HomeDir, ".config"), nil
		}
	}

	// Fallback to standard method
	return os.UserConfigDir()
}

// detectExistingInstall checks if jellywatch is already installed
func detectExistingInstall() (exists bool, dbPath string, modTime time.Time) {
	configDir, err := getConfigDir()
	if err != nil {
		return false, "", time.Time{}
	}

	dbPath = filepath.Join(configDir, "jellywatch", "media.db")
	info, err := os.Stat(dbPath)
	if err != nil {
		return false, dbPath, time.Time{}
	}

	return true, dbPath, info.ModTime()
}

// Initialize new model
func newModel() model {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(Secondary)
	s.Spinner = spinner.Dot

	// Detect existing installation
	exists, dbPath, modTime := detectExistingInstall()

	return model{
		step:             stepWelcome,
		currentTaskIndex: -1,
		spinner:          s,
		errors:           []string{},
		selectedOption:   0,
		// Default values
		sonarrEnabled:    false,
		radarrEnabled:    false,
		// AI defaults
		aiEnabled:         false,
		aiOllamaURL:       "http://localhost:11434",
		aiModel:           "",
		aiModels:          []string{},
		aiModelIndex:      0,
		aiModelDropdownOpen: false,
		aiOllamaInstalled: checkOllamaInstalled(),
		existingDBDetected: exists,
		existingDBPath:     dbPath,
		existingDBModTime:  modTime,
	}
}

func testSonarr(url, apiKey string) tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 5 * time.Second}

		req, err := http.NewRequest("GET", url+"/api/v3/system/status", nil)
		if err != nil {
			return apiTestResultMsg{service: "sonarr", success: false, err: err}
		}

		req.Header.Set("X-Api-Key", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			return apiTestResultMsg{service: "sonarr", success: false, err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return apiTestResultMsg{
				service: "sonarr",
				success: false,
				err:     fmt.Errorf("status %d", resp.StatusCode),
			}
		}

		var result struct {
			Version string `json:"version"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return apiTestResultMsg{service: "sonarr", success: false, err: err}
		}

		return apiTestResultMsg{
			service: "sonarr",
			success: true,
			version: result.Version,
		}
	}
}

func testRadarr(url, apiKey string) tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 5 * time.Second}

		req, err := http.NewRequest("GET", url+"/api/v3/system/status", nil)
		if err != nil {
			return apiTestResultMsg{service: "radarr", success: false, err: err}
		}

		req.Header.Set("X-Api-Key", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			return apiTestResultMsg{service: "radarr", success: false, err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return apiTestResultMsg{
				service: "radarr",
				success: false,
				err:     fmt.Errorf("status %d", resp.StatusCode),
			}
		}

		var result struct {
			Version string `json:"version"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return apiTestResultMsg{service: "radarr", success: false, err: err}
		}

		return apiTestResultMsg{
			service: "radarr",
			success: true,
			version: result.Version,
		}
	}
}

func initializeDatabase(m *model) error {
	db, err := database.Open()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()
	return nil
}

func (m *model) initTasks() {
	if m.uninstallMode {
		// Build task description based on user's choice
		dbTaskDesc := "Removing database"
		if m.keepDatabase {
			dbTaskDesc = "Preserving database"
		}

		m.tasks = []installTask{
			{name: "Stop daemon", description: "Stopping jellywatchd service", execute: stopDaemon, status: statusPending, optional: true},
			{name: "Remove binaries", description: "Removing /usr/local/bin/jellywatch*", execute: removeBinaries, status: statusPending},
			{name: "Remove systemd files", description: "Removing systemd service file", execute: removeSystemdFiles, status: statusPending},
			{name: "Database", description: dbTaskDesc, execute: removeConfigAndDB, status: statusPending},
		}
	} else {
		m.tasks = []installTask{
			{name: "Build binaries", description: "Building jellywatch and jellywatchd", execute: buildBinaries, status: statusPending},
			{name: "Install binaries", description: "Installing to /usr/local/bin", execute: installBinaries, status: statusPending},
			{name: "Create config", description: "Creating configuration directory", execute: createConfig, status: statusPending},
			{name: "Initialize database", description: "Creating media database", execute: initializeDatabase, status: statusPending},
			{name: "Install systemd files", description: "Installing service file", execute: installSystemdFiles, status: statusPending},
			{name: "Enable daemon", description: "Enabling and starting jellywatchd", execute: enableDaemon, status: statusPending},
		}
	}
}

func executeTask(index int, m *model) tea.Cmd {
	return func() tea.Msg {
		task := &m.tasks[index]
		err := task.execute(m)

		if err != nil {
			return taskCompleteMsg{
				index:   index,
				success: false,
				error:   err.Error(),
			}
		}

		return taskCompleteMsg{
			index:   index,
			success: true,
			error:   "",
		}
	}
}

func buildBinaries(m *model) error {
	cmd := exec.Command("go", "build", "-o", "jellywatch", "./cmd/jellywatch")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build jellywatch: %w", err)
	}

	cmd = exec.Command("go", "build", "-o", "jellywatchd", "./cmd/jellywatchd")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build jellywatchd: %w", err)
	}

	return nil
}

func installBinaries(m *model) error {
	// Check if jellywatchd is currently running
	wasRunning := false
	if _, err := os.Stat("/usr/local/bin/jellywatchd"); err == nil {
		// Check service status
		cmd := exec.Command("systemctl", "is-active", "jellywatchd")
		output, _ := cmd.Output()
		if strings.TrimSpace(string(output)) == "active" {
			wasRunning = true
			// Stop the service before replacing binaries
			if err := exec.Command("systemctl", "stop", "jellywatchd").Run(); err != nil {
				// If stop fails, try kill
				exec.Command("killall", "jellywatchd").Run()
			}
			// Give it a moment to fully stop
			time.Sleep(500 * time.Millisecond)
		}
	}

	binaries := []string{"jellywatch", "jellywatchd"}
	for _, binary := range binaries {
		src := filepath.Join(".", binary)
		dst := filepath.Join("/usr/local/bin", binary)

		input, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", binary, err)
		}

		if err := os.WriteFile(dst, input, 0755); err != nil {
			return fmt.Errorf("failed to install %s: %w", binary, err)
		}
	}

	// Store whether we need to restart the daemon later
	m.daemonWasRunning = wasRunning

	return nil
}

func getRealUser() (username, homeDir string, uid, gid int) {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		username = sudoUser
		homeDir = "/home/" + sudoUser
		if sudoUID := os.Getenv("SUDO_UID"); sudoUID != "" {
			fmt.Sscanf(sudoUID, "%d", &uid)
		}
		if sudoGID := os.Getenv("SUDO_GID"); sudoGID != "" {
			fmt.Sscanf(sudoGID, "%d", &gid)
		}
		return
	}
	homeDir, _ = os.UserHomeDir()
	username = os.Getenv("USER")
	uid = os.Getuid()
	gid = os.Getgid()
	return
}

func createConfig(m *model) error {
	username, homeDir, uid, gid := getRealUser()
	_ = username

	configDir := filepath.Join(homeDir, ".config", "jellywatch")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if uid > 0 && gid > 0 {
		os.Chown(filepath.Join(homeDir, ".config"), uid, gid)
		os.Chown(configDir, uid, gid)
	}

	configPath := filepath.Join(configDir, "config.toml")

	// Check if config already exists - don't overwrite during updates
	if _, err := os.Stat(configPath); err == nil {
		// Config exists - skip writing to preserve user's settings
		return nil
	}

	// Config doesn't exist - create new one
	configContent := generateConfig(m)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	if uid > 0 && gid > 0 {
		os.Chown(configPath, uid, gid)
	}

	return nil
}

func splitAndFormatPaths(pathsInput string) string {
	paths := strings.Split(pathsInput, ",")
	var formatted []string
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed != "" {
			formatted = append(formatted, fmt.Sprintf("\"%s\"", trimmed))
		}
	}
	return strings.Join(formatted, ", ")
}

func generateConfig(m *model) string {
	var sb strings.Builder

	sb.WriteString("[watch]\n")
	if m.tvWatchDir != "" {
		sb.WriteString(fmt.Sprintf("tv = [%s]\n", splitAndFormatPaths(m.tvWatchDir)))
	}
	if m.movieWatchDir != "" {
		sb.WriteString(fmt.Sprintf("movies = [%s]\n", splitAndFormatPaths(m.movieWatchDir)))
	}
	sb.WriteString("\n")

	sb.WriteString("[libraries]\n")
	if m.tvLibraryDir != "" {
		sb.WriteString(fmt.Sprintf("tv = [%s]\n", splitAndFormatPaths(m.tvLibraryDir)))
	}
	if m.movieLibraryDir != "" {
		sb.WriteString(fmt.Sprintf("movies = [%s]\n", splitAndFormatPaths(m.movieLibraryDir)))
	}
	sb.WriteString("\n")

	if m.sonarrEnabled {
		sb.WriteString("[sonarr]\n")
		sb.WriteString("enabled = true\n")
		sb.WriteString(fmt.Sprintf("url = \"%s\"\n", m.sonarrURL))
		sb.WriteString(fmt.Sprintf("api_key = \"%s\"\n", m.sonarrAPIKey))
		sb.WriteString("notify_on_import = true\n")
		sb.WriteString("\n")
	}

	if m.radarrEnabled {
		sb.WriteString("[radarr]\n")
		sb.WriteString("enabled = true\n")
		sb.WriteString(fmt.Sprintf("url = \"%s\"\n", m.radarrURL))
		sb.WriteString(fmt.Sprintf("api_key = \"%s\"\n", m.radarrAPIKey))
		sb.WriteString("notify_on_import = true\n")
		sb.WriteString("\n")
	}

	if m.aiEnabled {
		sb.WriteString("[ai]\n")
		sb.WriteString("enabled = true\n")
		sb.WriteString(fmt.Sprintf("ollama_endpoint = \"%s\"\n", m.aiOllamaURL))
		if m.aiModel != "" {
			sb.WriteString(fmt.Sprintf("model = \"%s\"\n", m.aiModel))
		}
		sb.WriteString("confidence_threshold = 0.7\n")
		sb.WriteString("timeout_seconds = 30\n")
		sb.WriteString("cache_enabled = true\n")
		sb.WriteString("auto_resolve_risky = true\n")
		sb.WriteString("\n")
	}

	sb.WriteString("[daemon]\n")
	sb.WriteString("enabled = true\n")
	sb.WriteString("scan_frequency = \"5m\"\n")
	sb.WriteString("\n")

	sb.WriteString("[options]\n")
	sb.WriteString("dry_run = false\n")
	sb.WriteString("verify_checksums = false\n")
	sb.WriteString("delete_source = true\n")

	if m.permUser != "" || m.permGroup != "" {
		sb.WriteString("\n[permissions]\n")
		if m.permUser != "" {
			sb.WriteString(fmt.Sprintf("user = \"%s\"\n", m.permUser))
		}
		if m.permGroup != "" {
			sb.WriteString(fmt.Sprintf("group = \"%s\"\n", m.permGroup))
		}
		if m.permFileMode != "" && m.permFileMode != "0644" {
			sb.WriteString(fmt.Sprintf("file_mode = \"%s\"\n", m.permFileMode))
		}
		if m.permDirMode != "" && m.permDirMode != "0755" {
			sb.WriteString(fmt.Sprintf("dir_mode = \"%s\"\n", m.permDirMode))
		}
	}

	return sb.String()
}

func installSystemdFiles(m *model) error {
	username, homeDir, _, _ := getRealUser()
	configPath := filepath.Join(homeDir, ".config", "jellywatch", "config.toml")

	serviceContent := fmt.Sprintf(`[Unit]
Description=JellyWatch media file organizer daemon
After=network.target

[Service]
Type=simple
User=%s
Environment="HOME=%s"
ExecStart=/usr/local/bin/jellywatchd --config "%s"
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
`, username, homeDir, configPath)

	servicePath := "/etc/systemd/system/jellywatchd.service"
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to install systemd service: %w", err)
	}

	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	return nil
}

func enableDaemon(m *model) error {
	cmd := exec.Command("systemctl", "enable", "jellywatchd")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	// If the service was recently stopped (during binary update), wait a moment before starting
	if m.daemonWasRunning {
		time.Sleep(500 * time.Millisecond)
	}

	cmd = exec.Command("systemctl", "start", "jellywatchd")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

// stopDaemon stops and disables the jellywatchd service.
// It's robust - doesn't fail if service is already stopped or doesn't exist.
func stopDaemon(m *model) error {
	// First check if service exists and is active
	checkCmd := exec.Command("systemctl", "is-active", "jellywatchd")
	output, _ := checkCmd.Output()
	state := strings.TrimSpace(string(output))

	// Only try to stop if the service is actually active or activating
	if state == "active" || state == "activating" || state == "deactivating" {
		stopCmd := exec.Command("systemctl", "stop", "jellywatchd")
		if err := stopCmd.Run(); err != nil {
			return fmt.Errorf("failed to stop service (state: %s): %w", state, err)
		}
	}

	// Disable the service (ignore errors if not enabled/doesn't exist)
	_ = exec.Command("systemctl", "disable", "jellywatchd").Run()

	return nil
}

func removeBinaries(m *model) error {
	binaries := []string{
		"/usr/local/bin/jellywatch",
		"/usr/local/bin/jellywatchd",
	}

	for _, binary := range binaries {
		if err := os.Remove(binary); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %w", binary, err)
		}
	}

	return nil
}

func removeSystemdFiles(m *model) error {
	servicePath := "/etc/systemd/system/jellywatchd.service"

	// Stop and disable the service BEFORE removing the service file
	// Use systemctl is-active to check if service is running
	checkCmd := exec.Command("systemctl", "is-active", "jellywatchd")
	if output, _ := checkCmd.Output(); strings.TrimSpace(string(output)) == "active" {
		// Service is running, stop it
		if err := exec.Command("systemctl", "stop", "jellywatchd").Run(); err != nil {
			// Log warning but continue
			fmt.Printf("Warning: failed to stop jellywatchd service: %v\n", err)
		}
	}

	// Disable the service
	exec.Command("systemctl", "disable", "jellywatchd").Run() // Ignore errors, service might not be enabled

	// Now remove the service file
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	return nil
}

func removeConfigAndDB(m *model) error {
	// Get the actual user's config dir, respecting SUDO_USER
	configDir, err := getConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config dir: %w", err)
	}

	jellywatchDir := filepath.Join(configDir, "jellywatch")
	dbPath := filepath.Join(jellywatchDir, "media.db")

	if m.keepDatabase {
		// Only show message - database is preserved
		fmt.Printf("\n  Database preserved: %s\n", dbPath)
		return nil
	}

	// Remove only the database file
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove database: %w", err)
	}

	fmt.Printf("\n  Database removed: %s\n", dbPath)
	return nil
}

func (m *model) initInputsForStep() {
	m.inputs = make([]textinput.Model, 0)
	m.focusedInput = 0

	switch m.step {
	case stepWatchDirs:
		tvInput := textinput.New()
		tvInput.Placeholder = "/mnt/downloads/tv (comma-separated for multiple)"
		tvInput.Focus()
		tvInput.Width = 60
		tvInput.SetValue(m.tvWatchDir)

		movieInput := textinput.New()
		movieInput.Placeholder = "/mnt/downloads/movies (comma-separated for multiple)"
		movieInput.Width = 60
		movieInput.SetValue(m.movieWatchDir)

		m.inputs = []textinput.Model{tvInput, movieInput}

	case stepLibraryPaths:
		tvLibInput := textinput.New()
		tvLibInput.Placeholder = "/mnt/media/TV Shows (comma-separated for multiple)"
		tvLibInput.Focus()
		tvLibInput.Width = 60
		tvLibInput.SetValue(m.tvLibraryDir)

		movieLibInput := textinput.New()
		movieLibInput.Placeholder = "/mnt/media/Movies (comma-separated for multiple)"
		movieLibInput.Width = 60
		movieLibInput.SetValue(m.movieLibraryDir)

		m.inputs = []textinput.Model{tvLibInput, movieLibInput}

	case stepSonarr:
		urlInput := textinput.New()
		urlInput.Placeholder = "http://localhost:8989"
		urlInput.Focus()
		urlInput.Width = 50
		urlInput.SetValue(m.sonarrURL)

		apiInput := textinput.New()
		apiInput.Placeholder = "API Key"
		apiInput.Width = 50
		apiInput.EchoMode = textinput.EchoPassword
		apiInput.SetValue(m.sonarrAPIKey)

		m.inputs = []textinput.Model{urlInput, apiInput}

	case stepRadarr:
		urlInput := textinput.New()
		urlInput.Placeholder = "http://localhost:7878"
		urlInput.Focus()
		urlInput.Width = 50
		urlInput.SetValue(m.radarrURL)

		apiInput := textinput.New()
		apiInput.Placeholder = "API Key"
		apiInput.Width = 50
		apiInput.EchoMode = textinput.EchoPassword
		apiInput.SetValue(m.radarrAPIKey)

		m.inputs = []textinput.Model{urlInput, apiInput}

	case stepAI:
		if m.aiEnabled {
			// Ollama URL input
			urlInput := textinput.New()
			urlInput.Placeholder = "http://localhost:11434"
			urlInput.Focus()
			urlInput.Width = 50
			urlInput.SetValue(m.aiOllamaURL)
			m.inputs = []textinput.Model{urlInput}
		} else {
			m.inputs = []textinput.Model{}
		}

	case stepPermissions:
		userInput := textinput.New()
		userInput.Placeholder = "jellyfin (username or UID)"
		userInput.Focus()
		userInput.Width = 50
		userInput.SetValue(m.permUser)

		groupInput := textinput.New()
		groupInput.Placeholder = "jellyfin (group name or GID)"
		groupInput.Width = 50
		groupInput.SetValue(m.permGroup)

		fileModeInput := textinput.New()
		fileModeInput.Placeholder = "0644"
		fileModeInput.Width = 20
		if m.permFileMode == "" {
			fileModeInput.SetValue("0644")
		} else {
			fileModeInput.SetValue(m.permFileMode)
		}

		dirModeInput := textinput.New()
		dirModeInput.Placeholder = "0755"
		dirModeInput.Width = 20
		if m.permDirMode == "" {
			dirModeInput.SetValue("0755")
		} else {
			dirModeInput.SetValue(m.permDirMode)
		}

		m.inputs = []textinput.Model{userInput, groupInput, fileModeInput, dirModeInput}
	}
}

func (m *model) saveInputsFromStep() {
	switch m.step {
	case stepWatchDirs:
		if len(m.inputs) >= 2 {
			m.tvWatchDir = strings.TrimSpace(m.inputs[0].Value())
			m.movieWatchDir = strings.TrimSpace(m.inputs[1].Value())
		}
	case stepLibraryPaths:
		if len(m.inputs) >= 2 {
			m.tvLibraryDir = strings.TrimSpace(m.inputs[0].Value())
			m.movieLibraryDir = strings.TrimSpace(m.inputs[1].Value())
		}
	case stepSonarr:
		if len(m.inputs) >= 2 {
			m.sonarrURL = strings.TrimSpace(m.inputs[0].Value())
			m.sonarrAPIKey = strings.TrimSpace(m.inputs[1].Value())
		}
	case stepRadarr:
		if len(m.inputs) >= 2 {
			m.radarrURL = strings.TrimSpace(m.inputs[0].Value())
			m.radarrAPIKey = strings.TrimSpace(m.inputs[1].Value())
		}
	case stepAI:
		// Save Ollama URL from text input
		if len(m.inputs) >= 1 {
			m.aiOllamaURL = strings.TrimSpace(m.inputs[0].Value())
		}
	case stepPermissions:
		if len(m.inputs) >= 4 {
			m.permUser = strings.TrimSpace(m.inputs[0].Value())
			m.permGroup = strings.TrimSpace(m.inputs[1].Value())
			m.permFileMode = strings.TrimSpace(m.inputs[2].Value())
			m.permDirMode = strings.TrimSpace(m.inputs[3].Value())
		}
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Cancel scan if running
			if m.scanCancel != nil {
				m.scanCancel()
				m.scanCancel = nil
			}
			if m.step != stepInstalling && m.step != stepInitialScan {
				return m, tea.Quit
			}
			// If scanning, allow quit but scan will continue in background
			if m.step == stepInitialScan {
				return m, tea.Quit
			}
		case "w", "W":
			if m.step == stepWelcome && m.existingDBDetected {
				m.forceWizard = true
				// Re-render to show fresh install options
				return m, nil
			}
		case "up", "k":
			if m.step == stepWelcome && m.selectedOption > 0 {
				m.selectedOption--
			} else if m.step == stepUpdateNotice && m.selectedOption > 0 {
				m.selectedOption--
			} else if m.step == stepUninstallConfirm && m.selectedOption > 0 {
				m.selectedOption--
			} else if m.step == stepAI && m.aiModelDropdownOpen {
				if m.aiModelIndex > 0 {
					m.aiModelIndex--
					if len(m.aiModels) > 0 {
						m.aiModel = m.aiModels[m.aiModelIndex]
					}
				}
				return m, nil
			} else if m.step == stepSonarr || m.step == stepRadarr {
				if m.focusedInput == 1 && m.selectedOption > 0 {
					m.selectedOption--
				}
			} else if m.step == stepConfirm && m.selectedOption > 0 {
				m.selectedOption--
			} else if len(m.inputs) > 0 && m.focusedInput > 0 {
				m.inputs[m.focusedInput].Blur()
				m.focusedInput--
				m.inputs[m.focusedInput].Focus()
			}
		case "down", "j":
			if m.step == stepWelcome && m.selectedOption < 1 {
				m.selectedOption++
			} else if m.step == stepUpdateNotice && m.selectedOption < 1 {
				m.selectedOption++
			} else if m.step == stepUninstallConfirm && m.selectedOption < 1 {
				m.selectedOption++
			} else if m.step == stepAI && m.aiModelDropdownOpen {
				if m.aiModelIndex < len(m.aiModels)-1 {
					m.aiModelIndex++
					if len(m.aiModels) > 0 {
						m.aiModel = m.aiModels[m.aiModelIndex]
					}
				}
				return m, nil
			} else if m.step == stepSonarr || m.step == stepRadarr {
				if m.focusedInput == 1 && m.selectedOption < 1 {
					m.selectedOption++
				}
			} else if m.step == stepConfirm && m.selectedOption < 1 {
				m.selectedOption++
			} else if len(m.inputs) > 0 && m.focusedInput >= 0 && m.focusedInput < len(m.inputs)-1 {
				m.inputs[m.focusedInput].Blur()
				m.focusedInput++
				m.inputs[m.focusedInput].Focus()
			}
		case "tab":
			if m.step == stepSonarr || m.step == stepRadarr {
				if len(m.inputs) > 0 {
					if m.focusedInput < len(m.inputs)-1 {
						m.inputs[m.focusedInput].Blur()
						m.focusedInput++
						m.inputs[m.focusedInput].Focus()
						m.selectedOption = 0
					} else {
						m.inputs[m.focusedInput].Blur()
						m.focusedInput = 0
						m.selectedOption = 1
					}
				}
			} else if m.step == stepAI && len(m.inputs) > 0 {
				// Toggle between URL input focused and blurred
				if m.focusedInput >= 0 && m.focusedInput < len(m.inputs) {
					// Input is focused, blur it
					m.inputs[m.focusedInput].Blur()
					m.focusedInput = -1
				} else {
					// Input is blurred, focus it
					m.focusedInput = 0
					m.inputs[0].Focus()
				}
			}
		case "t":
			if m.step == stepSonarr && !m.testingAPI {
				m.saveInputsFromStep()
				if m.sonarrURL != "" && m.sonarrAPIKey != "" {
					m.testingAPI = true
					m.testError = ""
					return m, testSonarr(m.sonarrURL, m.sonarrAPIKey)
				}
			} else if m.step == stepRadarr && !m.testingAPI {
				m.saveInputsFromStep()
				if m.radarrURL != "" && m.radarrAPIKey != "" {
					m.testingAPI = true
					m.testError = ""
					return m, testRadarr(m.radarrURL, m.radarrAPIKey)
				}
			} else if m.step == stepAI && !m.aiTesting && !m.aiModelDropdownOpen {
				// Test Ollama connection and fetch models
				m.aiTesting = true
				m.aiTestResult = ""
				return m, testOllama(m.aiOllamaURL)
			}
		case "e":
			if m.step == stepAI && !m.aiModelDropdownOpen {
				// Toggle AI enabled/disabled
				m.aiEnabled = !m.aiEnabled
				if m.aiEnabled {
					// Initialize inputs when enabling AI
					m.initInputsForStep()
					m.focusedInput = 0
					if len(m.inputs) > 0 {
						m.inputs[0].Focus()
					}
					// Auto-fetch models when enabling (only if Ollama is installed)
					if m.aiOllamaInstalled {
						m.aiTesting = true
						m.aiTestResult = ""
						return m, testOllama(m.aiOllamaURL)
					}
				} else {
					// Clear model selection when disabled
					m.aiModel = ""
					m.aiModels = []string{}
					m.aiModelIndex = 0
					m.aiTestResult = ""
					m.inputs = []textinput.Model{}
				}
			}
		case "i":
			if m.step == stepAI && !m.aiInstallingOllama && !m.aiOllamaInstalled {
				// Install Ollama
				m.aiInstallingOllama = true
				m.aiInstallResult = ""
				return m, installOllama()
			}
		case "m", "M":
			if m.step == stepAI && m.aiEnabled && len(m.aiModels) > 0 && !m.aiModelDropdownOpen {
				// Open model dropdown
				m.aiModelDropdownOpen = true
				return m, nil
			}
		case "enter":
			if m.step == stepWelcome {
				if m.selectedOption == 0 {
					if m.existingDBDetected && !m.forceWizard {
						// Go to update notice screen
						m.step = stepUpdateNotice
					} else {
						// Fresh install - go to watch dirs
						m.step = stepWatchDirs
						m.initInputsForStep()
					}
				} else {
					// Uninstall - go to confirmation screen first
					m.uninstallMode = true
					m.selectedOption = 0 // Default: remove database
					m.step = stepUninstallConfirm
					return m, nil
				}
				return m, nil
			} else if m.step == stepAI {
				if m.aiModelDropdownOpen {
					// Close dropdown and confirm selection
					m.aiModelDropdownOpen = false
					if len(m.aiModels) > 0 && m.aiModelIndex < len(m.aiModels) {
						m.aiModel = m.aiModels[m.aiModelIndex]
					}
					return m, nil
				}
				// Continue to next step (Enter)
				m.saveInputsFromStep()
				m.step = stepPermissions
				m.initInputsForStep()
				return m, nil
			} else if m.step == stepUpdateNotice {
				switch msg.String() {
				case "w", "W":
					m.forceWizard = true
					m.step = stepWatchDirs
					m.initInputsForStep()
					return m, nil
				case "enter":
					m.updateWithRefresh = (m.selectedOption == 0)
					m.initTasks()
					m.step = stepInstalling
					m.currentTaskIndex = 0
					m.tasks[0].status = statusRunning
					return m, tea.Batch(m.spinner.Tick, executeTask(0, &m))
				case "esc":
					m.step = stepWelcome
					m.selectedOption = 0
					return m, nil
				}
			} else if m.step == stepUninstallConfirm {
				switch msg.String() {
				case "enter":
					// selectedOption 0 = remove database, 1 = keep database
					m.keepDatabase = (m.selectedOption == 1)
					m.initTasks()
					m.step = stepInstalling
					m.currentTaskIndex = 0
					m.tasks[0].status = statusRunning
					return m, tea.Batch(m.spinner.Tick, executeTask(0, &m))
				case "esc":
					m.step = stepWelcome
					m.selectedOption = 0
					return m, nil
				}
			} else if m.step == stepWatchDirs || m.step == stepLibraryPaths {
				m.saveInputsFromStep()
				if m.step == stepWatchDirs {
					m.step = stepLibraryPaths
				} else if m.step == stepLibraryPaths {
					m.step = stepSonarr
				}
				m.initInputsForStep()
				return m, nil
			} else if m.step == stepSonarr {
				m.saveInputsFromStep()
				m.sonarrEnabled = len(m.sonarrURL) > 0 && len(m.sonarrAPIKey) > 0
				m.step = stepRadarr
				m.initInputsForStep()
				return m, nil
			} else if m.step == stepRadarr {
				m.saveInputsFromStep()
				m.radarrEnabled = len(m.radarrURL) > 0 && len(m.radarrAPIKey) > 0
				m.step = stepAI
				m.initInputsForStep()
				return m, nil
			} else if m.step == stepPermissions {
				m.saveInputsFromStep()
				m.step = stepConfirm
				m.selectedOption = 0
				return m, nil
			} else if m.step == stepConfirm {
				if m.selectedOption == 0 {
					m.initTasks()
					m.step = stepInstalling
					m.currentTaskIndex = 0
					m.tasks[0].status = statusRunning
					return m, tea.Batch(
						m.spinner.Tick,
						executeTask(0, &m),
					)
				} else {
					m.step = stepWatchDirs
					m.initInputsForStep()
				}
				return m, nil
			} else if m.step == stepScanEducation {
				if msg.String() == "enter" {
					m.step = stepComplete
					return m, nil
				}
			} else if m.step == stepComplete {
				return m, tea.Quit
			}
		case "esc":
			if m.step == stepAI && m.aiModelDropdownOpen {
				// Close dropdown but stay on AI step
				m.aiModelDropdownOpen = false
				return m, nil
			} else if m.step != stepWelcome && m.step != stepInstalling && m.step != stepComplete && m.step != stepUpdateNotice {
				prevStep := m.step - 1
				if prevStep < stepWelcome {
					prevStep = stepWelcome
				}
				m.step = prevStep
				if m.step != stepWelcome && m.step != stepConfirm && m.step != stepUpdateNotice {
					m.initInputsForStep()
				}
				return m, nil
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case apiTestResultMsg:
		m.testingAPI = false
		if msg.service == "sonarr" {
			if msg.success {
				m.sonarrTested = true
				m.sonarrVersion = msg.version
				m.testError = ""
			} else {
				m.sonarrTested = false
				m.sonarrVersion = ""
				m.testError = msg.err.Error()
			}
		} else if msg.service == "radarr" {
			if msg.success {
				m.radarrTested = true
				m.radarrVersion = msg.version
				m.testError = ""
			} else {
				m.radarrTested = false
				m.radarrVersion = ""
				m.testError = msg.err.Error()
			}
		}
		return m, nil

	case ollamaTestResultMsg:
		m.aiTesting = false
		if msg.success {
			m.aiTestResult = fmt.Sprintf("Found %d models", len(msg.models))
			m.aiModels = msg.models
			if len(msg.models) > 0 && m.aiModel == "" {
				m.aiModel = msg.models[0]
				m.aiModelIndex = 0
			}
		} else {
			m.aiTestResult = fmt.Sprintf("Connection failed: %s", msg.err.Error())
			m.aiModels = []string{}
		}
		return m, nil

	case ollamaInstallResultMsg:
		m.aiInstallingOllama = false
		if msg.success {
			m.aiOllamaInstalled = true
			m.aiInstallResult = "Ollama installed successfully!"
			// Auto-test connection after successful installation
			m.aiTesting = true
			m.aiTestResult = ""
			return m, testOllama(m.aiOllamaURL)
		} else {
			m.aiOllamaInstalled = false
			m.aiInstallResult = fmt.Sprintf("Installation failed: %s", msg.err.Error())
		}
		return m, nil

	case taskCompleteMsg:
		if msg.success {
			m.tasks[msg.index].status = statusComplete
		} else {
			if m.tasks[msg.index].optional {
				m.tasks[msg.index].status = statusSkipped
				m.errors = append(m.errors, fmt.Sprintf("%s (skipped): %s", m.tasks[msg.index].name, msg.error))
			} else {
				m.tasks[msg.index].status = statusFailed
				m.errors = append(m.errors, fmt.Sprintf("%s: %s", m.tasks[msg.index].name, msg.error))
				m.step = stepComplete
				return m, nil
			}
		}

		m.currentTaskIndex++
		if m.currentTaskIndex >= len(m.tasks) {
			// Check if we should run initial scan
			shouldScan := !m.uninstallMode && (!m.existingDBDetected || m.updateWithRefresh || m.forceWizard)
			if shouldScan && (m.tvLibraryDir != "" || m.movieLibraryDir != "") {
				m.step = stepInitialScan
				return m, tea.Batch(m.spinner.Tick, m.runInitialScan())
			}
			m.step = stepComplete
			return m, nil
		}

		m.tasks[m.currentTaskIndex].status = statusRunning
		return m, executeTask(m.currentTaskIndex, &m)

	case scanStartMsg:
		// Store cancel function for scan cancellation
		m.scanCancel = msg.cancel
		return m, nil

	case scanProgressMsg:
		m.scanProgress = msg.progress
		return m, nil

	case scanCompleteMsg:
		if msg.err != nil {
			m.errors = append(m.errors, fmt.Sprintf("Scan failed: %s", msg.err.Error()))
		}
		m.scanResult = msg.result
		m.scanStats = msg.stats
		m.exampleDupe = msg.dupe
		m.episodeCount = msg.episodeCount
		m.movieCount = msg.movieCount
		m.scanCancel = nil // Clear cancel function after scan completes
		m.step = stepScanEducation
		return m, nil
	}

	if len(m.inputs) > 0 && m.focusedInput >= 0 && m.focusedInput < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.focusedInput], cmd = m.inputs[m.focusedInput].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Check minimum terminal size
	const minWidth = 80
	const minHeight = 24
	if m.width < minWidth || m.height < minHeight {
		warningStyle := lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true).
			Align(lipgloss.Center, lipgloss.Center).
			Width(m.width).
			Height(m.height)

		warning := fmt.Sprintf(
			"Terminal too small!\n\nMinimum: %dx%d\nCurrent: %dx%d\n\nPlease resize your terminal.",
			minWidth, minHeight, m.width, m.height,
		)
		return warningStyle.Render(warning)
	}

	// Vertical ASCII sidebar - JELLYWATCH stacked vertically
	jellywatchVertical := `    ███
▄▄   ██
▀██▄▄██
  ▀▀▀▀▀

██████
██▄▄
██▄▄▄▄
▀▀▀▀▀▀

██
██
██▄▄██
▀▀▀▀▀▀

██
██
██▄▄██
▀▀▀▀▀▀

██   ██
██▄  ██
 ▀█████
     ▀▀

██   ██
██▄█▄██
███▀███
▀▀   ▀▀

 ▄█████
██▀  ██
███████
▀▀   ▀▀

██████
  ██
  ██
  ▀▀

 ▄█████
██▀
██▄▄▄▄▄
▀▀▀▀▀▀▀

██   ██
██▄▄▄██
██▀▀▀██
▀▀   ▀▀`

	verticalASCII := lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true).
		Padding(0, 1).
		Render(jellywatchVertical)

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(SuccessColor).
		Bold(true)

	title := "jellywatch installer"
	if m.uninstallMode {
		title = "jellywatch uninstaller"
	}
	titleRendered := titleStyle.Render(title) + "\n"

	// Main content based on step
	var mainContent string
	switch m.step {
	case stepWelcome:
		mainContent = m.renderWelcome()
	case stepUpdateNotice:
		mainContent = m.renderUpdateNotice()
	case stepUninstallConfirm:
		mainContent = m.renderUninstallConfirm()
	case stepWatchDirs:
		mainContent = m.renderWatchDirs()
	case stepLibraryPaths:
		mainContent = m.renderLibraryPaths()
	case stepSonarr:
		mainContent = m.renderSonarr()
	case stepRadarr:
		mainContent = m.renderRadarr()
	case stepAI:
		mainContent = m.renderAI()
	case stepPermissions:
		mainContent = m.renderPermissions()
	case stepConfirm:
		mainContent = m.renderConfirm()
	case stepInstalling:
		mainContent = m.renderInstalling()
	case stepInitialScan:
		mainContent = m.renderInitialScan()
	case stepScanEducation:
		mainContent = m.renderScanEducation()
	case stepComplete:
		mainContent = m.renderComplete()
	default:
		mainContent = "TODO: Other screens"
	}

	// Combine title and main content
	contentArea := titleRendered + mainContent

	// Wrap main content in border
	sidebarWidth := 10
	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Secondary).
		Width(m.width - sidebarWidth - 6)

	contentRendered := mainStyle.Render(contentArea)

	// Help text
	helpText := m.getHelpText()
	if helpText != "" {
		helpStyle := lipgloss.NewStyle().
			Foreground(FgMuted).
			Italic(true)
		contentRendered += "\n" + helpStyle.Render(helpText)
	}

	// Join sidebar and content horizontally
	layout := lipgloss.JoinHorizontal(
		lipgloss.Top,
		verticalASCII,
		contentRendered,
	)

	// Wrap everything in background with centering
	bgStyle := lipgloss.NewStyle().
		Background(BgBase).
		Foreground(FgPrimary).
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center)

	return bgStyle.Render(layout)
}

func (m model) renderWelcome() string {
	var content string

	content += "Select an option:\n\n"

	if m.existingDBDetected && !m.forceWizard {
		// Update mode - existing installation detected
		updatePrefix := "  "
		if m.selectedOption == 0 {
			updatePrefix = lipgloss.NewStyle().Foreground(Primary).Render("▸ ")
		}
		content += updatePrefix + "Update jellywatch\n"
		content += "    Reinstalls binaries, preserves configuration\n\n"

		uninstallPrefix := "  "
		if m.selectedOption == 1 {
			uninstallPrefix = lipgloss.NewStyle().Foreground(Primary).Render("▸ ")
		}
		content += uninstallPrefix + "Uninstall jellywatch\n"
		content += "    Removes jellywatch from your system\n\n"

		content += lipgloss.NewStyle().Foreground(FgMuted).Render(
			fmt.Sprintf("Database: %s\nLast modified: %s",
				m.existingDBPath,
				m.existingDBModTime.Format("2006-01-02 15:04")))
		content += "\n\n"
		content += lipgloss.NewStyle().Foreground(Secondary).Render("Press W to run first-run wizard")
	} else {
		// Fresh install mode
		installPrefix := "  "
		if m.selectedOption == 0 {
			installPrefix = lipgloss.NewStyle().Foreground(Primary).Render("▸ ")
		}
		content += installPrefix + "Install jellywatch\n"
		content += "    Builds binaries, installs to system, configures daemon\n\n"

		uninstallPrefix := "  "
		if m.selectedOption == 1 {
			uninstallPrefix = lipgloss.NewStyle().Foreground(Primary).Render("▸ ")
		}
		content += uninstallPrefix + "Uninstall jellywatch\n"
		content += "    Removes jellywatch from your system\n\n"

		content += lipgloss.NewStyle().Foreground(FgMuted).Render("Requires root privileges")
	}

	return content
}

func (m model) renderUpdateNotice() string {
	var content string

	content += lipgloss.NewStyle().Foreground(Primary).Bold(true).Render("Existing Installation Detected") + "\n\n"

	content += fmt.Sprintf("Database: %s\n", lipgloss.NewStyle().Foreground(FgMuted).Render(m.existingDBPath))
	content += fmt.Sprintf("Last modified: %s\n\n", lipgloss.NewStyle().Foreground(FgMuted).Render(m.existingDBModTime.Format("2006-01-02 15:04")))

	// Option 1: Update and refresh
	refreshPrefix := "  "
	if m.selectedOption == 0 {
		refreshPrefix = lipgloss.NewStyle().Foreground(Primary).Render("▸ ")
	}
	content += refreshPrefix + "Update and refresh database " + lipgloss.NewStyle().Foreground(SuccessColor).Render("(recommended)") + "\n"
	content += "    Reinstalls binaries, runs library scan\n\n"

	// Option 2: Update only
	updatePrefix := "  "
	if m.selectedOption == 1 {
		updatePrefix = lipgloss.NewStyle().Foreground(Primary).Render("▸ ")
	}
	content += updatePrefix + "Update only\n"
	content += "    Reinstalls binaries, keeps existing database\n\n"

	content += lipgloss.NewStyle().Foreground(Secondary).Render("Press W to run first-run wizard instead")

	return content
}

func (m model) renderUninstallConfirm() string {
	var content string

	content += lipgloss.NewStyle().Foreground(Primary).Bold(true).Render("Uninstall Confirmation") + "\n\n"

	content += lipgloss.NewStyle().Foreground(FgMuted).Render("The following will be removed:") + "\n\n"
	content += "  • jellywatch and jellywatchd binaries\n"
	content += "  • systemd service file\n\n"

	content += lipgloss.NewStyle().Foreground(FgMuted).Render("Choose what to do with the database:") + "\n\n"

	// Option 1: Remove database (default)
	removePrefix := "  "
	if m.selectedOption == 0 {
		removePrefix = lipgloss.NewStyle().Foreground(Primary).Render("▸ ")
	}
	content += removePrefix + "Remove database " + lipgloss.NewStyle().Foreground(SuccessColor).Render("(recommended for clean uninstall)") + "\n"
	content += "    Deletes learned data, allows fresh reinstall\n\n"

	// Option 2: Keep database
	keepPrefix := "  "
	if m.selectedOption == 1 {
		keepPrefix = lipgloss.NewStyle().Foreground(Primary).Render("▸ ")
	}
	content += keepPrefix + "Keep database\n"
	content += "    Preserves learned data for potential reinstall\n\n"

	content += lipgloss.NewStyle().Foreground(Secondary).Render("Note: config.toml will be preserved regardless of your choice.")

	return content
}

func (m *model) runInitialScan() tea.Cmd {
	return func() tea.Msg {
		// Create cancellable context for scan
		ctx, cancel := context.WithCancel(context.Background())

		// Send cancel function to model via message
		if m.program != nil {
			m.program.Send(scanStartMsg{cancel: cancel})
		}

		// Get the correct database path, respecting SUDO_USER
		configDir, err := getConfigDir()
		if err != nil {
			return scanCompleteMsg{err: fmt.Errorf("failed to get config dir: %w", err)}
		}
		dbPath := filepath.Join(configDir, "jellywatch", "media.db")

		db, err := database.OpenPath(dbPath)
		if err != nil {
			return scanCompleteMsg{err: err}
		}
		defer db.Close()

		fileScanner := scanner.NewFileScanner(db, nil)

		// Build library lists from config
		var tvLibs, movieLibs []string
		if m.tvLibraryDir != "" {
			tvLibs = strings.Split(m.tvLibraryDir, ",")
			for i := range tvLibs {
				tvLibs[i] = strings.TrimSpace(tvLibs[i])
			}
		}
		if m.movieLibraryDir != "" {
			movieLibs = strings.Split(m.movieLibraryDir, ",")
			for i := range movieLibs {
				movieLibs[i] = strings.TrimSpace(movieLibs[i])
			}
		}

		// Run scan with progress callback and cancellable context
		result, err := fileScanner.ScanWithOptions(ctx, scanner.ScanOptions{
			TVLibraries:    tvLibs,
			MovieLibraries: movieLibs,
			OnProgress: func(p scanner.ScanProgress) {
				if m.program != nil {
					m.program.Send(scanProgressMsg{
						progress: ScanProgress{
							FilesScanned:   p.FilesScanned,
							CurrentPath:    p.CurrentPath,
							LibrariesDone:  p.LibrariesDone,
							LibrariesTotal: p.LibrariesTotal,
						},
					})
				}
			},
		})

		if err != nil {
			return scanCompleteMsg{err: err}
		}

		// Get stats and example duplicate
		stats, err := db.GetConsolidationStats()
		if err != nil {
			// Log error but don't fail - stats are informational
			stats = nil
		}

		// Get actual episode and movie counts
		episodeCount, err := db.CountMediaFilesByType("episode")
		if err != nil {
			episodeCount = 0
		}
		movieCount, err := db.CountMediaFilesByType("movie")
		if err != nil {
			movieCount = 0
		}

		var exampleDupe *database.DuplicateGroup
		movieDupes, err := db.FindDuplicateMovies()
		if err == nil && len(movieDupes) > 0 {
			exampleDupe = &movieDupes[0]
		} else {
			episodeDupes, err := db.FindDuplicateEpisodes()
			if err == nil && len(episodeDupes) > 0 {
				exampleDupe = &episodeDupes[0]
			}
		}

		return scanCompleteMsg{
			result: &ScanResult{
				FilesScanned: result.FilesScanned,
				FilesAdded:   result.FilesAdded,
				Duration:     result.Duration,
				Errors:       result.Errors,
			},
			stats:        stats,
			dupe:         exampleDupe,
			episodeCount: episodeCount,
			movieCount:   movieCount,
		}
	}
}

func (m model) renderInitialScan() string {
	var content string

	content += lipgloss.NewStyle().Foreground(Primary).Bold(true).Render("Scanning Libraries...") + "\n\n"

	content += fmt.Sprintf("  Libraries: %d/%d complete\n",
		m.scanProgress.LibrariesDone, m.scanProgress.LibrariesTotal)
	content += fmt.Sprintf("  Files:     %d scanned\n", m.scanProgress.FilesScanned)

	// Truncate current path if too long
	currentPath := m.scanProgress.CurrentPath
	if len(currentPath) > 50 {
		currentPath = "..." + currentPath[len(currentPath)-47:]
	}
	content += fmt.Sprintf("  Current:   %s\n\n", lipgloss.NewStyle().Foreground(FgMuted).Render(currentPath))

	// Progress bar
	progress := 0.0
	if m.scanProgress.LibrariesTotal > 0 {
		progress = float64(m.scanProgress.LibrariesDone) / float64(m.scanProgress.LibrariesTotal)
	}
	barWidth := 40
	filled := int(progress * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	content += fmt.Sprintf("  [%s] %d%%\n", bar, int(progress*100))

	content += "\n" + m.spinner.View() + " Scanning..."

	return content
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (m model) renderScanEducation() string {
	var content string

	content += lipgloss.NewStyle().Foreground(SuccessColor).Bold(true).Render("✓ Library Scan Complete") + "\n\n"

	// Scan results section
	content += lipgloss.NewStyle().Foreground(Primary).Render("SCAN RESULTS") + "\n"
	content += lipgloss.NewStyle().Foreground(FgMuted).Render("───────────────") + "\n"

	if m.scanResult != nil {
		content += fmt.Sprintf("  Files scanned:    %d\n", m.scanResult.FilesScanned)
	}

	if m.scanStats != nil {
		// Use actual counts from database instead of estimates
		content += fmt.Sprintf("  TV episodes:      %d\n", m.episodeCount)
		content += fmt.Sprintf("  Movies:           %d\n", m.movieCount)
		content += fmt.Sprintf("  Duplicates found: %d\n", m.scanStats.DuplicateGroups)
	}
	content += "\n"

	// Example duplicate section
	if m.exampleDupe != nil && len(m.exampleDupe.Files) >= 2 {
		content += lipgloss.NewStyle().Foreground(Primary).Render("EXAMPLE DUPLICATE") + "\n"
		content += lipgloss.NewStyle().Foreground(FgMuted).Render("───────────────────") + "\n"

		// Title with year
		title := m.exampleDupe.NormalizedTitle
		if m.exampleDupe.Year != nil {
			title += fmt.Sprintf(" (%d)", *m.exampleDupe.Year)
		}
		content += "  " + title + "\n"

		// Best file (KEEP)
		best := m.exampleDupe.Files[0]
		bestPath := best.Path
		if len(bestPath) > 35 {
			bestPath = "..." + bestPath[len(bestPath)-32:]
		}
		content += fmt.Sprintf("    %s %s  %s  %s\n",
			lipgloss.NewStyle().Foreground(SuccessColor).Render("[KEEP]"),
			bestPath,
			best.Resolution,
			formatBytes(best.Size))

		// Inferior file (DELETE)
		inferior := m.exampleDupe.Files[1]
		inferiorPath := inferior.Path
		if len(inferiorPath) > 35 {
			inferiorPath = "..." + inferiorPath[len(inferiorPath)-32:]
		}
		content += fmt.Sprintf("    %s %s  %s  %s\n",
			lipgloss.NewStyle().Foreground(ErrorColor).Render("[DELETE]"),
			inferiorPath,
			inferior.Resolution,
			formatBytes(inferior.Size))
		content += "\n"
	} else {
		content += lipgloss.NewStyle().Foreground(SuccessColor).Render("  No duplicates detected - your library is clean!") + "\n\n"
	}

	// Workflow commands section
	content += lipgloss.NewStyle().Foreground(Primary).Render("WORKFLOW COMMANDS") + "\n"
	content += lipgloss.NewStyle().Foreground(FgMuted).Render("─────────────────") + "\n"
	content += fmt.Sprintf("  %s   Refresh database\n", lipgloss.NewStyle().Foreground(Secondary).Render("jellywatch scan"))
	content += fmt.Sprintf("  %s   List all duplicates\n", lipgloss.NewStyle().Foreground(Secondary).Render("jellywatch duplicates"))
	content += fmt.Sprintf("  %s   Manage duplicates\n", lipgloss.NewStyle().Foreground(Secondary).Render("jellywatch consolidate"))
	content += "\n"

	content += lipgloss.NewStyle().Foreground(FgMuted).Italic(true).Render(
		"Tip: Run 'jellywatch consolidate --dry-run' to preview changes.")

	return content
}

func (m model) renderWatchDirs() string {
	var content string

	content += lipgloss.NewStyle().Foreground(Primary).Bold(true).Render("Watch Directories") + "\n\n"
	content += "Enter the directories where JellyWatch should monitor for new downloads.\n"
	content += lipgloss.NewStyle().Foreground(FgMuted).Render("Leave empty to skip monitoring for that type.") + "\n\n"

	content += "TV Downloads:\n"
	if len(m.inputs) > 0 {
		content += m.inputs[0].View() + "\n\n"
	}

	content += "Movies Downloads:\n"
	if len(m.inputs) > 1 {
		content += m.inputs[1].View() + "\n"
	}

	return content
}

func (m model) renderLibraryPaths() string {
	var content string

	content += lipgloss.NewStyle().Foreground(Primary).Bold(true).Render("Library Paths") + "\n\n"
	content += "Enter the directories where organized media should be stored.\n"
	content += lipgloss.NewStyle().Foreground(FgMuted).Render("Leave empty to skip.") + "\n\n"

	content += "TV Library:\n"
	if len(m.inputs) > 0 {
		content += m.inputs[0].View() + "\n\n"
	}

	content += "Movies Library:\n"
	if len(m.inputs) > 1 {
		content += m.inputs[1].View() + "\n"
	}

	return content
}

func (m model) renderSonarr() string {
	var content string

	content += lipgloss.NewStyle().Foreground(Primary).Bold(true).Render("Sonarr Integration (Optional)") + "\n\n"
	content += "Configure Sonarr for TV show metadata and organization.\n\n"

	content += "Sonarr URL:\n"
	if len(m.inputs) > 0 {
		content += m.inputs[0].View() + "\n\n"
	}

	content += "API Key:\n"
	if len(m.inputs) > 1 {
		content += m.inputs[1].View() + "\n\n"
	}

	if m.testingAPI {
		content += m.spinner.View() + " Testing connection...\n"
	} else if m.sonarrTested {
		content += lipgloss.NewStyle().Foreground(SuccessColor).Render("✓ Connected! ") +
			lipgloss.NewStyle().Foreground(FgMuted).Render(fmt.Sprintf("(v%s)", m.sonarrVersion)) + "\n"
	} else if m.testError != "" {
		content += lipgloss.NewStyle().Foreground(ErrorColor).Render("✗ Connection failed: "+m.testError) + "\n"
	} else {
		content += lipgloss.NewStyle().Foreground(FgMuted).Render("Press 't' to test connection") + "\n"
	}

	return content
}

func (m model) renderRadarr() string {
	var content string

	content += lipgloss.NewStyle().Foreground(Primary).Bold(true).Render("Radarr Integration (Optional)") + "\n\n"
	content += "Configure Radarr for movie metadata and organization.\n\n"

	content += "Radarr URL:\n"
	if len(m.inputs) > 0 {
		content += m.inputs[0].View() + "\n\n"
	}

	content += "API Key:\n"
	if len(m.inputs) > 1 {
		content += m.inputs[1].View() + "\n\n"
	}

	if m.testingAPI {
		content += m.spinner.View() + " Testing connection...\n"
	} else if m.radarrTested {
		content += lipgloss.NewStyle().Foreground(SuccessColor).Render("✓ Connected! ") +
			lipgloss.NewStyle().Foreground(FgMuted).Render(fmt.Sprintf("(v%s)", m.radarrVersion)) + "\n"
	} else if m.testError != "" {
		content += lipgloss.NewStyle().Foreground(ErrorColor).Render("✗ Connection failed: "+m.testError) + "\n"
	} else {
		content += lipgloss.NewStyle().Foreground(FgMuted).Render("Press 't' to test connection") + "\n"
	}

	return content
}

func (m model) renderAI() string {
	var content string

	content += lipgloss.NewStyle().Foreground(Primary).Bold(true).Render("AI Configuration (Optional)") + "\n\n"
	content += "Configure local AI for improved title matching and edge case resolution.\n\n"

	// Enable/Disable toggle AT THE TOP
	checkbox := "[  ]"
	if m.aiEnabled {
		checkbox = lipgloss.NewStyle().Foreground(SuccessColor).Render("[✓]")
	}
	content += checkbox + " Enable fallback AI for improved title matching"
	if !m.aiModelDropdownOpen {
		content += lipgloss.NewStyle().Foreground(FgMuted).Render(" (Press 'e' to toggle)")
	}
	content += "\n"

	// Show configuration if enabled
	if m.aiEnabled {
		content += "\n"

		// Check Ollama installation first - if not installed, show only installation prompt
		if !m.aiOllamaInstalled && !m.aiInstallingOllama {
			distro := detectDistro()
			content += fmt.Sprintf("\n%s Ollama is required but not detected on this system (%s).\n",
				lipgloss.NewStyle().Foreground(ErrorColor).Render("⚠"),
				lipgloss.NewStyle().Foreground(FgMuted).Render(distro))
			content += lipgloss.NewStyle().Foreground(FgMuted).Render("Press 'i' to automatically install Ollama, or install it manually and press 'e' to disable AI") + "\n"
		} else if m.aiInstallingOllama {
			content += fmt.Sprintf("\n%s Installing Ollama...\n",
				m.spinner.View())
			content += lipgloss.NewStyle().Foreground(FgMuted).Render("This may take a few minutes.") + "\n"
		} else if m.aiInstallResult != "" {
			if strings.Contains(m.aiInstallResult, "successfully") {
				content += fmt.Sprintf("\n%s Ollama installed successfully!\n",
					lipgloss.NewStyle().Foreground(SuccessColor).Render("✓"))
				content += lipgloss.NewStyle().Foreground(FgMuted).Render(m.aiInstallResult) + "\n"
			} else {
				content += fmt.Sprintf("\n%s Installation failed: %s\n",
					lipgloss.NewStyle().Foreground(ErrorColor).Render("✗"),
					lipgloss.NewStyle().Foreground(FgMuted).Render(m.aiInstallResult))
			}
		} else {
			// Ollama IS installed - show configuration options
			content += fmt.Sprintf("\n%s Ollama detected on this system.\n",
				lipgloss.NewStyle().Foreground(SuccessColor).Render("✓"))
			content += "\n"

			// Ollama URL text input
			content += "Ollama URL:\n"
			if len(m.inputs) > 0 {
				content += m.inputs[0].View() + "\n\n"
			} else {
				// Fallback if inputs not initialized
				content += fmt.Sprintf("  %s\n\n", m.aiOllamaURL)
			}

			// Model dropdown
			content += "Model:\n"
			if m.aiModelDropdownOpen {
				// Show dropdown with all models
				dropdownBorder := lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(Primary)

				var dropdownItems string
				for i, modelName := range m.aiModels {
					prefix := "  "
					if i == m.aiModelIndex {
						prefix = lipgloss.NewStyle().Foreground(Primary).Render("▸ ")
						modelName = lipgloss.NewStyle().Foreground(Secondary).Bold(true).Render(modelName)
					}
					dropdownItems += prefix + modelName + "\n"
				}

				if len(m.aiModels) == 0 {
					dropdownItems = lipgloss.NewStyle().Foreground(FgMuted).Render("  None") + "\n"
				}

				content += dropdownBorder.Render(dropdownItems)
				content += "\n"
			} else {
				// Show current selection with dropdown indicator
				currentModel := m.aiModel
				if currentModel == "" {
					if m.aiTesting {
						currentModel = lipgloss.NewStyle().Foreground(FgMuted).Render("(Fetching models...)")
					} else if len(m.aiModels) > 0 {
						currentModel = lipgloss.NewStyle().Foreground(FgMuted).Render("(Press Enter to select model)")
					} else {
						currentModel = lipgloss.NewStyle().Foreground(FgMuted).Render("(None)")
					}
				}

				dropdownIndicator := "▼"
				if len(m.aiModels) > 0 {
					dropdownIndicator = lipgloss.NewStyle().Foreground(Secondary).Render("▼")
				} else {
					dropdownIndicator = lipgloss.NewStyle().Foreground(FgMuted).Render("▼")
				}

				content += fmt.Sprintf("  %s %s\n\n", currentModel, dropdownIndicator)
			}

			// Test connection result
			if m.aiTesting {
				content += m.spinner.View() + " Fetching available models...\n"
			} else if m.aiTestResult != "" {
				if strings.Contains(m.aiTestResult, "Found") {
					content += lipgloss.NewStyle().Foreground(SuccessColor).Render("✓ "+m.aiTestResult) + "\n"
					if len(m.aiModels) > 0 {
						content += lipgloss.NewStyle().Foreground(FgMuted).Render("Press Enter to select a model from dropdown") + "\n"
					}
				} else {
					content += lipgloss.NewStyle().Foreground(ErrorColor).Render("✗ "+m.aiTestResult) + "\n"
					content += lipgloss.NewStyle().Foreground(FgMuted).Render("Press 't' to retry connection") + "\n"
				}
			}

			// Show usage guidance at the bottom when enabled and models are available
			if len(m.aiModels) > 0 {
				content += "\n"
				content += lipgloss.NewStyle().Foreground(Secondary).Render("Getting Started:") + "\n"
				content += lipgloss.NewStyle().Foreground(FgMuted).Render("Download model: ollama pull <model>") + "\n"
				content += lipgloss.NewStyle().Foreground(FgMuted).Render("Recommended models: gemma3:4b & llama3.2:3b") + "\n"
			}
		}
	}

	return content
}

func (m model) renderPermissions() string {
	var content string

	content += lipgloss.NewStyle().Foreground(Primary).Bold(true).Render("File Permissions (Optional)") + "\n\n"
	content += "Set ownership and permissions for transferred files.\n"
	content += lipgloss.NewStyle().Foreground(FgMuted).Render("Configure this if Jellyfin runs as a different user.") + "\n\n"

	content += "Owner User:\n"
	if len(m.inputs) > 0 {
		content += m.inputs[0].View() + "\n\n"
	}

	content += "Owner Group:\n"
	if len(m.inputs) > 1 {
		content += m.inputs[1].View() + "\n\n"
	}

	content += "File Mode:                    Directory Mode:\n"
	if len(m.inputs) > 2 {
		content += m.inputs[2].View() + "                       "
	}
	if len(m.inputs) > 3 {
		content += m.inputs[3].View() + "\n\n"
	}

	content += lipgloss.NewStyle().Foreground(FgMuted).Render("Leave user/group empty to preserve source ownership.") + "\n"

	return content
}

func (m model) renderConfirm() string {
	var content string

	content += lipgloss.NewStyle().Foreground(Primary).Bold(true).Render("Confirm Installation") + "\n\n"
	content += "Review your configuration:\n\n"

	if m.tvWatchDir != "" || m.movieWatchDir != "" {
		content += lipgloss.NewStyle().Foreground(Secondary).Render("Watch Directories:") + "\n"
		if m.tvWatchDir != "" {
			content += "  TV:     " + m.tvWatchDir + "\n"
		}
		if m.movieWatchDir != "" {
			content += "  Movies: " + m.movieWatchDir + "\n"
		}
		content += "\n"
	}

	if m.tvLibraryDir != "" || m.movieLibraryDir != "" {
		content += lipgloss.NewStyle().Foreground(Secondary).Render("Library Paths:") + "\n"
		if m.tvLibraryDir != "" {
			content += "  TV:     " + m.tvLibraryDir + "\n"
		}
		if m.movieLibraryDir != "" {
			content += "  Movies: " + m.movieLibraryDir + "\n"
		}
		content += "\n"
	}

	if m.sonarrEnabled {
		status := "✓"
		if m.sonarrTested {
			status = lipgloss.NewStyle().Foreground(SuccessColor).Render("✓")
		}
		content += lipgloss.NewStyle().Foreground(Secondary).Render("Sonarr:") + " " + m.sonarrURL + " " + status + "\n"
	}

	if m.radarrEnabled {
		status := "✓"
		if m.radarrTested {
			status = lipgloss.NewStyle().Foreground(SuccessColor).Render("✓")
		}
		content += lipgloss.NewStyle().Foreground(Secondary).Render("Radarr:") + " " + m.radarrURL + " " + status + "\n"
	}

	if m.permUser != "" || m.permGroup != "" {
		content += "\n" + lipgloss.NewStyle().Foreground(Secondary).Render("Permissions:") + "\n"
		if m.permUser != "" {
			content += "  User:  " + m.permUser + "\n"
		}
		if m.permGroup != "" {
			content += "  Group: " + m.permGroup + "\n"
		}
		if m.permFileMode != "" {
			content += "  Files: " + m.permFileMode + "  Dirs: " + m.permDirMode + "\n"
		}
	}

	content += "\n"

	installPrefix := "  "
	if m.selectedOption == 0 {
		installPrefix = lipgloss.NewStyle().Foreground(Primary).Render("▸ ")
	}
	content += installPrefix + "Install\n\n"

	backPrefix := "  "
	if m.selectedOption == 1 {
		backPrefix = lipgloss.NewStyle().Foreground(Primary).Render("▸ ")
	}
	content += backPrefix + "Go back\n"

	return content
}

func (m model) renderInstalling() string {
	var content string

	title := "Installing JellyWatch"
	if m.uninstallMode {
		title = "Uninstalling JellyWatch"
	}
	content += lipgloss.NewStyle().Foreground(Primary).Bold(true).Render(title) + "\n\n"

	for i, task := range m.tasks {
		var statusMark string
		switch task.status {
		case statusPending:
			statusMark = lipgloss.NewStyle().Foreground(FgMuted).Render("[ ]")
		case statusRunning:
			statusMark = m.spinner.View()
		case statusComplete:
			statusMark = checkMark.String()
		case statusFailed:
			statusMark = failMark.String()
		case statusSkipped:
			statusMark = skipMark.String()
		}

		taskLine := fmt.Sprintf("%s %s", statusMark, task.description)
		if i == m.currentTaskIndex && task.status == statusRunning {
			taskLine = lipgloss.NewStyle().Foreground(Secondary).Render(taskLine)
		}
		content += taskLine + "\n"
	}

	if len(m.errors) > 0 {
		content += "\n" + lipgloss.NewStyle().Foreground(ErrorColor).Render("Errors:") + "\n"
		for _, err := range m.errors {
			content += "  " + err + "\n"
		}
	}

	return content
}

func (m model) renderComplete() string {
	var content string

	if m.uninstallMode {
		if len(m.errors) > 0 {
			content += lipgloss.NewStyle().Foreground(ErrorColor).Bold(true).Render("✗ Uninstallation failed") + "\n\n"
			content += lipgloss.NewStyle().Foreground(ErrorColor).Render("Errors:") + "\n"
			for _, err := range m.errors {
				content += "  " + err + "\n"
			}
		} else {
			content += lipgloss.NewStyle().Foreground(SuccessColor).Bold(true).Render("✓ Uninstallation complete!") + "\n\n"
			content += "JellyWatch has been removed from your system.\n\n"
			content += "Config files preserved at: " + lipgloss.NewStyle().Foreground(FgMuted).Render("~/.config/jellywatch/") + "\n\n"
		}
	} else {
		if len(m.errors) > 0 {
			content += lipgloss.NewStyle().Foreground(ErrorColor).Bold(true).Render("✗ Installation failed") + "\n\n"
			content += lipgloss.NewStyle().Foreground(ErrorColor).Render("Errors:") + "\n"
			for _, err := range m.errors {
				content += "  " + err + "\n"
			}
		} else if m.scanResult != nil {
			// Fresh install with scan completed
			content += lipgloss.NewStyle().Foreground(SuccessColor).Bold(true).Render("✓ Installation complete!") + "\n\n"
			content += "Your library has been scanned and is ready to use.\n\n"

			content += "Quick Commands:\n"
			if m.scanStats != nil && m.scanStats.DuplicateGroups > 0 {
				content += fmt.Sprintf("  %s   View %d duplicates found\n",
					lipgloss.NewStyle().Foreground(Secondary).Render("jellywatch duplicates"),
					m.scanStats.DuplicateGroups)
			}
			content += fmt.Sprintf("  %s   Manage duplicates\n", lipgloss.NewStyle().Foreground(Secondary).Render("jellywatch consolidate"))
			content += fmt.Sprintf("  %s   Check daemon\n\n", lipgloss.NewStyle().Foreground(Secondary).Render("systemctl status jellywatchd"))

			content += "Config: " + lipgloss.NewStyle().Foreground(FgMuted).Render("~/.config/jellywatch/config.toml") + "\n"
			content += "Database: " + lipgloss.NewStyle().Foreground(FgMuted).Render("~/.config/jellywatch/media.db") + "\n\n"
		} else if m.existingDBDetected && !m.forceWizard {
			// Update without scan
			content += lipgloss.NewStyle().Foreground(SuccessColor).Bold(true).Render("✓ Update complete!") + "\n\n"
			content += "Binaries updated. Run " + lipgloss.NewStyle().Foreground(Secondary).Render("jellywatch scan") + " if you want to refresh your database.\n\n"
			content += "Config: " + lipgloss.NewStyle().Foreground(FgMuted).Render("~/.config/jellywatch/config.toml") + "\n\n"
		} else {
			// Fresh install without scan (no libraries configured)
			content += lipgloss.NewStyle().Foreground(SuccessColor).Bold(true).Render("✓ Installation complete!") + "\n\n"
			content += "Get Started:\n\n"
			content += lipgloss.NewStyle().Foreground(Secondary).Render("  jellywatch") + "              Launch CLI\n"
			content += lipgloss.NewStyle().Foreground(Secondary).Render("  systemctl status jellywatchd") + "   Check daemon\n\n"
			content += "Config: " + lipgloss.NewStyle().Foreground(FgMuted).Render("~/.config/jellywatch/config.toml") + "\n\n"
		}
	}

	content += "Press Enter to exit"

	return content
}

func (m model) getHelpText() string {
	switch m.step {
	case stepWelcome:
		return "↑/↓: Navigate  •  Enter: Continue  •  Q/Ctrl+C: Quit"
	case stepUpdateNotice:
		return "↑/↓: Navigate  •  Enter: Continue  •  W: Run wizard  •  Esc: Back  •  Q: Quit"
	case stepUninstallConfirm:
		return "↑/↓: Navigate  •  Enter: Confirm  •  Esc: Back  •  Q: Quit"
	case stepWatchDirs, stepLibraryPaths:
		return "↑/↓: Switch field  •  Enter: Continue  •  Esc: Back  •  Q: Quit"
	case stepSonarr, stepRadarr:
		return "Tab: Next field  •  T: Test connection  •  Enter: Continue  •  Esc: Back  •  Q: Quit"
	case stepAI:
		helpText := "E: Toggle AI"
		if !m.aiOllamaInstalled && !m.aiInstallingOllama {
			helpText += "  •  I: Install Ollama"
		}
		if m.aiEnabled && !m.aiModelDropdownOpen && !m.aiInstallingOllama {
			helpText += "  •  T: Test connection"
		}
		if m.aiModelDropdownOpen {
			helpText = "↑/↓: Navigate  •  Enter: Select  •  Esc: Close  •  Q: Quit"
		} else if !m.aiInstallingOllama {
			if m.aiEnabled && len(m.aiModels) > 0 {
				helpText += "  •  M: Select model"
			}
			helpText += "  •  Enter: Continue  •  Q: Quit"
		} else {
			helpText += "  •  Q: Quit (installation in progress)"
		}
		return helpText
	case stepPermissions:
		return "↑/↓: Switch field  •  Enter: Continue  •  Esc: Back  •  Q: Quit"
	case stepConfirm:
		return "↑/↓: Navigate  •  Enter: Confirm  •  Esc: Back  •  Q: Quit"
	case stepInstalling:
		return "Installation in progress..."
	case stepInitialScan:
		return "Scanning libraries..."
	case stepScanEducation:
		return "Press Enter to continue"
	case stepComplete:
		return "Enter: Exit  •  Q/Ctrl+C: Quit"
	default:
		return "Installation in progress..."
	}
}

// discoverOllamaModels queries Ollama for available models
// This helper can be used by installer CLI or future TUI integration
func discoverOllamaModels(endpoint string) ([]string, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(endpoint + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Models {
		models = append(models, m.Name)
	}

	return models, nil
}

// testOllama tests connection to Ollama and fetches available models
func testOllama(url string) tea.Cmd {
	return func() tea.Msg {
		models, err := discoverOllamaModels(url)
		if err != nil {
			return ollamaTestResultMsg{
				success: false,
				models:  nil,
				err:     err,
			}
		}

		return ollamaTestResultMsg{
			success: true,
			models:  models,
			err:     nil,
		}
	}
}

// checkOllamaInstalled checks if Ollama command is available
func checkOllamaInstalled() bool {
	_, err := exec.LookPath("ollama")
	return err == nil
}

// detectDistro detects the Linux distribution
func detectDistro() string {
	// Try reading /etc/os-release first
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "ID=") {
				distroID := strings.TrimPrefix(line, "ID=")
				distroID = strings.Trim(distroID, "\"")
				return distroID
			}
		}
	}

	// Fallback: check for package managers
	if _, err := exec.LookPath("pacman"); err == nil {
		return "arch"
	}
	if _, err := exec.LookPath("apt-get"); err == nil {
		return "debian"
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return "fedora"
	}
	if _, err := exec.LookPath("zypper"); err == nil {
		return "opensuse"
	}

	return "unknown"
}

// installOllama installs Ollama using the appropriate method for the distro
func installOllama() tea.Cmd {
	return func() tea.Msg {
		distro := detectDistro()

		var cmd *exec.Cmd

		switch distro {
		case "arch", "manjaro", "endeavouros":
			// Arch-based: Check for yay or paru, otherwise use AUR helper script
			if _, err := exec.LookPath("yay"); err == nil {
				cmd = exec.Command("yay", "-S", "--noconfirm", "ollama-bin")
			} else if _, err := exec.LookPath("paru"); err == nil {
				cmd = exec.Command("paru", "-S", "--noconfirm", "ollama-bin")
			} else {
				// Fallback to official install script
				cmd = exec.Command("sh", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
			}

		case "debian", "ubuntu", "linuxmint", "pop":
			// Debian-based: Use official install script
			cmd = exec.Command("sh", "-c", "curl -fsSL https://ollama.com/install.sh | sh")

		case "fedora", "rhel", "centos":
			// RHEL-based: Use official install script
			cmd = exec.Command("sh", "-c", "curl -fsSL https://ollama.com/install.sh | sh")

		case "opensuse", "suse":
			// openSUSE: Use official install script
			cmd = exec.Command("sh", "-c", "curl -fsSL https://ollama.com/install.sh | sh")

		default:
			// Unknown distro: Try official install script as fallback
			cmd = exec.Command("sh", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			return ollamaInstallResultMsg{
				success: false,
				err:     fmt.Errorf("installation failed: %w\nOutput: %s", err, string(output)),
			}
		}

		// Verify installation
		if !checkOllamaInstalled() {
			return ollamaInstallResultMsg{
				success: false,
				err:     fmt.Errorf("installation completed but 'ollama' command not found"),
			}
		}

		return ollamaInstallResultMsg{
			success: true,
			err:     nil,
		}
	}
}

func main() {
	if os.Geteuid() != 0 {
		fmt.Println("Error: This installer must be run as root")
		fmt.Println("Please run: sudo", os.Args[0])
		os.Exit(1)
	}

	m := newModel()
	p := tea.NewProgram(&m, tea.WithAltScreen())
	m.program = p

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
