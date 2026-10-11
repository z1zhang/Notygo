package storage

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// OpenTestDB opens a MySQL database for testing using the TEST_DATABASE_DSN
// environment variable. If the variable is not set, the test is skipped.
func OpenTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set, skipping database test")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	if err := db.Ping(); err != nil {
		t.Fatalf("ping test db: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}
