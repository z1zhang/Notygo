package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// Open opens a native pgx connection pool for the application.
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if dsn == "" {
		return nil, fmt.Errorf("database DSN is empty (check DB_USER / DB_PASSWORD)")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}

	// Connection pool settings
	// MaxConns=50: with 4 event processors + 8 delivery workers, each
	// requiring 7-9 DB queries per event/delivery, 25 connections caused pool
	// exhaustion under concurrent load (queries silently failed → events
	// processed with 0 matched rules).
	cfg.MaxConns = 50
	cfg.MinConns = 4
	cfg.MaxConnLifetime = 5 * time.Minute
	cfg.MaxConnIdleTime = 2 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}

// OpenMigrationsDB opens a database/sql connection used only for running
// goose migrations, which only accepts *sql.DB. It is derived from the same
// DSN as the application pool; business code must use the pgx pool.
func OpenMigrationsDB(dsn string) (*sql.DB, error) {
	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	return stdlib.OpenDB(*connConfig), nil
}
