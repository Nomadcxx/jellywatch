package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// MediaDB is the main database handle for JellyWatch media tracking
type MediaDB struct {
	db   *sql.DB
	path string
	mu   sync.RWMutex
}

// Open opens or creates the database at the default location
func Open() (*MediaDB, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config dir: %w", err)
	}

	dbPath := filepath.Join(configDir, "jellywatch", "media.db")
	return OpenPath(dbPath)
}

// OpenPath opens or creates the database at a specific path
func OpenPath(path string) (*MediaDB, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open with WAL mode for better concurrent access
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	mdb := &MediaDB{
		db:   db,
		path: path,
	}

	// Apply migrations
	if err := mdb.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return mdb, nil
}

// OpenInMemory opens an in-memory database for testing
func OpenInMemory() (*MediaDB, error) {
	// Open in-memory database with shared cache enabled
	db, err := sql.Open("sqlite", ":memory:?_cache=shared")
	if err != nil {
		return nil, fmt.Errorf("failed to open in-memory database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping in-memory database: %w", err)
	}

	mdb := &MediaDB{
		db:   db,
		path: ":memory:",
	}

	// Apply migrations
	if err := mdb.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate in-memory database: %w", err)
	}

	return mdb, nil
}

// Close closes the database connection
func (m *MediaDB) Close() error {
	return m.db.Close()
}

// Path returns the filesystem path to the database file
func (m *MediaDB) Path() string {
	return m.path
}

// migrate applies any pending schema migrations
func (m *MediaDB) migrate() error {
	return applyMigrations(m.db)
}

// DB returns the underlying sql.DB for advanced operations
func (m *MediaDB) DB() *sql.DB {
	return m.db
}
