package organizer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nomadcxx/jellywatch/internal/jellyfin"
)

func TestCheckPlaybackSafetyDisabled(t *testing.T) {
	org, err := NewOrganizer([]string{"/library"})
	if err != nil {
		t.Fatalf("NewOrganizer() error = %v", err)
	}

	if err := org.checkPlaybackSafety("/media/file.mkv"); err != nil {
		t.Fatalf("expected no error when playback safety disabled, got %v", err)
	}
}

func TestCheckPlaybackSafetyLocked(t *testing.T) {
	locks := jellyfin.NewPlaybackLockManager()
	sourcePath := "/media/Movies/The Matrix (1999)/The Matrix (1999).mkv"
	locks.Lock(sourcePath, jellyfin.PlaybackInfo{UserName: "alice", DeviceName: "TV"})

	org, err := NewOrganizer([]string{"/library"}, WithPlaybackLockManager(locks))
	if err != nil {
		t.Fatalf("NewOrganizer() error = %v", err)
	}

	err = org.checkPlaybackSafety(sourcePath)
	if err == nil {
		t.Fatalf("expected error for locked path")
	}
	if !strings.Contains(err.Error(), "being streamed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckPlaybackSafetyUnlocked(t *testing.T) {
	locks := jellyfin.NewPlaybackLockManager()
	sourcePath := "/media/Movies/Inception (2010)/Inception (2010).mkv"

	org, err := NewOrganizer([]string{"/library"}, WithPlaybackLockManager(locks))
	if err != nil {
		t.Fatalf("NewOrganizer() error = %v", err)
	}

	if err := org.checkPlaybackSafety(sourcePath); err != nil {
		t.Fatalf("expected no error for unlocked path, got %v", err)
	}
}

func TestOrganizeMovieLockedQueuesDeferredOperation(t *testing.T) {
	sourceDir := t.TempDir()
	libraryDir := t.TempDir()

	sourcePath := filepath.Join(sourceDir, "The.Matrix.1999.1080p.mkv")
	if err := os.WriteFile(sourcePath, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	locks := jellyfin.NewPlaybackLockManager()
	queue := jellyfin.NewDeferredQueue()
	locks.Lock(sourcePath, jellyfin.PlaybackInfo{UserName: "alice", DeviceName: "TV"})

	org, err := NewOrganizer(
		[]string{libraryDir},
		WithPlaybackLockManager(locks),
		WithDeferredQueue(queue),
	)
	if err != nil {
		t.Fatalf("NewOrganizer() error = %v", err)
	}

	result, err := org.OrganizeMovie(sourcePath, libraryDir)
	if err != nil {
		t.Fatalf("OrganizeMovie() error = %v", err)
	}
	if result == nil || !result.Skipped {
		t.Fatalf("expected organize result to be skipped when locked")
	}
	if queue.Count() != 1 {
		t.Fatalf("expected deferred queue count 1, got %d", queue.Count())
	}
	ops := queue.GetForPath(sourcePath)
	if len(ops) != 1 {
		t.Fatalf("expected 1 deferred operation for source path, got %d", len(ops))
	}
	if ops[0].Type != "organize_movie" {
		t.Fatalf("expected deferred type organize_movie, got %s", ops[0].Type)
	}
}

func TestOrganizeMovieTargetPathLockedQueuesDeferredOperation(t *testing.T) {
	sourceDir := t.TempDir()
	libraryDir := t.TempDir()

	sourcePath := filepath.Join(sourceDir, "Inception.2010.1080p.mkv")
	if err := os.WriteFile(sourcePath, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	lockedTargetPath := filepath.Join(libraryDir, "Inception (2010)", "Inception (2010).mkv")
	locks := jellyfin.NewPlaybackLockManager()
	queue := jellyfin.NewDeferredQueue()
	locks.Lock(lockedTargetPath, jellyfin.PlaybackInfo{UserName: "bob", DeviceName: "Tablet"})

	org, err := NewOrganizer(
		[]string{libraryDir},
		WithPlaybackLockManager(locks),
		WithDeferredQueue(queue),
	)
	if err != nil {
		t.Fatalf("NewOrganizer() error = %v", err)
	}

	result, err := org.OrganizeMovie(sourcePath, libraryDir)
	if err != nil {
		t.Fatalf("OrganizeMovie() error = %v", err)
	}
	if result == nil || !result.Skipped {
		t.Fatalf("expected organize result to be skipped when target path locked")
	}
	if queue.Count() != 1 {
		t.Fatalf("expected deferred queue count 1, got %d", queue.Count())
	}
	ops := queue.GetForPath(lockedTargetPath)
	if len(ops) != 1 {
		t.Fatalf("expected deferred op keyed by locked target path, got %d", len(ops))
	}
}
