package storage

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenTestDB opens a PostgreSQL connection pool for testing using the
// TEST_DATABASE_DSN environment variable. If the variable is not set,
// the test is skipped.
func OpenTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set, skipping database test")
	}

	pool, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	t.Cleanup(func() { pool.Close() })
	return pool
}
