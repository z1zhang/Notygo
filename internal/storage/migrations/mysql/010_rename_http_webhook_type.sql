-- +goose Up
-- Rename the stored destination type 'http_webhook' to 'http' (matches the
-- sender registry key and the frontend channel id). Idempotent: only rows
-- still carrying the old value are touched.
UPDATE destinations SET type = 'http' WHERE type = 'http_webhook';

-- +goose Down
UPDATE destinations SET type = 'http_webhook' WHERE type = 'http';
