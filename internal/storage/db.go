package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// OpenSQLite opens a SQLite database, creates parent directories if needed,
// and sets recommended pragmas for V1 (WAL, busy_timeout, foreign_keys).
func OpenSQLite(dbPath string) (*sql.DB, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create database directory %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}

	// Set connection pool settings appropriate for SQLite
	// SQLite only supports one writer at a time; keep max open connections low.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Apply pragmas
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA synchronous=NORMAL;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("exec pragma %q: %w", p, err)
		}
	}

	return db, nil
}
