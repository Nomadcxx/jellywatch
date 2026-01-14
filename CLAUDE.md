# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

JellyWatch is a Go-based media file watcher and organizer for Jellyfin libraries. It monitors download directories and automatically organizes media files according to Jellyfin naming conventions. The project includes an interactive CLI tool, a background daemon service, and integrations with Sonarr/Radarr APIs.

## Build and Development Commands

### Build
```bash
# Build CLI tool
go build -o jellywatch ./cmd/jellywatch

# Build daemon
go build -o jellywatchd ./cmd/jellywatchd

# Build interactive installer
go build -o installer ./cmd/installer
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run tests for a specific package
go test ./internal/naming/

# Run specific test
go test -run TestParseTVEpisode ./internal/naming/

# Run benchmarks
go test -bench=. ./internal/naming/
```

### Running
```bash
# Interactive CLI
./jellywatch

# Watch directory
./jellywatch watch /path/to/downloads --tv-library /media/TV

# Organize files
./jellywatch organize /path/to/source --library /media/Movies

# Validate compliance
./jellywatch validate /media/Movies

# Compliance checking and fixing
./jellywatch compliance                # List non-compliant files
./jellywatch compliance --fix-dry      # Preview fixes
./jellywatch compliance --fix          # Execute safe fixes

# Review skipped items
./jellywatch review                    # List pending items
./jellywatch review --summary          # Show statistics
./jellywatch review --retry-ai         # Retry with AI

# Dry run mode (preview changes)
./jellywatch organize --dry-run /path/to/source

# Daemon
./jellywatchd --config ~/.config/jellywatch/config.toml
```

### Systemd Service
```bash
# View logs
journalctl -u jellywatchd -f

# Service management
sudo systemctl start jellywatchd
sudo systemctl status jellywatchd
sudo systemctl restart jellywatchd
```

## Architecture

### High-Level Data Flow

1. **File Detection** → Watcher (`internal/watcher/`) monitors directories using fsnotify
2. **Naming Analysis** → Naming package (`internal/naming/`) parses filename to extract metadata
3. **AI Enhancement** (optional) → AI matcher (`internal/ai/`) improves title extraction for edge cases
4. **Library Selection** → Selector (`internal/library/`) determines target library using:
   - HOLDEN database (self-learning system, fastest)
   - Sonarr/Radarr API cache (fallback)
   - Heuristics (episode count, file size)
5. **File Transfer** → Transfer package (`internal/transfer/`) uses rsync backend with timeout handling
6. **Database Update** → HOLDEN (`internal/database/`) learns from successful organizations
7. **Notifications** → Notifies Sonarr/Radarr to trigger import scans

### Key Components

#### Naming Package (`internal/naming/`)
**Purpose:** Core logic for parsing filenames and detecting media types

- Extensive regex patterns for release markers, quality indicators, metadata
- TV episode detection: `IsTVEpisodeFilename()`, `ParseTVEpisode()`
- Movie detection: `IsMovieFilename()`, `ParseMovie()`
- AI integration for edge cases (obfuscated names, complex punctuation)
- **Critical:** All parsing must preserve year information and remove release group markers

#### Organizer Package (`internal/organizer/`)
**Purpose:** Orchestrates file organization workflow

- `OrganizeTVEpisode()` and `OrganizeMovie()` - main entry points
- `OrganizeFolder()` - intelligent folder analysis (detects samples, junk, subtitles)
- Quality comparison using `internal/quality/` package
- Duplicate detection and handling
- Self-learning: Updates HOLDEN database after successful organization

#### Transfer Package (`internal/transfer/`)
**Purpose:** Robust file transfers with timeout and retry logic

- **CRITICAL:** Never use `os.Rename()` or `io.Copy()` directly - they hang indefinitely on failing disks
- Backends: `rsync` (default), `pv`, `native`
- Timeout handling, checksum verification, progress tracking
- Permission management (chown/chmod via rsync flags)

#### Library Package (`internal/library/`)
**Purpose:** Intelligent library selection for multi-drive setups

- `Selector` type with prioritized selection logic:
  1. **Database canonical path** (HOLDEN) - `<1ms` lookup
  2. **Sonarr/Radarr cache** - `~100ms` warm cache, `1-5s` refresh
  3. **Episode count heuristic** - counts existing episodes
  4. **Free space** - picks library with sufficient space
- `Scanner` type for fast library scanning with caching

#### Database Package (`internal/database/`) - HOLDEN System
**Purpose:** Home-Operated Library Database Engine - Self-learning database that makes JellyWatch authoritative

**Key Principle:** JellyWatch-learned paths are authoritative. When JellyWatch learns a path that differs from Sonarr/Radarr, JellyWatch updates THEM, not the other way around.

Core features:
- **Sub-millisecond lookups** - Local SQLite queries instead of network API calls
- **Source priority system** - jellywatch (100) > filesystem (50) > sonarr/radarr (25)
- **Self-learning** - Updates database after each successful file organization
- **Conflict detection** - Identifies shows split across multiple drives
- **Daily sync** - Scheduled updates from Sonarr/Radarr (default 3:00 AM)
- **Graceful degradation** - Works perfectly even if Sonarr/Radarr offline

Database location: `~/.config/jellywatch/media.db`

Tables:
- `series` - TV shows with canonical paths, episode counts, external IDs
- `movies` - Movies with canonical paths, external IDs
- `aliases` - Alternative titles for fuzzy matching
- `conflicts` - Tracks shows in multiple locations
- `sync_log` - History of sync operations
- `schema_version` - Migration tracking

#### AI Package (`internal/ai/`)
**Purpose:** AI-powered title matching for edge cases

- Supports local models (Ollama) and cloud models (configurable)
- Achieves 100% accuracy on edge cases vs 52% regex baseline
- Database caching makes repeated lookups instant
- Graceful fallback to regex if AI unavailable
- **Config:** `[ai]` section in `config.toml`

#### Config Package (`internal/config/`)
**Purpose:** Configuration management using Viper (TOML)

Main sections:
- `[watch]` - directories to monitor
- `[libraries]` - destination library paths
- `[permissions]` - file ownership and mode (uid/gid/chmod via rsync)
- `[sonarr]` / `[radarr]` - API integration
- `[ai]` - AI title matching configuration
- `[logging]` - structured logging configuration

#### Sync Package (`internal/sync/`)
**Purpose:** Daily synchronization from external sources to populate HOLDEN database

- `SyncService` - Scheduler and coordinator for sync operations
- Sonarr sync - Imports series data from Sonarr API
- Radarr sync - Imports movie data from Radarr API
- Filesystem sync - Scans actual library directories
- Respects source priority - never overwrites JellyWatch-learned paths
- Runs at configurable hour (default 3:00 AM local time)
- Updates Sonarr/Radarr when JellyWatch path differs

#### Consolidate Package (`internal/consolidate/`)
**Purpose:** Tools for fixing shows split across multiple libraries

- Detects shows in multiple locations
- Generates consolidation plan (dry-run capable)
- Executes moves with progress tracking
- Updates database and notifies Sonarr/Radarr after consolidation
- CLI: `jellywatch consolidate --dry-run`

### Important Patterns

#### File Transfer Pattern (CRITICAL)
```go
// NEVER do this - hangs on failing disks:
os.Rename(src, dst)
io.Copy(dst, src)

// ALWAYS use transfer package:
transferer := transfer.New(transfer.BackendRsync)
result := transferer.Transfer(src, dst, transfer.Options{
    Timeout: 5 * time.Minute,
    TargetUID: uid,
    TargetGID: gid,
})
```

#### Library Selection Pattern (uses HOLDEN database first)
```go
selector := library.NewSelectorWithConfig(library.SelectorConfig{
    Libraries:    []string{"/media/TV1", "/media/TV2"},
    SonarrClient: sonarrClient,  // optional, fallback
    DB:           db,             // HOLDEN database - checked FIRST
})

result := selector.SelectTVShowLibrary(showName, year, season, fileSize)
// Priority order:
// 1. Database canonical path (<1ms)
// 2. Sonarr cache (~100ms warm, 1-5s refresh)
// 3. Episode count heuristic (last resort)
```

#### Self-Learning Pattern (updates HOLDEN after organization)
```go
// After successful file organization
series := &database.Series{
    Title:          showName,
    Year:           yearInt,
    CanonicalPath:  targetPath,
    LibraryRoot:    libraryPath,
    Source:         "jellywatch",
    SourcePriority: 100,  // Highest priority - overrides all
}

shouldUpdateExternal, err := db.UpsertSeries(series)
if shouldUpdateExternal && sonarrClient != nil {
    // JellyWatch path differs from Sonarr - update Sonarr
    sonarrClient.UpdateSeriesPath(sonarrID, targetPath)
}
```

#### AI Title Matching Pattern
```go
// AI matching is integrated into naming package
// Falls back to regex if AI unavailable
parsed := naming.ParseTVEpisode(filename)
// Uses AI internally if configured and enabled
```

## Jellyfin Naming Requirements

### Movies
Format: `Movies/Movie Name (YYYY)/Movie Name (YYYY).ext`

Example: `Movies/The Matrix (1999)/The Matrix (1999).mkv`

### TV Shows
Format: `TV Shows/Show Name (Year)/Season 01/Show Name (Year) S01E01.ext`

Example: `TV Shows/Silo (2023)/Season 01/Silo (2023) S01E02.mkv`

### Rules
- Year MUST be in parentheses: `(YYYY)`
- Season folders MUST be padded: `Season 01`, `Season 02`
- Episode format: `SXXEYY` with zero-padded numbers
- Remove all release markers: resolution, codec, audio, HDR, streaming sources, release groups
- Remove special characters: `< > : " / \ | ? *`
- Preserve apostrophes in titles but sanitize for filesystem

### HOLDEN Database Management

#### Populating the Database
```bash
# Scan all libraries and populate database
jellywatch scan

# Scan and check for conflicts (shows in multiple locations)
jellywatch scan --check-conflicts

# Scan and sync from Sonarr/Radarr
jellywatch scan --sonarr --radarr
```

#### Checking Status
```bash
# View database status, sync history, conflicts
jellywatch status
```

#### Consolidating Split Shows
```bash
# Preview consolidation plan (no changes)
jellywatch consolidate --dry-run

# Consolidate all split shows (with confirmation)
jellywatch consolidate

# Consolidate specific show
jellywatch consolidate "Silo"

# Consolidate without confirmation
jellywatch consolidate --force
```

#### Source Priority Rules
When conflicts occur between different sources:

| Priority | Source | When Used | Behavior |
|----------|--------|-----------|----------|
| 100 | `jellywatch` | After organizing a file | Always wins, triggers external API update |
| 50 | `filesystem` | During library scan | Updates if no higher priority exists |
| 25 | `sonarr` | During Sonarr sync | Never overwrites jellywatch/filesystem |
| 25 | `radarr` | During Radarr sync | Never overwrites jellywatch/filesystem |

**Key insight:** JellyWatch never asks "where should this go?" after it has learned. It tells Sonarr/Radarr where things ARE.

## Common Development Tasks

### Adding New Release Group Patterns
Edit `internal/naming/init.go` to add patterns to `releaseGroupPatterns` slice.

### Testing Naming Logic
```bash
# Run real-world tests with actual filenames
go test -v -run TestRealWorld ./internal/naming/

# Add test cases to naming_test.go or realworld_test.go
```

### Testing AI Integration
```bash
# Run AI matching experiments
cd experiments/ai-matching
go test -v

# Benchmark AI vs regex
go test -bench=.
```

### Adding New Commands
Commands are defined in `cmd/jellywatch/` using Cobra:
- Create new `new*Cmd()` function
- Add to `rootCmd.AddCommand()` in `main.go`

### Modifying File Organization Logic
Edit `internal/organizer/organizer.go`:
- `OrganizeTVEpisode()` - TV show organization
- `OrganizeMovie()` - movie organization
- `OrganizeFolder()` - folder analysis and organization

## Development Phase Status

The project has completed several development phases:

- **Phase 0:** Core functionality (file watching, naming, organization) ✅
- **Phase 1:** AI title matching prototype (100% accuracy vs 52% regex baseline) ✅
- **Phase 2:** AI integration into naming package with database caching ✅
- **Phase 3:** HOLDEN self-learning database integration ✅
  - Library selector uses database first (before Sonarr API)
  - Organizer updates database after successful operations
  - Database becomes authoritative source of truth
- **Phase 4:** Consolidation tools (mostly complete) ✅
  - Infrastructure complete: plan generation, execution, CLI command
  - All tests passing, deadlock bug fixed
  - **Known limitation:** Conflict detection needs filesystem scanning integration
  - See `docs/consolidation-architecture.md` for details

**Current State:** Core functionality complete and operational:
- AI-powered title matching for edge cases
- Self-learning database (HOLDEN) for instant lookups
- Graceful fallbacks at every layer
- Sonarr/Radarr integration with JellyWatch as authority
- Consolidation infrastructure ready (conflict detection pending)

**Next:** Phase 5 (testing, benchmarking, documentation)

See detailed documentation:
- `HOLDEN.md` - Complete HOLDEN system architecture
- `docs/consolidation-architecture.md` - Conflict detection architecture issue
- `PHASE_*_VERIFICATION.md` - Phase completion reports
- `DEOBFUSCATE.md` - AI title matching details
- `AGENTS.md` - Package-specific development guidance

## Critical Conventions

### Data Preservation
- NEVER delete source files without explicit `delete_source = true` config
- Always preserve higher quality when duplicates found
- Use `--dry-run` for testing

### Quality Hierarchy (highest to lowest)
1. REMUX (direct bluray rip)
2. BluRay / BDRip
3. WEB-DL (streaming download)
4. WEBRip (stream capture)
5. HDTV (broadcast capture)
6. DVDRip

Resolution: `2160p/4K` > `1080p` > `720p`

### Timeout Handling
All file operations MUST have timeout handling because:
- Failing disks can cause indefinite hangs
- Network mounts can become unresponsive
- Use `transfer` package which implements proper timeouts via rsync

### Permission Handling
- Permissions configured via `[permissions]` in config.toml
- Uses rsync's `--chown` and `--chmod` flags for efficient permission changes
- Requires root/CAP_CHOWN for ownership changes
- Non-root processes silently skip ownership changes but still apply mode changes

## Testing Philosophy

- Unit tests for naming logic (most critical component)
- Integration tests for file organization workflow
- Real-world test cases in `naming/realworld_test.go`
- Benchmarks for performance-critical code (naming, AI matching)
- Database tests using temporary SQLite files
