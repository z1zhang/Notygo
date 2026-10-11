-- +goose Up
-- rendered_format column is already included in 001_init.sql for MySQL, so this is a no-op.
-- This migration exists only to keep the version number in sync with SQLite.

-- +goose Down
-- No-op: column was created in 001_init.sql
