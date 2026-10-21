-- +goose Up
-- Per-destination retry budget. 0/missing rows fall back to 5
-- at delivery-creation time.
ALTER TABLE destinations ADD COLUMN max_attempts INT NOT NULL DEFAULT 5;

-- +goose Down
ALTER TABLE destinations DROP COLUMN max_attempts;
