-- +goose Up
-- Remove continue_on_match from rules: this field has moved to the mapping level
-- (mappings.continue_on_match). The per-rule field was redundant since a mapping
-- now controls whether to continue evaluating rules after a match.
ALTER TABLE rules DROP COLUMN continue_on_match;

-- +goose Down
ALTER TABLE rules ADD COLUMN continue_on_match TINYINT(1) NOT NULL DEFAULT 0 AFTER condition_mode;
