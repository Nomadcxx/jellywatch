<div align="center">
  <img src="assets/jellywatch-header.png" alt="JellyWatch" />
  <p><em>Media file watcher and organizer for Jellyfin libraries</em></p>
  <p>Automatically monitors download directories and organizes files according to Jellyfin naming standards</p>
</div>

---

## Quick Install

```bash
curl -sSL https://raw.githubusercontent.com/Nomadcxx/jellywatch/main/install.sh | sudo bash
```

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

### Quick Install (Recommended)

```bash
curl -sSL https://raw.githubusercontent.com/Nomadcxx/jellywatch/main/install.sh | sudo bash
```

Or download and run manually:

```bash
wget https://raw.githubusercontent.com/Nomadcxx/jellywatch/main/install.sh
chmod +x install.sh
sudo ./install.sh
```

The installer will:
- Check for Go installation (1.21+ required)
- Clone the repository
- Build the interactive installer
- Guide you through configuration
- Install binaries to `/usr/local/bin`
- Set up systemd service
- Create config at `~/.config/jellywatch/config.toml`

### Manual Installation

Requirements:
- Go 1.21 or later
- Root privileges (for systemd and `/usr/local/bin`)

```bash
git clone https://github.com/Nomadcxx/jellywatch.git
cd jellywatch
go build -o installer ./cmd/installer
sudo ./installer
```

### Uninstall

```bash
sudo jellywatch-installer
# Select "Uninstall" option
```

Or run the installer again and choose uninstall mode.

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

The installer automatically sets up the systemd service. To manage it:

Check status:

```bash
systemctl status jellywatchd
journalctl -u jellywatchd -f
```

Start/stop/restart:

```bash
sudo systemctl start jellywatchd
sudo systemctl stop jellywatchd
sudo systemctl restart jellywatchd
```

Enable/disable auto-start:

```bash
sudo systemctl enable jellywatchd
sudo systemctl disable jellywatchd
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
