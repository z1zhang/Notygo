package storage

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

// MigrationsFS holds the embedded SQL migration files.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS

// RunMigrations applies all pending SQL migrations from the embedded FS.
func RunMigrations(db *sql.DB) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	goose.SetBaseFS(MigrationsFS)

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
