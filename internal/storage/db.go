package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"

	"zPush/internal/config"
)

// Open opens a database connection based on the configured driver.
func Open(cfg config.DatabaseConfig) (*sql.DB, error) {
	switch cfg.Driver {
	case "mysql":
		return openMySQL(cfg.DSN)
	case "sqlite":
		return OpenSQLite(cfg.Path)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q (use \"sqlite\" or \"mysql\")", cfg.Driver)
	}
}

// OpenSQLite opens a SQLite database, creates parent directories if needed,
// and sets recommended pragmas for V1 (WAL, busy_timeout, foreign_keys).
func OpenSQLite(dbPath string) (*sql.DB, error) {
	CurrentDialect = DialectSQLite

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

// openMySQL opens a MySQL database connection.
func openMySQL(dsn string) (*sql.DB, error) {
	CurrentDialect = DialectMySQL

	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_DSN is required when DB_DRIVER=mysql")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	// MySQL connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	// Set strict SQL mode
	_, err = db.Exec("SET sql_mode = 'STRICT_TRANS_TABLES'")
	if err != nil {
		return nil, fmt.Errorf("set sql_mode: %w", err)
	}

	return db, nil
}
