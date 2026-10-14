-- +goose Up
-- Dedupe keys table for rule-level deduplication.
-- Stores a SHA-256 hash of the dedupe key per rule with an expiration time,
-- so we can check whether a rule has already produced a delivery for the same
-- key_fields values within a configured time window.
CREATE TABLE dedupe_keys (
    id          VARCHAR(36) PRIMARY KEY,
    rule_id     VARCHAR(36) NOT NULL,
    dedupe_hash VARCHAR(64) NOT NULL,
    expires_at  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_dedupe_lookup (rule_id, dedupe_hash, expires_at),
    INDEX idx_dedupe_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS dedupe_keys;
