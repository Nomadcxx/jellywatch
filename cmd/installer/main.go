package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Theme colors - Jellyfin
var (
	Primary      = lipgloss.Color("#AA5CC3") // Purple (gradient start)
	Secondary    = lipgloss.Color("#00A4DC") // Cyan/Blue (gradient end)
	BgBase       = lipgloss.Color("#101010") // Dark background
	FgPrimary    = lipgloss.Color("#FFFFFF") // White text
	FgMuted      = lipgloss.Color("#888888") // Muted text
	ErrorColor   = lipgloss.Color("#FF5555") // Red for errors
	SuccessColor = lipgloss.Color("#00A4DC") // Cyan for success
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
	stepWatchDirs
	stepLibraryPaths
	stepSonarr
	stepRadarr
	stepPermissions
	stepConfirm
	stepInstalling
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

// Initialize new model
func newModel() model {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(Secondary)
	s.Spinner = spinner.Dot

	return model{
		step:             stepWelcome,
		currentTaskIndex: -1,
		spinner:          s,
		errors:           []string{},
		selectedOption:   0,
		// Default values
		sonarrEnabled: false,
		radarrEnabled: false,
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

func (m *model) initTasks() {
	if m.uninstallMode {
		m.tasks = []installTask{
			{name: "Stop daemon", description: "Stopping jellywatchd service", execute: stopDaemon, status: statusPending, optional: true},
			{name: "Remove binaries", description: "Removing /usr/local/bin/jellywatch*", execute: removeBinaries, status: statusPending},
			{name: "Remove systemd files", description: "Removing systemd service file", execute: removeSystemdFiles, status: statusPending},
		}
	} else {
		m.tasks = []installTask{
			{name: "Build binaries", description: "Building jellywatch and jellywatchd", execute: buildBinaries, status: statusPending},
			{name: "Install binaries", description: "Installing to /usr/local/bin", execute: installBinaries, status: statusPending},
			{name: "Create config", description: "Creating configuration directory", execute: createConfig, status: statusPending},
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
		sb.WriteString(fmt.Sprintf("url = \"%s\"\n", m.sonarrURL))
		sb.WriteString(fmt.Sprintf("api_key = \"%s\"\n", m.sonarrAPIKey))
		sb.WriteString("\n")
	}

	if m.radarrEnabled {
		sb.WriteString("[radarr]\n")
		sb.WriteString(fmt.Sprintf("url = \"%s\"\n", m.radarrURL))
		sb.WriteString(fmt.Sprintf("api_key = \"%s\"\n", m.radarrAPIKey))
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

	cmd = exec.Command("systemctl", "start", "jellywatchd")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

func stopDaemon(m *model) error {
	cmd := exec.Command("systemctl", "stop", "jellywatchd")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	cmd = exec.Command("systemctl", "disable", "jellywatchd")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to disable service: %w", err)
	}

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

	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

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
			if m.step != stepInstalling {
				return m, tea.Quit
			}
		case "up", "k":
			if m.step == stepWelcome && m.selectedOption > 0 {
				m.selectedOption--
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
			} else if m.step == stepSonarr || m.step == stepRadarr {
				if m.focusedInput == 1 && m.selectedOption < 1 {
					m.selectedOption++
				}
			} else if m.step == stepConfirm && m.selectedOption < 1 {
				m.selectedOption++
			} else if len(m.inputs) > 0 && m.focusedInput < len(m.inputs)-1 {
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
			}
		case "enter":
			if m.step == stepWelcome {
				m.uninstallMode = m.selectedOption == 1
				if !m.uninstallMode {
					m.step = stepWatchDirs
					m.initInputsForStep()
					return m, nil
				} else {
					m.initTasks()
					m.step = stepInstalling
					m.currentTaskIndex = 0
					m.tasks[0].status = statusRunning
					return m, tea.Batch(
						m.spinner.Tick,
						executeTask(0, &m),
					)
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
				m.step = stepPermissions
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
			} else if m.step == stepComplete {
				return m, tea.Quit
			}
		case "esc":
			if m.step != stepWelcome && m.step != stepInstalling && m.step != stepComplete {
				prevStep := m.step - 1
				if prevStep < stepWelcome {
					prevStep = stepWelcome
				}
				m.step = prevStep
				if m.step != stepWelcome && m.step != stepConfirm {
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
			m.step = stepComplete
			return m, nil
		}

		m.tasks[m.currentTaskIndex].status = statusRunning
		return m, executeTask(m.currentTaskIndex, &m)
	}

	if len(m.inputs) > 0 && m.focusedInput < len(m.inputs) {
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

	var content string

	// ASCII Header
	jellywatchASCII := `  ▀▀        ▀██   ▀██                        ▄▄          ██    
 ▀██ ██▀▀██  ██    ██  ██  ██ ██ ▄ █ ▀▀▀▀██ ▀██▀▀ ██▀▀██ ██▀▀██
  ██ ██▀▀▀▀  ██    ██  ██  ██ ██▄█▄█ ██▀▀██  ██   ██  ▄▄ ██  ██
  ██ ▀▀▀▀▀▀ ▀▀▀▀  ▀▀▀▀ ▀▀▀▀██ ▀▀▀▀▀▀ ▀▀▀▀▀▀  ▀▀▀▀ ▀▀▀▀▀▀ ▀▀  ▀▀
▀▀▀▀                   ▀▀▀▀▀▀                                  `

	content += headerStyle.Render(jellywatchASCII) + "\n\n"

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(SuccessColor).
		Bold(true).
		Align(lipgloss.Center)

	title := "jellywatch installer"
	if m.uninstallMode {
		title = "jellywatch uninstaller"
	}
	content += titleStyle.Render(title) + "\n\n"

	// Main content based on step
	var mainContent string
	switch m.step {
	case stepWelcome:
		mainContent = m.renderWelcome()
	case stepWatchDirs:
		mainContent = m.renderWatchDirs()
	case stepLibraryPaths:
		mainContent = m.renderLibraryPaths()
	case stepSonarr:
		mainContent = m.renderSonarr()
	case stepRadarr:
		mainContent = m.renderRadarr()
	case stepPermissions:
		mainContent = m.renderPermissions()
	case stepConfirm:
		mainContent = m.renderConfirm()
	case stepInstalling:
		mainContent = m.renderInstalling()
	case stepComplete:
		mainContent = m.renderComplete()
	default:
		mainContent = "TODO: Other screens"
	}

	// Wrap in border
	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Width(m.width - 4)
	content += mainStyle.Render(mainContent) + "\n"

	// Help text
	helpText := m.getHelpText()
	if helpText != "" {
		helpStyle := lipgloss.NewStyle().
			Foreground(FgMuted).
			Italic(true).
			Align(lipgloss.Center)
		content += "\n" + helpStyle.Render(helpText)
	}

	// Wrap everything in background with centering
	bgStyle := lipgloss.NewStyle().
		Background(BgBase).
		Foreground(FgPrimary).
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Top)

	return bgStyle.Render(content)
}

func (m model) renderWelcome() string {
	var content string

	content += "Select an option:\n\n"

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
		} else {
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
	case stepWatchDirs, stepLibraryPaths:
		return "↑/↓: Switch field  •  Enter: Continue  •  Esc: Back  •  Q: Quit"
	case stepSonarr, stepRadarr:
		return "Tab: Next field  •  T: Test connection  •  Enter: Continue  •  Esc: Back  •  Q: Quit"
	case stepPermissions:
		return "↑/↓: Switch field  •  Enter: Continue  •  Esc: Back  •  Q: Quit"
	case stepConfirm:
		return "↑/↓: Navigate  •  Enter: Confirm  •  Esc: Back  •  Q: Quit"
	case stepInstalling:
		return "Installation in progress..."
	case stepComplete:
		return "Enter: Exit  •  Q/Ctrl+C: Quit"
	default:
		return "Installation in progress..."
	}
}

func main() {
	if os.Geteuid() != 0 {
		fmt.Println("Error: This installer must be run as root")
		fmt.Println("Please run: sudo", os.Args[0])
		os.Exit(1)
	}

	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
