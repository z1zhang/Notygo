-- +goose Up
-- rendered_format column is already included in 001_init.sql for MySQL, so this is a no-op.
-- This migration is a no-op: the rendered_format column was already created in 001_init.sql.

-- +goose Down
-- No-op: column was created in 001_init.sql
