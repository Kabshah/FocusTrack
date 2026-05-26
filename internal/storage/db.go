package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens a SQLite database at the given path, enables WAL mode,
// and configures connection settings for optimal performance.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// SQLite only supports one writer at a time — enforce single connection
	db.SetMaxOpenConns(1)

	// WAL mode and synchronous NORMAL are for persistent files. 
	// Do not use them for in-memory databases.
	if path != ":memory:" {
		if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
			db.Close()
			return nil, fmt.Errorf("setting WAL mode: %w", err)
		}
		if _, err := db.Exec(`PRAGMA synchronous=NORMAL`); err != nil {
			db.Close()
			return nil, fmt.Errorf("setting synchronous mode: %w", err)
		}
	}

	return db, nil
}

// DefaultPath returns the recommended path for the FocusTrack database.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		exe, _ := os.Executable()
		dir = filepath.Dir(exe)
	}
	appDir := filepath.Join(dir, "FocusTrack")
	_ = os.MkdirAll(appDir, 0755)
	return filepath.Join(appDir, "FocusTrack.db")
}


// Migrate creates tables and indexes if they don't exist.
func Migrate(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			app_name     TEXT NOT NULL,
			process_name TEXT NOT NULL,
			window_title TEXT NOT NULL,
			started_at   INTEGER NOT NULL,
			ended_at     INTEGER,
			duration_sec INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_app ON sessions(app_name)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_date ON sessions(started_at)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("executing migration statement: %w", err)
		}
	}
	return nil
}

