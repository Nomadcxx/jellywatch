# Installer CONDOR Integration Design

**Date:** 2026-01-12
**Status:** Approved
**Scope:** Update installer to support CONDOR database workflow

## Overview

The installer needs to be updated to support the new CONDOR (Comprehensive Database-Oriented Refactor) workflow. This includes database initialization, initial library scanning with progress, and user education about the new commands.

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Detection | Auto-detect with override | Smart defaults, but users can run wizard via 'W' key |
| Wizard content | Scan + education + live examples | Concrete value demonstration |
| Scan execution | Progress callback | Real-time feedback during potentially long operation |
| Update flow | Offer database refresh | Keeps database current after updates |

## Installation Flows

### Fresh Install (no database detected)

```
Welcome → WatchDirs → LibraryPaths → Sonarr → Radarr → Permissions → Confirm → Installing → InitialScan → ScanEducation → Complete
```

### Update (database exists)

```
Welcome → (detect DB) → UpdateNotice → [Optional: Scan + Education] → Installing → Complete
```

### Uninstall (unchanged)

```
Welcome → Confirm → Uninstalling → Complete
```

## New Steps

### stepUpdateNotice

Shown when existing database detected at `~/.config/jellywatch/media.db`.

```
┌─────────────────────────────────────────────────────────┐
│  Existing Installation Detected                         │
│                                                         │
│  Database: ~/.config/jellywatch/media.db                │
│  Last modified: 2 days ago                              │
│                                                         │
│  ▸ Update and refresh database (recommended)            │
│      Reinstalls binaries, runs library scan             │
│                                                         │
│    Update only                                          │
│      Reinstalls binaries, keeps existing database       │
│                                                         │
│                                                         │
│  Press W to run first-run wizard instead                │
└─────────────────────────────────────────────────────────┘
```

**Outcomes:**
- "Update and refresh" → Installing → InitialScan → ScanEducation → Complete
- "Update only" → Installing → Complete
- Press 'W' → WatchDirs (full wizard flow)

### stepInitialScan

Runs the library scanner with real-time progress updates.

```
┌─────────────────────────────────────────────────────────┐
│  Scanning libraries...                                  │
│                                                         │
│    Libraries: 2/4 complete                              │
│    Files:     1,234 scanned                             │
│    Current:   /media/TV/Breaking Bad (2008)/Season 01/  │
│                                                         │
│    [████████░░░░░░░░] 48%                               │
└─────────────────────────────────────────────────────────┘
```

### stepScanEducation

Shows scan results and educates users about the new workflow.

```
┌─────────────────────────────────────────────────────────┐
│  ✓ Library Scan Complete                                │
│                                                         │
│  SCAN RESULTS                                           │
│  ───────────────                                        │
│    Files scanned:    3,847                              │
│    TV episodes:      2,156                              │
│    Movies:           1,691                              │
│    Duplicates found: 23                                 │
│                                                         │
│  EXAMPLE DUPLICATE                                      │
│  ───────────────────                                    │
│  Interstellar (2014)                                    │
│    [KEEP]   /media/MOVIES1/...  2160p REMUX  45.2 GB   │
│    [DELETE] /media/MOVIES2/...  1080p WEB-DL  8.1 GB   │
│                                                         │
│  WORKFLOW COMMANDS                                      │
│  ─────────────────                                      │
│    jellywatch scan           Refresh database           │
│    jellywatch duplicates     List all duplicates        │
│    jellywatch consolidate    Manage duplicates          │
│                                                         │
│  Tip: Run 'jellywatch consolidate --dry-run' to        │
│       preview changes before executing.                 │
└─────────────────────────────────────────────────────────┘
```

If no duplicates found:
```
  No duplicates detected - your library is clean!
```

## Scanner Progress Callback

### New Scanner Interface

```go
// ProgressCallback reports scan progress
type ProgressCallback func(stats ScanProgress)

type ScanProgress struct {
    FilesScanned   int
    CurrentPath    string      // Current file/directory being processed
    LibrariesDone  int
    LibrariesTotal int
}

// ScanOptions extends existing options
type ScanOptions struct {
    Libraries  []string
    OnProgress ProgressCallback  // Optional, called periodically
    // ... existing fields
}
```

### Installer Integration

```go
func (m *model) runInitialScan() tea.Cmd {
    return func() tea.Msg {
        scanner := scanner.New(m.getLibraryPaths())

        err := scanner.Scan(scanner.ScanOptions{
            OnProgress: func(p scanner.ScanProgress) {
                // Send progress to TUI via program.Send()
                m.program.Send(scanProgressMsg{p})
            },
        })

        return scanCompleteMsg{stats: scanner.Stats(), err: err}
    }
}
```

Reference jellysink TUI for existing progress patterns.

## Installation Tasks

### Fresh Install Tasks

1. Build binaries
2. Install binaries
3. Create config
4. **Initialize database** (NEW)
5. Install systemd files
6. Enable daemon

### Initialize Database Task

```go
func initializeDatabase(m *model) error {
    db, err := database.Open()  // Creates DB + runs migrations
    if err != nil {
        return err
    }
    defer db.Close()
    return nil
}
```

## Completion Screen Updates

### Fresh Install (after scan)

```
✓ Installation complete!

Your library has been scanned and is ready to use.

Quick Commands:
  jellywatch duplicates         View 23 duplicates found
  jellywatch consolidate        Manage duplicates
  systemctl status jellywatchd  Check daemon

Config: ~/.config/jellywatch/config.toml
Database: ~/.config/jellywatch/media.db
```

### Update (no scan)

```
✓ Update complete!

Binaries updated. Run 'jellywatch scan' if you want to refresh your database.

Config: ~/.config/jellywatch/config.toml
```

## Detection Logic

```go
func detectExistingInstall() (exists bool, dbPath string) {
    configDir, _ := os.UserConfigDir()
    dbPath = filepath.Join(configDir, "jellywatch", "media.db")

    if _, err := os.Stat(dbPath); err == nil {
        return true, dbPath
    }
    return false, dbPath
}
```

## Welcome Screen Changes

The welcome screen gains smart detection:
- If `~/.config/jellywatch/media.db` exists → shows "Update / Uninstall" options
- If database doesn't exist → shows "Install / Uninstall" options (current behavior)

When update is detected, show keyboard hint: *"Press W to run first-run wizard"*

## Implementation Checklist

- [ ] Add `stepUpdateNotice`, `stepInitialScan`, `stepScanEducation` steps
- [ ] Add detection logic in `newModel()` or `Init()`
- [ ] Update welcome screen for update detection
- [ ] Add "Initialize database" install task
- [ ] Add scanner progress callback to `internal/scanner`
- [ ] Implement scan progress UI (reference jellysink)
- [ ] Implement education screen with duplicate query
- [ ] Update completion screens for fresh/update flows
- [ ] Handle 'W' key override for wizard
