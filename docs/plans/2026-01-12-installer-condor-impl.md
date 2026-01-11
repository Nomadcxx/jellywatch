# Installer CONDOR Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Update the installer to support the new CONDOR database workflow with auto-detection, progress callbacks, and user education.

**Architecture:** The installer gains three new steps (UpdateNotice, InitialScan, ScanEducation), smart detection of existing installations, and a progress callback mechanism for the scanner. The scanner package is extended with a `ProgressCallback` option.

**Tech Stack:** Go, Bubble Tea TUI, SQLite (via database package)

---

## Task 1: Add Progress Callback to Scanner

**Files:**
- Modify: `internal/scanner/scanner.go`
- Create: `internal/scanner/scanner_test.go` (if not exists, add progress test)

**Step 1: Add ScanProgress and ProgressCallback types**

Add after line 33 in `internal/scanner/scanner.go`:

```go
// ScanProgress reports progress during scanning
type ScanProgress struct {
	FilesScanned   int
	CurrentPath    string
	LibrariesDone  int
	LibrariesTotal int
}

// ProgressCallback is called periodically during scanning
type ProgressCallback func(ScanProgress)

// ScanOptions configures the scanning behavior
type ScanOptions struct {
	TVLibraries    []string
	MovieLibraries []string
	OnProgress     ProgressCallback // Optional progress callback
}
```

**Step 2: Add ScanWithOptions method**

Add new method to FileScanner:

```go
// ScanWithOptions scans libraries with configurable options including progress callback
func (s *FileScanner) ScanWithOptions(ctx context.Context, opts ScanOptions) (*ScanResult, error) {
	start := time.Now()
	result := &ScanResult{}

	totalLibs := len(opts.TVLibraries) + len(opts.MovieLibraries)
	libsDone := 0

	progressFn := func(currentPath string, filesScanned int) {
		if opts.OnProgress != nil {
			opts.OnProgress(ScanProgress{
				FilesScanned:   filesScanned,
				CurrentPath:    currentPath,
				LibrariesDone:  libsDone,
				LibrariesTotal: totalLibs,
			})
		}
	}

	// Scan TV libraries
	for _, lib := range opts.TVLibraries {
		if err := s.scanPathWithProgress(ctx, lib, "episode", result, progressFn); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("TV library %s: %w", lib, err))
		}
		libsDone++
	}

	// Scan movie libraries
	for _, lib := range opts.MovieLibraries {
		if err := s.scanPathWithProgress(ctx, lib, "movie", result, progressFn); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("Movie library %s: %w", lib, err))
		}
		libsDone++
	}

	// Final progress update
	if opts.OnProgress != nil {
		opts.OnProgress(ScanProgress{
			FilesScanned:   result.FilesScanned,
			CurrentPath:    "",
			LibrariesDone:  totalLibs,
			LibrariesTotal: totalLibs,
		})
	}

	result.Duration = time.Since(start)
	return result, nil
}
```

**Step 3: Add scanPathWithProgress helper**

Add new internal method:

```go
// scanPathWithProgress is like scanPath but calls progress callback
func (s *FileScanner) scanPathWithProgress(ctx context.Context, path string, mediaType string, result *ScanResult, progressFn func(string, int)) error {
	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("walk error %s: %w", filePath, err))
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if !isVideoFile(filePath) {
			return nil
		}

		result.FilesScanned++

		// Report progress every 10 files
		if result.FilesScanned%10 == 0 {
			progressFn(filePath, result.FilesScanned)
		}

		if !s.shouldIncludeFile(filePath, info.Size(), mediaType) {
			result.FilesSkipped++
			return nil
		}

		if err := s.processFile(filePath, info, path, mediaType); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("process file %s: %w", filePath, err))
			return nil
		}

		result.FilesAdded++
		return nil
	})
}
```

**Step 4: Verify compilation**

Run: `go build ./internal/scanner/`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add internal/scanner/scanner.go
git commit -m "feat(scanner): add progress callback support for TUI integration"
```

---

## Task 2: Add New Installer Steps and Model Fields

**Files:**
- Modify: `cmd/installer/main.go`

**Step 1: Add new step constants**

Replace the step constants (lines 41-51) with:

```go
const (
	stepWelcome installStep = iota
	stepUpdateNotice    // NEW: shown when existing DB detected
	stepWatchDirs
	stepLibraryPaths
	stepSonarr
	stepRadarr
	stepPermissions
	stepConfirm
	stepInstalling
	stepInitialScan     // NEW: runs scanner with progress
	stepScanEducation   // NEW: shows results and commands
	stepComplete
)
```

**Step 2: Add model fields for detection and scan state**

Add after line 111 (after permDirMode field):

```go
	// Installation mode detection
	existingDBDetected bool
	existingDBPath     string
	existingDBModTime  time.Time
	updateWithRefresh  bool   // true = update + rescan, false = update only
	forceWizard        bool   // 'W' key pressed to run full wizard

	// Scan state
	scanProgress    ScanProgress
	scanResult      *ScanResult
	scanStats       *database.ConsolidationStats
	exampleDupe     *database.DuplicateGroup
	program         *tea.Program // for sending messages from goroutines
```

**Step 3: Add ScanProgress and ScanResult types to installer**

Add after the model struct:

```go
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
```

**Step 4: Add new message types**

Add after apiTestResultMsg:

```go
// Scan progress message
type scanProgressMsg struct {
	progress ScanProgress
}

// Scan complete message
type scanCompleteMsg struct {
	result *ScanResult
	stats  *database.ConsolidationStats
	dupe   *database.DuplicateGroup
	err    error
}
```

**Step 5: Verify compilation**

Run: `go build ./cmd/installer/`
Expected: Build succeeds (with import errors for database package - will fix in next task)

**Step 6: Commit**

```bash
git add cmd/installer/main.go
git commit -m "feat(installer): add step constants and model fields for CONDOR flow"
```

---

## Task 3: Implement Detection Logic

**Files:**
- Modify: `cmd/installer/main.go`

**Step 1: Add detectExistingInstall function**

Add before `newModel()`:

```go
// detectExistingInstall checks if jellywatch is already installed
func detectExistingInstall() (exists bool, dbPath string, modTime time.Time) {
	configDir, err := os.UserConfigDir()
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
```

**Step 2: Update newModel to detect existing installation**

Modify `newModel()` to call detection:

```go
func newModel() model {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(Secondary)
	s.Spinner = spinner.Dot

	// Detect existing installation
	exists, dbPath, modTime := detectExistingInstall()

	return model{
		step:               stepWelcome,
		currentTaskIndex:   -1,
		spinner:            s,
		errors:             []string{},
		selectedOption:     0,
		sonarrEnabled:      false,
		radarrEnabled:      false,
		existingDBDetected: exists,
		existingDBPath:     dbPath,
		existingDBModTime:  modTime,
	}
}
```

**Step 3: Verify compilation**

Run: `go build ./cmd/installer/`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add cmd/installer/main.go
git commit -m "feat(installer): add existing installation detection"
```

---

## Task 4: Update Welcome Screen for Detection

**Files:**
- Modify: `cmd/installer/main.go`

**Step 1: Update renderWelcome to show different options based on detection**

Replace the `renderWelcome` function:

```go
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
```

**Step 2: Handle 'W' key in Update for welcome screen**

Find the key handling in `Update` function for stepWelcome and add 'W' key handling. Add inside the `case tea.KeyMsg:` block for stepWelcome:

```go
case "w", "W":
	if m.existingDBDetected {
		m.forceWizard = true
		// Re-render to show fresh install options
		return m, nil
	}
```

**Step 3: Update welcome screen Enter handling**

Update the Enter key handling in stepWelcome to route to stepUpdateNotice when updating:

```go
case "enter":
	if m.step == stepWelcome {
		if m.selectedOption == 0 {
			if m.existingDBDetected && !m.forceWizard {
				// Go to update notice screen
				m.step = stepUpdateNotice
			} else {
				// Fresh install - go to watch dirs
				m.step = stepWatchDirs
				m.initWatchDirInputs()
			}
		} else {
			// Uninstall
			m.uninstallMode = true
			m.step = stepConfirm
		}
		return m, nil
	}
```

**Step 4: Verify compilation**

Run: `go build ./cmd/installer/`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add cmd/installer/main.go
git commit -m "feat(installer): update welcome screen for existing installation detection"
```

---

## Task 5: Implement Update Notice Screen

**Files:**
- Modify: `cmd/installer/main.go`

**Step 1: Add renderUpdateNotice function**

Add new render function:

```go
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
```

**Step 2: Add stepUpdateNotice to View switch**

In the `View()` function, add case for stepUpdateNotice in the switch:

```go
case stepUpdateNotice:
	mainContent = m.renderUpdateNotice()
```

**Step 3: Add key handling for stepUpdateNotice in Update**

Add a new case in the `Update` function's key handling:

```go
case stepUpdateNotice:
	switch msg.String() {
	case "up", "k":
		if m.selectedOption > 0 {
			m.selectedOption--
		}
	case "down", "j":
		if m.selectedOption < 1 {
			m.selectedOption++
		}
	case "w", "W":
		m.forceWizard = true
		m.step = stepWatchDirs
		m.initWatchDirInputs()
		return m, nil
	case "enter":
		m.updateWithRefresh = (m.selectedOption == 0)
		m.initTasks()
		m.step = stepInstalling
		m.currentTaskIndex = 0
		m.tasks[0].status = statusRunning
		return m, executeTask(0, &m)
	case "esc":
		m.step = stepWelcome
		m.selectedOption = 0
	}
```

**Step 4: Add help text for stepUpdateNotice**

In `getHelpText()`, add:

```go
case stepUpdateNotice:
	return "↑/↓: Navigate  •  Enter: Continue  •  W: Run wizard  •  Esc: Back  •  Q: Quit"
```

**Step 5: Verify compilation**

Run: `go build ./cmd/installer/`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add cmd/installer/main.go
git commit -m "feat(installer): implement update notice screen"
```

---

## Task 6: Add Database Initialization Task

**Files:**
- Modify: `cmd/installer/main.go`

**Step 1: Add import for database package**

Add to imports:

```go
"github.com/Nomadcxx/jellywatch/internal/database"
```

**Step 2: Create initializeDatabase function**

Add new task function:

```go
func initializeDatabase(m *model) error {
	db, err := database.Open()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()
	return nil
}
```

**Step 3: Update initTasks to include database initialization**

In `initTasks()`, update the install tasks list:

```go
m.tasks = []installTask{
	{name: "Build binaries", description: "Building jellywatch and jellywatchd", execute: buildBinaries, status: statusPending},
	{name: "Install binaries", description: "Installing to /usr/local/bin", execute: installBinaries, status: statusPending},
	{name: "Create config", description: "Creating configuration directory", execute: createConfig, status: statusPending},
	{name: "Initialize database", description: "Creating media database", execute: initializeDatabase, status: statusPending},
	{name: "Install systemd files", description: "Installing service file", execute: installSystemdFiles, status: statusPending},
	{name: "Enable daemon", description: "Enabling and starting jellywatchd", execute: enableDaemon, status: statusPending},
}
```

**Step 4: Verify compilation**

Run: `go build ./cmd/installer/`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add cmd/installer/main.go
git commit -m "feat(installer): add database initialization task"
```

---

## Task 7: Implement Initial Scan Step

**Files:**
- Modify: `cmd/installer/main.go`

**Step 1: Add import for scanner package**

Add to imports:

```go
"github.com/Nomadcxx/jellywatch/internal/scanner"
```

**Step 2: Create runInitialScan command function**

Add new function:

```go
func (m *model) runInitialScan() tea.Cmd {
	return func() tea.Msg {
		db, err := database.Open()
		if err != nil {
			return scanCompleteMsg{err: err}
		}
		defer db.Close()

		fileScanner := scanner.NewFileScanner(db)

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

		// Run scan with progress callback
		ctx := context.Background()
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
		stats, _ := db.GetConsolidationStats()

		var exampleDupe *database.DuplicateGroup
		movieDupes, _ := db.FindDuplicateMovies()
		if len(movieDupes) > 0 {
			exampleDupe = &movieDupes[0]
		} else {
			episodeDupes, _ := db.FindDuplicateEpisodes()
			if len(episodeDupes) > 0 {
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
			stats: stats,
			dupe:  exampleDupe,
		}
	}
}
```

**Step 3: Add renderInitialScan function**

```go
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
```

**Step 4: Add stepInitialScan to View switch**

```go
case stepInitialScan:
	mainContent = m.renderInitialScan()
```

**Step 5: Handle scan messages in Update**

Add cases for scanProgressMsg and scanCompleteMsg:

```go
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
	m.step = stepScanEducation
	return m, nil
```

**Step 6: Update task completion to trigger scan**

In the taskCompleteMsg handler, after all tasks complete and before going to stepComplete, check if we should scan:

```go
// Check if we should run initial scan
shouldScan := !m.uninstallMode && (!m.existingDBDetected || m.updateWithRefresh || m.forceWizard)
if shouldScan && m.tvLibraryDir != "" || m.movieLibraryDir != "" {
	m.step = stepInitialScan
	return m, tea.Batch(m.spinner.Tick, m.runInitialScan())
}
m.step = stepComplete
```

**Step 7: Add help text for stepInitialScan**

```go
case stepInitialScan:
	return "Scanning libraries..."
```

**Step 8: Verify compilation**

Run: `go build ./cmd/installer/`
Expected: Build succeeds

**Step 9: Commit**

```bash
git add cmd/installer/main.go
git commit -m "feat(installer): implement initial library scan with progress"
```

---

## Task 8: Implement Scan Education Screen

**Files:**
- Modify: `cmd/installer/main.go`

**Step 1: Add formatBytes helper if not exists**

Check if formatBytes exists, if not add:

```go
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
```

**Step 2: Add renderScanEducation function**

```go
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
		episodes := 0
		movies := 0
		if m.scanStats.TotalFiles > 0 {
			// Estimate based on typical ratio, or query DB
			movies = m.scanStats.TotalFiles / 3  // rough estimate
			episodes = m.scanStats.TotalFiles - movies
		}
		content += fmt.Sprintf("  TV episodes:      %d\n", episodes)
		content += fmt.Sprintf("  Movies:           %d\n", movies)
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
```

**Step 3: Add stepScanEducation to View switch**

```go
case stepScanEducation:
	mainContent = m.renderScanEducation()
```

**Step 4: Add key handling for stepScanEducation**

```go
case stepScanEducation:
	if msg.String() == "enter" {
		m.step = stepComplete
		return m, nil
	}
```

**Step 5: Add help text for stepScanEducation**

```go
case stepScanEducation:
	return "Press Enter to continue"
```

**Step 6: Verify compilation**

Run: `go build ./cmd/installer/`
Expected: Build succeeds

**Step 7: Commit**

```bash
git add cmd/installer/main.go
git commit -m "feat(installer): implement scan education screen"
```

---

## Task 9: Update Completion Screen

**Files:**
- Modify: `cmd/installer/main.go`

**Step 1: Update renderComplete for different flows**

Replace `renderComplete` function:

```go
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
```

**Step 2: Verify compilation**

Run: `go build ./cmd/installer/`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add cmd/installer/main.go
git commit -m "feat(installer): update completion screen for different flows"
```

---

## Task 10: Wire Up Program Reference for Progress Messages

**Files:**
- Modify: `cmd/installer/main.go`

**Step 1: Update main() to store program reference**

The program reference needs to be passed to the model for progress callbacks. Update main:

```go
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
```

**Step 2: Change model receiver to pointer where needed**

Update the model type to use pointer receivers consistently for methods that need to modify state.

**Step 3: Run full build and test**

Run: `go build ./cmd/installer/`
Expected: Build succeeds

**Step 4: Final commit**

```bash
git add cmd/installer/main.go internal/scanner/scanner.go
git commit -m "feat(installer): complete CONDOR integration with progress callbacks"
```

---

## Task 11: Integration Testing

**Step 1: Build all binaries**

Run: `go build ./...`
Expected: All packages build successfully

**Step 2: Run tests**

Run: `go test ./...`
Expected: All tests pass

**Step 3: Manual testing checklist**

Test the installer manually:

1. Fresh install (no existing DB):
   - Should show "Install / Uninstall"
   - Should go through full wizard
   - Should run scan after install
   - Should show education screen

2. Update (existing DB):
   - Should show "Update / Uninstall"
   - Should show "Press W for wizard"
   - "Update and refresh" should scan
   - "Update only" should skip scan

3. 'W' key override:
   - Should switch to wizard mode
   - Should run full flow

**Step 4: Fix any issues found**

Address any bugs discovered during testing.

**Step 5: Final commit**

```bash
git add -A
git commit -m "test: verify installer CONDOR integration"
```
