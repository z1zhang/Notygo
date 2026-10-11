-- +goose Up
ALTER TABLE deliveries ADD COLUMN rendered_format TEXT;

-- +goose Down
-- SQLite does not support DROP COLUMN before version 3.35.0
-- For safety, we leave the column in down migrations.
