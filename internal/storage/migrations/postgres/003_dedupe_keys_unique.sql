-- Atomic dedupe judgment: a unique index on (rule_id, dedupe_hash) lets
-- CheckAndInsertDedupeKey collapse its two-step SELECT+INSERT into a single
-- INSERT ... ON CONFLICT statement, closing the race where two workers both
-- saw "not found" and both inserted (and both sent the notification).
--
-- Deployed instances may already carry duplicate rows from that race: keep
-- the row with the latest expires_at (the one still able to suppress) and
-- drop the rest before the unique index goes in.
-- +goose Up

DELETE FROM notygo_dedupe_keys a
USING notygo_dedupe_keys b
WHERE a.rule_id = b.rule_id
  AND a.dedupe_hash = b.dedupe_hash
  AND a.id <> b.id
  AND (a.expires_at, a.id) < (b.expires_at, b.id);

-- Replaces idx_dedupe_lookup: the unique index covers the same lookups
-- (rule_id + dedupe_hash prefix; expiry is a filter, not a key). The
-- expires_at index stays for the cleanup job.
DROP INDEX idx_dedupe_lookup;
CREATE UNIQUE INDEX uq_dedupe_keys_rule_hash ON notygo_dedupe_keys(rule_id, dedupe_hash);

-- +goose Down

DROP INDEX uq_dedupe_keys_rule_hash;
CREATE INDEX idx_dedupe_lookup ON notygo_dedupe_keys(rule_id, dedupe_hash, expires_at);
