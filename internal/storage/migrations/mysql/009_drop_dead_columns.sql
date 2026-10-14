-- +goose Up
ALTER TABLE events DROP COLUMN expire_at;
ALTER TABLE sources DROP COLUMN expected_response;
ALTER TABLE destinations DROP COLUMN rate_limit;
ALTER TABLE destinations DROP COLUMN circuit_breaker;
ALTER TABLE rules DROP COLUMN condition_mode;

-- +goose Down
ALTER TABLE events ADD COLUMN expire_at DATETIME DEFAULT NULL;
ALTER TABLE sources ADD COLUMN expected_response TEXT;
ALTER TABLE destinations ADD COLUMN rate_limit TEXT;
ALTER TABLE destinations ADD COLUMN circuit_breaker TEXT;
ALTER TABLE rules ADD COLUMN condition_mode VARCHAR(20) NOT NULL DEFAULT 'and';
