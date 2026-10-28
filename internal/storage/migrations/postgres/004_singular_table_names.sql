-- Singular table names, matching the Go layout (one package / one table per
-- entity, singular — the same convention the rule/template packages follow).
-- Append-only by design: 001 created the plural names, so deployed databases
-- rename here, and fresh installs run 001..003 first and land on the same
-- singular schema. Index and constraint names follow their tables.
-- +goose Up

ALTER TABLE notygo_endpoints         RENAME TO notygo_endpoint;
ALTER TABLE notygo_mappings          RENAME TO notygo_mapping;
ALTER TABLE notygo_rules             RENAME TO notygo_rule;
ALTER TABLE notygo_templates         RENAME TO notygo_template;
ALTER TABLE notygo_channels          RENAME TO notygo_channel;
ALTER TABLE notygo_events            RENAME TO notygo_event;
ALTER TABLE notygo_event_runs        RENAME TO notygo_event_run;
ALTER TABLE notygo_deliveries        RENAME TO notygo_delivery;
ALTER TABLE notygo_delivery_attempts RENAME TO notygo_delivery_attempt;
ALTER TABLE notygo_dedupe_keys       RENAME TO notygo_dedupe_key;
ALTER TABLE notygo_audit_logs        RENAME TO notygo_audit_log;
ALTER TABLE notygo_users             RENAME TO notygo_user;
ALTER TABLE notygo_sessions          RENAME TO notygo_session;

ALTER INDEX idx_events_endpoint_status    RENAME TO idx_event_endpoint_status;
ALTER INDEX idx_events_status_received   RENAME TO idx_event_status_received;
ALTER INDEX idx_events_upstream_event_id RENAME TO idx_event_upstream_event_id;
ALTER INDEX idx_events_payload_hash      RENAME TO idx_event_payload_hash;
ALTER INDEX idx_event_runs_event_id      RENAME TO idx_event_run_event_id;
ALTER INDEX idx_deliveries_status_next   RENAME TO idx_delivery_status_next;
ALTER INDEX idx_deliveries_event_id      RENAME TO idx_delivery_event_id;
ALTER INDEX idx_delivery_attempts_delivery RENAME TO idx_delivery_attempt_delivery;
ALTER INDEX idx_audit_logs_target        RENAME TO idx_audit_log_target;
ALTER INDEX idx_audit_logs_created       RENAME TO idx_audit_log_created;
ALTER INDEX uq_dedupe_keys_rule_hash     RENAME TO uq_dedupe_key_rule_hash;

ALTER TABLE notygo_endpoint RENAME CONSTRAINT notygo_endpoints_auth_type_check TO notygo_endpoint_auth_type_check;

-- +goose Down

ALTER TABLE notygo_endpoint RENAME CONSTRAINT notygo_endpoint_auth_type_check TO notygo_endpoints_auth_type_check;

ALTER INDEX uq_dedupe_key_rule_hash       RENAME TO uq_dedupe_keys_rule_hash;
ALTER INDEX idx_audit_log_created         RENAME TO idx_audit_logs_created;
ALTER INDEX idx_audit_log_target          RENAME TO idx_audit_logs_target;
ALTER INDEX idx_delivery_attempt_delivery RENAME TO idx_delivery_attempts_delivery;
ALTER INDEX idx_delivery_event_id         RENAME TO idx_deliveries_event_id;
ALTER INDEX idx_delivery_status_next      RENAME TO idx_deliveries_status_next;
ALTER INDEX idx_event_run_event_id        RENAME TO idx_event_runs_event_id;
ALTER INDEX idx_event_payload_hash        RENAME TO idx_events_payload_hash;
ALTER INDEX idx_event_upstream_event_id   RENAME TO idx_events_upstream_event_id;
ALTER INDEX idx_event_status_received     RENAME TO idx_events_status_received;
ALTER INDEX idx_event_endpoint_status     RENAME TO idx_events_endpoint_status;

ALTER TABLE notygo_session          RENAME TO notygo_sessions;
ALTER TABLE notygo_user             RENAME TO notygo_users;
ALTER TABLE notygo_audit_log        RENAME TO notygo_audit_logs;
ALTER TABLE notygo_dedupe_key       RENAME TO notygo_dedupe_keys;
ALTER TABLE notygo_delivery_attempt RENAME TO notygo_delivery_attempts;
ALTER TABLE notygo_delivery         RENAME TO notygo_deliveries;
ALTER TABLE notygo_event_run        RENAME TO notygo_event_runs;
ALTER TABLE notygo_event            RENAME TO notygo_events;
ALTER TABLE notygo_channel          RENAME TO notygo_channels;
ALTER TABLE notygo_template         RENAME TO notygo_templates;
ALTER TABLE notygo_rule             RENAME TO notygo_rules;
ALTER TABLE notygo_mapping          RENAME TO notygo_mappings;
ALTER TABLE notygo_endpoint         RENAME TO notygo_endpoints;
