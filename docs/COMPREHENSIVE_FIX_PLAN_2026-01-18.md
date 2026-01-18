# Comprehensive Fix Plan - JellyWatch Issues
**Date:** 2026-01-18
**Status:** Investigation Complete - Ready for Implementation

---

## Executive Summary

Deep investigation of jellywatch logs and database revealed **5 distinct issues** ranging from critical (blocking file transfers) to low-priority (cosmetic). All root causes identified, solutions designed, and prioritized by impact.

**Quick Stats:**
- 🔴 **1 Critical Issue** - Blocks file organization
- 🟠 **2 High Priority Issues** - Causes errors and log spam
- 🟡 **1 Medium Priority Issue** - Boot timing issue
- 🟢 **1 Low Priority Issue** - False positive duplicates

---

## Issue #1: Permission Denied on Disk Health Checks 🔴 CRITICAL

### Impact
**BLOCKING:** Files cannot be transferred to 9 library directories across multiple storage devices.

**Affected Systems:**
- 2,145 directories with incorrect permissions (755 instead of 775)
- Prevents jellywatchd from writing health check files before transfers
- Affects Pocoyo episodes, Tell Me Lies S03, and many others

### Root Cause
Sonarr/Radarr create directories with mode 755 (rwxr-xr-x) instead of 775 (rwxrwxr-x).

**Why this fails:**
```
drwxr-xr-x  sonarr:media  # Owner can write, group CANNOT write
jellywatchd runs as:      nomadx (member of media group)
Result:                   Permission denied when creating .jellywatch_health_check_* files
```

### Evidence
```log
[ERROR] Organization failed | error=unable to organize:
  write test failed: open /mnt/STORAGE10/TVSHOWS/Pocoyo (2005)/Season 04/.jellywatch_health_check_*:
  permission denied
```

### Solution

**Option A: Mass Permission Fix (Immediate)**
```bash
# Fix all existing directories
sudo /tmp/fix_permissions.sh
```

**Option B: Prevent Future Issues (Long-term)**
1. Configure Sonarr/Radarr to create directories with 775 permissions
2. Add umask setting to service files
3. OR: Change jellywatchd to run as sonarr user (not recommended - breaks principle of least privilege)

**Recommended:** Use both Option A (immediate fix) + Option B (prevent recurrence)

### Implementation Steps
1. ✅ **Done:** Created `/tmp/fix_permissions.sh` - fixes 2,145 directories
2. **Todo:** Run the script: `sudo /tmp/fix_permissions.sh`
3. **Todo:** Configure Sonarr/Radarr umask or directory permissions
4. **Todo:** Test file transfer to previously failing libraries
5. **Todo:** Monitor logs for permission errors

### Validation
```bash
# Before: 755 permissions
stat -c "%a" /mnt/STORAGE10/TVSHOWS/Pocoyo\ \(2005\)/Season\ 04
# Expected output: 755

# After: 775 permissions
sudo /tmp/fix_permissions.sh
stat -c "%a" /mnt/STORAGE10/TVSHOWS/Pocoyo\ \(2005\)/Season\ 04
# Expected output: 775

# Test transfer
./jellywatch organize /path/to/test/file --tv-library /mnt/STORAGE10/TVSHOWS
```

### Risk Assessment
- **Risk:** Low - chmod 775 is safe for media directories
- **Rollback:** Re-run script with 755 if issues arise
- **Blast Radius:** All media libraries (intentional - needed for fix)

---

## Issue #2: SABnzbd Extraction Race Condition 🟠 HIGH PRIORITY

### Impact
**HIGH FREQUENCY ERRORS:** ~40+ failed organization attempts per session, log spam, wasted CPU cycles.

### Root Cause
JellyWatch processes files while SABnzbd is still extracting them in temporary `_UNPACK_*` folders.

**Event Timeline:**
```
1. SABnzbd starts extracting → creates _UNPACK_* folder
2. fsnotify CREATE event → JellyWatch detects file
3. JellyWatch waits 10s debounce
4. SABnzbd finishes → renames folder (removes _UNPACK_), moves files
5. JellyWatch tries to process → file not found at original path
```

### Evidence
```log
[INFO] Processing file | filename=FecTgShVV50uu0MXsRLbG4CB7Lf1LBXZ.mkv
[INFO] Detected obfuscated filename, using folder name
[ERROR] Organization failed | error=unable to get file size:
  stat /mnt/NVME3/Sabnzbd/complete/tv/_UNPACK_Pocoyo.S04E23.*/FecTgShVV50uu0MXsRLbG4CB7Lf1LBXZ.mkv:
  no such file or directory
```

**Pattern:** ALL errors contain either `_UNPACK_`, `_FAILED_`, or `_INCOMPLETE_` prefixes.

### Solution: Combined Approach (4-Layer Defense)

**Layer 1: Skip Temp Directories in Watcher**
```go
// internal/watcher/watcher.go:addRecursive()
basename := filepath.Base(path)

// Skip SABnzbd temporary directories
if strings.HasPrefix(basename, "_UNPACK_") ||
   strings.HasPrefix(basename, "_FAILED_") ||
   strings.HasPrefix(basename, "_INCOMPLETE_") {
    return filepath.SkipDir
}
```

**Layer 2: File-Exists Check Before Processing**
```go
// internal/daemon/handler.go:processFile()
func (h *MediaHandler) processFile(path string) {
    // ... existing code ...

    // Graceful degradation if file moved during debounce
    if _, err := os.Stat(path); os.IsNotExist(err) {
        h.logger.Debug("handler", "File no longer exists (likely moved by download client)",
            logging.F("path", path))
        return
    }

    filename := filepath.Base(path)
    // ... continue processing ...
}
```

**Layer 3: Keep Existing Debounce (10s)**
- Sufficient once temp directories are ignored
- No need to increase to 30s+ and delay processing

**Layer 4: Logging Improvements**
- Change "Organization failed" to DEBUG for "file not found" errors
- Add INFO log when skipping temp directories

### Implementation Steps
1. **Todo:** Modify `internal/watcher/watcher.go` - add temp directory filtering
2. **Todo:** Modify `internal/daemon/handler.go` - add file-exists check
3. **Todo:** Add tests for temp directory exclusion
4. **Todo:** Rebuild and deploy: `go build -o jellywatchd ./cmd/jellywatchd`
5. **Todo:** Restart daemon: `sudo systemctl restart jellywatchd`
6. **Todo:** Monitor logs - should see ~40 fewer errors per session

### Validation
```bash
# Create test _UNPACK_ directory
mkdir -p /tmp/test_sabnzbd/_UNPACK_Test.Show.S01E01/
touch /tmp/test_sabnzbd/_UNPACK_Test.Show.S01E01/file.mkv

# Add to watcher, verify it's NOT added
./jellywatchd --config test_config.toml

# Check logs - should NOT see "Watching: /tmp/test_sabnzbd/_UNPACK_*"
```

### Full Report
See: `JELLYWATCH_BUG_REPORT_SABnzbd_Race_Condition.md`

---

## Issue #3: Database Locked Error (SQLITE_BUSY) 🟠 HIGH PRIORITY

### Impact
**AI DISABLED:** When database is locked at startup, AI-powered title matching is unavailable for that session.

### Root Cause
**Multiple concurrent processes** accessing database + **non-WAL journal mode**.

**Why it happens:**
1. Database opened with `?_journal_mode=WAL` connection parameter
2. **BUT:** modernc.org/sqlite doesn't support connection string parameters for journal mode
3. Database remains in "delete" mode (default)
4. When installer/scan runs while daemon starts → SQLITE_BUSY

### Evidence
```log
2026-01-18T04:51:35 [WARN] Failed to open database, AI disabled |
  error=database is locked (5) (SQLITE_BUSY)
```

```bash
$ sqlite3 ~/.config/jellywatch/media.db "PRAGMA journal_mode;"
delete  # ← Should be "wal"
```

**Current Code (doesn't work):**
```go
// internal/database/database.go:39
db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
```

### Solution: Set WAL Mode with PRAGMA Statement

**Option A: Set on First Connection (Recommended)**
```go
// internal/database/database.go:OpenPath()
db, err := sql.Open("sqlite", path+"?_busy_timeout=5000")
if err != nil {
    return nil, fmt.Errorf("failed to open database: %w", err)
}

// Enable WAL mode explicitly
if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
    db.Close()
    return nil, fmt.Errorf("failed to set WAL mode: %w", err)
}

// Also enable other optimizations
if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
    db.Close()
    return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
}
```

**Option B: Migration-Based (Alternative)**
Add WAL mode setting to migration system.

### Benefits of WAL Mode
- **Multiple readers** can read while one writer writes
- **Better concurrency** - eliminates most SQLITE_BUSY errors
- **Faster writes** - better performance for AI cache
- **Industry standard** - SQLite recommendation for concurrent access

### Implementation Steps
1. **Todo:** Modify `internal/database/database.go:OpenPath()`
2. **Todo:** Add PRAGMA statements after `sql.Open()`
3. **Todo:** Add test to verify WAL mode is set
4. **Todo:** Rebuild CLI and daemon
5. **Todo:** Delete existing database to force recreation with WAL:
   ```bash
   sudo systemctl stop jellywatchd
   mv ~/.config/jellywatch/media.db ~/.config/jellywatch/media.db.backup
   sudo systemctl start jellywatchd
   ./jellywatch scan  # Repopulate
   ```
6. **Todo:** Verify: `sqlite3 ~/.config/jellywatch/media.db "PRAGMA journal_mode;"`
   - Expected output: `wal`

### Validation
```bash
# Check journal mode after fix
sqlite3 ~/.config/jellywatch/media.db "PRAGMA journal_mode;"
# Expected: wal

# Concurrent access test
./jellywatch scan &          # Process 1 (background)
./jellywatch duplicates      # Process 2 (should not error)

# Check for SQLITE_BUSY errors
journalctl -u jellywatchd --since "5 minutes ago" | grep SQLITE_BUSY
# Expected: no output
```

### Alternative: Existing Database Migration
If you want to convert existing database without recreating:
```bash
sqlite3 ~/.config/jellywatch/media.db "PRAGMA journal_mode=WAL;"
```

**Note:** This only affects that database file. Code change ensures ALL future databases use WAL.

---

## Issue #4: Sonarr Connection Refused 🟡 MEDIUM PRIORITY

### Impact
**BOOT TIMING ISSUE:** Jellywatchd degrades to heuristic-based library selection during startup.

### Root Cause
**Service startup order** - jellywatchd starts before Sonarr during system boot.

**Timeline:**
```
21:55:42 - jellywatchd starts
21:55:42 - Checks Sonarr connection → connection refused
21:55:42 - Continues without Sonarr (graceful degradation)
21:57:38 - Sonarr starts (2 minutes later)
```

### Evidence
```log
2026-01-18T21:55:42 [WARN] Sonarr connection failed, will continue without intelligent library selection |
  error=dial tcp [::1]:8989: connect: connection refused
```

```bash
$ systemctl status sonarr
Active: active (running) since Sun 2026-01-18 21:57:38  # Started AFTER jellywatchd
```

**Current state:** Sonarr IS running and accessible:
```bash
$ curl -H "X-Api-Key: 811882e144c34c948de1a996fc9563a0" http://localhost:8989/api/v3/system/status
{"appName":"Sonarr","version":"4.0.16.2944", ...}  # ✓ Works now
```

### Solution

**Option A: Systemd Service Dependencies (Recommended)**
```ini
# /etc/systemd/system/jellywatchd.service
[Unit]
After=network.target sonarr.service radarr.service
Wants=sonarr.service radarr.service
```

**Option B: Retry Logic in Daemon**
```go
// Retry Sonarr connection with exponential backoff
for i := 0; i < 3; i++ {
    if err := sonarrClient.Ping(); err == nil {
        break
    }
    time.Sleep(time.Duration(1<<i) * time.Second) // 1s, 2s, 4s
}
```

**Option C: Lazy Initialization**
Don't ping Sonarr at startup - initialize on first use.

**Recommended:** Option A (service dependencies) + Option B (retry logic for robustness)

### Implementation Steps
1. **Todo:** Add service dependencies to systemd unit file
2. **Todo:** Add retry logic to daemon Sonarr initialization
3. **Todo:** Reload systemd: `sudo systemctl daemon-reload`
4. **Todo:** Test: `sudo systemctl restart jellywatchd`
5. **Todo:** Verify Sonarr connection in logs (no "connection refused")

### Validation
```bash
# Restart services in correct order
sudo systemctl restart jellywatchd sonarr

# Check startup order
journalctl -u jellywatchd --since "1 minute ago" | grep -i sonarr
# Should see: "Sonarr integration enabled" (no errors)
```

### Current Workaround
System self-heals after boot - Sonarr integration works normally once both services are running. **Impact is minimal** - only affects first few file operations after boot before HOLDEN database is populated.

---

## Issue #5: Multi-Part DVD Rip False Positives 🟢 LOW PRIORITY

### Impact
**COSMETIC:** Old DVD rips (cd1/cd2) incorrectly flagged as duplicates in reports.

### Root Cause
Duplicate detection doesn't distinguish between:
- **Multi-part files:** Same movie split into parts (cd1/cd2) in SAME directory
- **True duplicates:** Same movie in DIFFERENT directories (different quality/location)

### Evidence
```sql
SELECT path FROM media_files WHERE normalized_title='iptr';
-- /mnt/STORAGE8/MOVIES/The recruit (2003)/ip-tr.cd1.avi
-- /mnt/STORAGE8/MOVIES/The recruit (2003)/ip-tr.cd2.avi
-- ^ Same directory, but flagged as duplicate
```

**Analysis of current duplicates:**
| Title | Count | Same Dir? | Multi-part? | Verdict |
|-------|-------|-----------|-------------|---------|
| conanthebarbarian | 2 | No | No | ✅ TRUE DUPLICATE |
| fackhamhall | 2 | No | No | ✅ TRUE DUPLICATE |
| ifihadlegsidkickyou | 2 | No | No | ✅ TRUE DUPLICATE (naming issue) |
| **iptr** | 2 | **Yes** | **Yes (cd1/cd2)** | ❌ FALSE POSITIVE |
| lethalweapon2 | 2 | Yes | No | ✅ TRUE DUPLICATE (diff quality) |
| shelbyoaks | 2 | No | No | ✅ TRUE DUPLICATE |
| fallout S02E04 | 2 | No | No | ✅ TRUE DUPLICATE |

**Pattern:** Only 1 out of 7 duplicate groups is a false positive (14%).

### Solution: Multi-Part Detection in Duplicate Query

**Add detection for:**
- `cd1`, `cd2`, `cd3` (DVD rips)
- `disc1`, `disc2` (multi-disc releases)
- `part1`, `part2` (split movies)
- `pt1`, `pt2` (abbreviated)

**Approach 1: Filename Pattern Matching**
```go
// internal/database/queries.go
func isMultiPartFile(path string) bool {
    filename := strings.ToLower(filepath.Base(path))

    // Match cd1/cd2/cd3 patterns
    if regexp.MustCompile(`\b(cd|disc|part|pt)[0-9]\b`).MatchString(filename) {
        return true
    }

    return false
}

func (m *MediaDB) FindDuplicateMovies() ([]DuplicateGroup, error) {
    // ... existing query ...

    // Filter out multi-part files
    var realDuplicates []DuplicateGroup
    for _, group := range groups {
        // If all files in same directory AND have multi-part markers → skip
        if allInSameDir(group.Files) && allHaveMultiPartMarkers(group.Files) {
            continue  // Not a real duplicate
        }
        realDuplicates = append(realDuplicates, group)
    }

    return realDuplicates, nil
}
```

**Approach 2: Directory-Based Filtering**
Simpler - just check if all files in same parent directory:
```go
// If all files in same directory → likely multi-part, not duplicate
if allInSameDirectory(group.Files) {
    // Optional: check for cd1/cd2 pattern to confirm
    continue  // Skip from duplicate report
}
```

**Recommended:** Approach 1 (pattern matching) - more precise, fewer false negatives.

### Implementation Steps
1. **Todo:** Add `isMultiPartFile()` helper function
2. **Todo:** Modify `FindDuplicateMovies()` to filter multi-part groups
3. **Todo:** Modify `FindDuplicateEpisodes()` for consistency
4. **Todo:** Add test cases with cd1/cd2 files
5. **Todo:** Rebuild: `go build -o jellywatch ./cmd/jellywatch`
6. **Todo:** Test: `./jellywatch duplicates`
7. **Todo:** Verify: iptr should NOT appear in output

### Validation
```bash
# Before fix
./jellywatch duplicates | grep -c "iptr"
# Expected: 1 (appears in output)

# After fix
./jellywatch duplicates | grep -c "iptr"
# Expected: 0 (filtered out)

# Ensure real duplicates still detected
./jellywatch duplicates | grep -c "conanthebarbarian"
# Expected: 1 (still appears)
```

### Alternative: Document as Known Limitation
Since impact is low (14% false positive rate, cosmetic only), could document this as known limitation and defer fix.

---

## Implementation Priority Matrix

| Priority | Issue | Impact | Effort | ROI |
|----------|-------|--------|--------|-----|
| **1** | Permission Denied | Critical - Blocks transfers | 5 min | ⭐⭐⭐⭐⭐ |
| **2** | SABnzbd Race Condition | High - 40+ errors/session | 30 min | ⭐⭐⭐⭐ |
| **3** | Database Locked | High - Disables AI | 15 min | ⭐⭐⭐⭐ |
| **4** | Sonarr Boot Timing | Medium - Minor degradation | 20 min | ⭐⭐⭐ |
| **5** | Multi-Part False Positives | Low - Cosmetic | 45 min | ⭐⭐ |

---

## Recommended Implementation Order

### Phase 1: Critical Fixes (Today)
1. Run permission fix script (5 min)
2. Implement WAL mode for database (15 min)
3. Implement SABnzbd race condition fix (30 min)

**Total time:** ~50 minutes
**Impact:** Unblocks all file transfers, eliminates 40+ errors/session, enables concurrent DB access

### Phase 2: Medium Priority (This Week)
4. Add Sonarr service dependencies + retry logic (20 min)

**Total time:** 20 minutes
**Impact:** Cleaner boots, better Sonarr integration

### Phase 3: Low Priority (When Time Permits)
5. Add multi-part DVD detection (45 min)

**Total time:** 45 minutes
**Impact:** Cleaner duplicate reports

---

## Testing Checklist

After implementing fixes:

### Smoke Tests
- [ ] Run permission fix script
- [ ] Verify permissions changed: `stat /mnt/STORAGE10/TVSHOWS/Pocoyo\ \(2005\)/Season\ 04`
- [ ] Check WAL mode enabled: `sqlite3 ~/.config/jellywatch/media.db "PRAGMA journal_mode;"`
- [ ] Restart jellywatchd: `sudo systemctl restart jellywatchd`
- [ ] Check logs for errors: `journalctl -u jellywatchd -f`

### Integration Tests
- [ ] Organize test file to previously failing library
- [ ] Run concurrent database operations (scan + duplicates)
- [ ] Verify no SQLITE_BUSY errors
- [ ] Check Sonarr connection in logs
- [ ] Verify no _UNPACK_ errors in logs

### Regression Tests
- [ ] Run full test suite: `go test ./...`
- [ ] Test duplicate detection: `./jellywatch duplicates`
- [ ] Verify real duplicates still detected
- [ ] Scan libraries: `./jellywatch scan`

---

## Rollback Plans

### Issue #1 (Permissions)
```bash
# Revert to 755 if needed
find /mnt/STORAGE*/TVSHOWS /mnt/STORAGE*/MOVIES -type d -perm 775 -exec chmod 755 {} \;
```

### Issue #2 (SABnzbd)
```bash
# Revert code changes
git revert <commit-hash>
go build -o jellywatchd ./cmd/jellywatchd
sudo systemctl restart jellywatchd
```

### Issue #3 (Database)
```bash
# Restore old database
mv ~/.config/jellywatch/media.db.backup ~/.config/jellywatch/media.db
```

---

## Success Metrics

### After Phase 1 Implementation:
- ✅ Zero "permission denied" errors in logs
- ✅ Zero "_UNPACK_" errors in logs
- ✅ Database journal_mode = "wal"
- ✅ AI enabled at startup (no SQLITE_BUSY)
- ✅ Files successfully transferred to all 9 libraries

### After Phase 2 Implementation:
- ✅ Sonarr connected at daemon startup
- ✅ No "connection refused" errors in boot logs

### After Phase 3 Implementation:
- ✅ Multi-part DVD rips excluded from duplicate reports
- ✅ False positive rate < 5%

---

## Additional Observations

### System Health
- **Daemon uptime:** 2h 58min (stable)
- **Memory usage:** 20.2M (good)
- **CPU usage:** 20ms (excellent)
- **Database size:** 10MB (reasonable)
- **Sonarr version:** 4.0.16.2944 (up to date)

### Architectural Strengths Found
1. **Graceful degradation** - AI disabled when DB locked, continues working
2. **Health checks** - Disk health verification before transfers (when permissions allow)
3. **Structured logging** - Easy to diagnose issues
4. **Debounce logic** - Prevents duplicate processing
5. **Self-learning HOLDEN database** - Reduces API dependencies

### Technical Debt Identified
1. modernc.org/sqlite connection string parameters don't work as expected
2. No systemd service ordering for external dependencies
3. Multi-part file detection not implemented
4. Permission management relies on external tools (Sonarr/Radarr) configuration

---

## Documentation Updates Needed

After implementing fixes, update:
1. `CLAUDE.md` - Add WAL mode requirement
2. `README.md` - Add systemd service dependency example
3. `docs/troubleshooting.md` - Add permission fix instructions
4. Migration guide - Document WAL mode conversion for existing databases

---

## Questions for User

1. **Permission fix:** Should I run `/tmp/fix_permissions.sh` now, or do you want to review it first?

2. **Database recreation:** Okay to delete and repopulate database to enable WAL mode? (Scan takes ~5-10 min)

3. **Multi-part DVD detection:** Worth implementing, or acceptable to document as known limitation?

4. **Implementation approach:** Should I implement fixes one-by-one with review, or batch Phase 1 together?

---

**End of Report**
