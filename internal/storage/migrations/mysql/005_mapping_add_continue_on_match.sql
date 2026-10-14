-- +goose Up
-- Move continue_on_match from rule-level to mapping-level.
-- Since mappings now own the rule list, the "keep evaluating after first match"
-- decision belongs to the mapping, not individual rules.
ALTER TABLE mappings ADD COLUMN continue_on_match TINYINT(1) NOT NULL DEFAULT 0 AFTER rule_ids;

-- +goose Down
ALTER TABLE mappings DROP COLUMN continue_on_match;
