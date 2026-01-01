package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Nomadcxx/jellywatch/internal/config"
	"github.com/Nomadcxx/jellywatch/internal/daemon"
	"github.com/Nomadcxx/jellywatch/internal/radarr"
	"github.com/Nomadcxx/jellywatch/internal/sonarr"
	"github.com/Nomadcxx/jellywatch/internal/transfer"
	"github.com/Nomadcxx/jellywatch/internal/watcher"
	"github.com/spf13/cobra"
)

var (
	cfgFile     string
	dryRun      bool
	backendName string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "jellywatchd",
		Short: "JellyWatch daemon service",
		Long: `JellyWatchd runs in the background monitoring directories for new media files.
It automatically organizes them according to Jellyfin naming conventions.`,
		RunE: runDaemon,
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "preview changes without moving files")
	rootCmd.PersistentFlags().StringVar(&backendName, "backend", "auto", "transfer backend: auto, pv, rsync, native")

	rootCmd.AddCommand(newInstallCmd())
	rootCmd.AddCommand(newUninstallCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runDaemon(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("unable to load config: %w", err)
	}

	var watchPaths []string
	watchPaths = append(watchPaths, cfg.Watch.Movies...)
	watchPaths = append(watchPaths, cfg.Watch.TV...)

	if len(watchPaths) == 0 {
		return fmt.Errorf("no watch directories configured")
	}

	var sonarrClient *sonarr.Client
	if cfg.Sonarr.Enabled && cfg.Sonarr.APIKey != "" && cfg.Sonarr.URL != "" {
		sonarrClient = sonarr.NewClient(sonarr.Config{
			URL:     cfg.Sonarr.URL,
			APIKey:  cfg.Sonarr.APIKey,
			Timeout: 30 * time.Second,
		})
		log.Printf("Sonarr integration enabled: %s", cfg.Sonarr.URL)
	}

	var radarrClient *radarr.Client
	if cfg.Radarr.Enabled && cfg.Radarr.APIKey != "" && cfg.Radarr.URL != "" {
		radarrClient = radarr.NewClient(radarr.Config{
			URL:     cfg.Radarr.URL,
			APIKey:  cfg.Radarr.APIKey,
			Timeout: 30 * time.Second,
		})
		log.Printf("Radarr integration enabled: %s", cfg.Radarr.URL)
	}

	handler := daemon.NewMediaHandler(daemon.MediaHandlerConfig{
		TVLibraries:  cfg.Libraries.TV,
		MovieLibs:    cfg.Libraries.Movies,
		DebounceTime: 10 * time.Second,
		DryRun:       dryRun || cfg.Options.DryRun,
		Timeout:      5 * time.Minute,
		SonarrClient: sonarrClient,
		RadarrClient: radarrClient,
		NotifySonarr: cfg.Sonarr.NotifyOnImport,
		NotifyRadarr: cfg.Radarr.NotifyOnImport,
		Backend:      transfer.ParseBackend(backendName),
	})

	w, err := watcher.NewWatcher(handler, dryRun || cfg.Options.DryRun)
	if err != nil {
		return fmt.Errorf("unable to create watcher: %w", err)
	}
	defer w.Close()

	if err := w.Watch(watchPaths); err != nil {
		return fmt.Errorf("unable to watch directories: %w", err)
	}

	log.Printf("JellyWatchd started")
	log.Printf("Watching %d directories", len(watchPaths))
	log.Printf("TV libraries: %v", cfg.Libraries.TV)
	log.Printf("Movie libraries: %v", cfg.Libraries.Movies)
	if dryRun || cfg.Options.DryRun {
		log.Printf("DRY RUN MODE - no files will be moved")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- w.Start()
	}()

	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v, shutting down...", sig)
		handler.Shutdown()
		cancel()
		return nil

	case err := <-errChan:
		return fmt.Errorf("watcher error: %w", err)

	case <-ctx.Done():
		return nil
	}
}

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install jellywatchd as a systemd service",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("To install jellywatchd as a systemd service:")
			fmt.Println()
			fmt.Println("1. Copy the binary:")
			fmt.Println("   sudo cp jellywatchd /usr/local/bin/")
			fmt.Println()
			fmt.Println("2. Copy the service file:")
			fmt.Println("   sudo cp jellywatchd.service /etc/systemd/system/")
			fmt.Println()
			fmt.Println("3. Reload systemd:")
			fmt.Println("   sudo systemctl daemon-reload")
			fmt.Println()
			fmt.Println("4. Enable and start:")
			fmt.Println("   sudo systemctl enable jellywatchd")
			fmt.Println("   sudo systemctl start jellywatchd")
			fmt.Println()
			fmt.Println("5. Check status:")
			fmt.Println("   sudo systemctl status jellywatchd")
			fmt.Println("   journalctl -u jellywatchd -f")
		},
	}
}

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall jellywatchd systemd service",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("To uninstall jellywatchd:")
			fmt.Println()
			fmt.Println("1. Stop and disable:")
			fmt.Println("   sudo systemctl stop jellywatchd")
			fmt.Println("   sudo systemctl disable jellywatchd")
			fmt.Println()
			fmt.Println("2. Remove files:")
			fmt.Println("   sudo rm /etc/systemd/system/jellywatchd.service")
			fmt.Println("   sudo rm /usr/local/bin/jellywatchd")
			fmt.Println()
			fmt.Println("3. Reload systemd:")
			fmt.Println("   sudo systemctl daemon-reload")
		},
	}
}
