package storage

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

// MigrationsFS holds the embedded PostgreSQL SQL migration files.
//
//go:embed migrations/postgres/*.sql
var MigrationsFS embed.FS

// RunMigrations applies all pending PostgreSQL migrations.
// It takes a *sql.DB opened via OpenMigrationsDB (goose does not accept pgx pools).
func RunMigrations(db *sql.DB) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	goose.SetBaseFS(MigrationsFS)

	if err := goose.Up(db, "migrations/postgres"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
