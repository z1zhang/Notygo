package storage

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

// MigrationsFS holds the embedded MySQL SQL migration files.
//
//go:embed migrations/mysql/*.sql
var MigrationsFS embed.FS

// RunMigrations applies all pending MySQL migrations.
func RunMigrations(db *sql.DB) error {
	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	goose.SetBaseFS(MigrationsFS)

	if err := goose.Up(db, "migrations/mysql"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
