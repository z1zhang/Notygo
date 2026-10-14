package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Open opens a MySQL database connection using the provided DSN.
// Automatically appends multiStatements=true for goose migration support.
func Open(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_DSN is required")
	}

	// Ensure multiStatements is enabled so goose can run multi-statement migrations
	if !strings.Contains(dsn, "multiStatements") {
		if strings.Contains(dsn, "?") {
			dsn += "&multiStatements=true"
		} else {
			dsn += "?multiStatements=true"
		}
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	// MySQL connection pool settings
	// MaxOpenConns=50: with 4 event processors + 8 delivery workers, each
	// requiring 7-9 DB queries per event/delivery, 25 connections caused pool
	// exhaustion under concurrent load (queries silently failed → events
	// processed with 0 matched rules).
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(20)
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
