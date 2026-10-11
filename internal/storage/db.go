package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Open opens a MySQL database connection using the provided DSN.
func Open(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_DSN is required")
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
