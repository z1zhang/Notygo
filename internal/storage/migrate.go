package storage

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

// MigrationsFS holds the embedded SQL migration files.
//
//go:embed migrations/sqlite/*.sql
var SQLiteMigrationsFS embed.FS

//go:embed migrations/mysql/*.sql
var MySQLMigrationsFS embed.FS

// RunMigrations applies all pending SQL migrations based on the current dialect.
func RunMigrations(db *sql.DB) error {
	var dialect string
	var fs embed.FS
	var dir string

	switch CurrentDialect {
	case DialectMySQL:
		dialect = "mysql"
		fs = MySQLMigrationsFS
		dir = "migrations/mysql"
	default:
		dialect = "sqlite3"
		fs = SQLiteMigrationsFS
		dir = "migrations/sqlite"
	}

	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("set goose dialect %q: %w", dialect, err)
	}

	goose.SetBaseFS(fs)

	if err := goose.Up(db, dir); err != nil {
		return fmt.Errorf("run migrations (%s): %w", dialect, err)
	}

	return nil
}
