-- +goose Up
-- Move rule↔mapping association from rules.mapping_id to mappings.rule_ids.
-- Uses idempotent checks since this migration may have partially run before.

-- Add rule_ids to mappings if not already present
-- +goose StatementBegin
SET @dbname = DATABASE();
SET @stmt = (SELECT IF(
  (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
   WHERE TABLE_SCHEMA = @dbname AND TABLE_NAME = 'mappings' AND COLUMN_NAME = 'rule_ids') = 0,
  'ALTER TABLE mappings ADD COLUMN rule_ids JSON NULL AFTER revision',
  'SELECT 1'
));
PREPARE s FROM @stmt; EXECUTE s; DEALLOCATE PREPARE s;
-- +goose StatementEnd

-- Drop mapping_id from rules if still present (added by migration 003)
-- +goose StatementBegin
SET @stmt = (SELECT IF(
  (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
   WHERE TABLE_SCHEMA = @dbname AND TABLE_NAME = 'rules' AND COLUMN_NAME = 'mapping_id') > 0,
  'ALTER TABLE rules DROP COLUMN mapping_id',
  'SELECT 1'
));
PREPARE s FROM @stmt; EXECUTE s; DEALLOCATE PREPARE s;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE mappings DROP COLUMN rule_ids;
ALTER TABLE rules ADD COLUMN mapping_id VARCHAR(36) NULL AFTER name;
