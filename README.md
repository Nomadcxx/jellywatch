# JellyWatch

Media file watcher and organizer for Jellyfin libraries. Automatically monitors download directories and organizes files according to Jellyfin naming standards.

## Features

- **File System Monitoring**: Watches download directories for new media files
- **Automatic Organization**: Renames and organizes files according to Jellyfin standards
- **Duplicate Detection**: Identifies duplicate files before moving
- **Compliance Validation**: Ensures all files follow Jellyfin naming conventions
- **Daemon Support**: Runs as a background service with systemd integration
- **Dry Run Mode**: Preview changes before applying them

## Naming Conventions

### Movies
```
Movies/Movie Name (YYYY)/Movie Name (YYYY).ext
```

### TV Shows
```
TV Shows/Show Name (Year)/Season 01/Show Name (Year) S01E01.ext
```

### Rules
- Year must be in parentheses: `(YYYY)`
- Season folders must be padded: `Season 01`, `Season 02`
- No special characters: `< > : " / \ | ? *`
- Release group markers removed: `1080p`, `x264`, `WEB-DL`, etc.
- Episode format: `SXXEYY` with padded numbers

## Installation

```bash
go install ./cmd/jellywatch
```

Or build locally:

```bash
go build -o jellywatch ./cmd/jellywatch
```

## Usage

### Interactive Mode
```bash
jellywatch
```

### Watch Directory
```bash
jellywatch watch /path/to/downloads
```

### Organize Existing Files
```bash
jellywatch organize /path/to/library
```

### Validate Compliance
```bash
jellywatch validate /path/to/library
```

### Dry Run (Preview Changes)
```bash
jellywatch organize --dry-run /path/to/library
```

## Configuration

Configuration file location: `~/.config/jellywatch/config.toml`

```toml
[watch]
movies = ["/path/to/downloads/movies"]
tv = ["/path/to/downloads/tv"]

[libraries]
movies = ["/path/to/jellyfin/Movies"]
tv = ["/path/to/jellyfin/TV Shows"]

[daemon]
enabled = true
scan_frequency = "5m"

[options]
dry_run = false
verify_checksums = false
delete_source = true
```

## Daemon Service

Install as systemd service:

```bash
sudo jellywatch daemon install
sudo systemctl enable jellywatchd
sudo systemctl start jellywatchd
```

Check status:

```bash
systemctl status jellywatchd
journalctl -u jellywatchd -f
```

## Development

```bash
# Run tests
go test ./...

# Run with race detector
go run -race ./cmd/jellywatch

# Build binaries
go build ./cmd/jellywatch
go build ./cmd/jellywatchd
```

## License

MIT
