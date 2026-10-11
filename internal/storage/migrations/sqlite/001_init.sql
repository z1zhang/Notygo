-- zPush V1 Schema
-- +goose Up

CREATE TABLE sources (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    enabled         INTEGER NOT NULL DEFAULT 1,
    auth_type       TEXT NOT NULL DEFAULT 'none',
    secret_ref      TEXT,
    payload_content_type TEXT NOT NULL DEFAULT 'application/json',
    max_body_bytes  INTEGER NOT NULL DEFAULT 1048576,
    rate_limit      TEXT,
    mapping_id      TEXT,
    expected_response TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE mappings (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    revision        INTEGER NOT NULL DEFAULT 1,
    fields          TEXT NOT NULL DEFAULT '[]',
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE rules (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    enabled         INTEGER NOT NULL DEFAULT 1,
    priority        INTEGER NOT NULL DEFAULT 0,
    conditions      TEXT NOT NULL DEFAULT '{}',
    condition_mode  TEXT NOT NULL DEFAULT 'and',
    continue_on_match INTEGER NOT NULL DEFAULT 1,
    template_id     TEXT,
    destination_ids TEXT NOT NULL DEFAULT '[]',
    dedupe_config   TEXT,
    revision        INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE templates (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    format          TEXT NOT NULL DEFAULT 'markdown',
    title_template  TEXT NOT NULL DEFAULT '',
    body_template   TEXT NOT NULL DEFAULT '',
    revision        INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE destinations (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL,
    enabled         INTEGER NOT NULL DEFAULT 1,
    config          TEXT NOT NULL DEFAULT '{}',
    secret_ref      TEXT,
    timeout_ms      INTEGER NOT NULL DEFAULT 5000,
    rate_limit      TEXT,
    circuit_breaker TEXT,
    revision        INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE events (
    id              TEXT PRIMARY KEY,
    source_id       TEXT NOT NULL,
    source_event_id TEXT,
    request_id      TEXT NOT NULL,
    received_at     TEXT NOT NULL DEFAULT (datetime('now')),
    content_type    TEXT NOT NULL DEFAULT 'application/json',
    payload_raw     TEXT NOT NULL,
    payload_hash    TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'received',
    error_code      TEXT,
    error_summary   TEXT,
    expire_at       TEXT,
    lease_until     TEXT,
    lease_owner     TEXT
);

CREATE TABLE event_runs (
    id              TEXT PRIMARY KEY,
    event_id        TEXT NOT NULL,
    run_type        TEXT NOT NULL DEFAULT 'initial',
    mapping_id      TEXT,
    mapping_revision INTEGER,
    rule_set_snapshot TEXT NOT NULL DEFAULT '[]',
    parsed_fields   TEXT NOT NULL DEFAULT '{}',
    validation_result TEXT NOT NULL DEFAULT '{}',
    matched_rule_ids TEXT NOT NULL DEFAULT '[]',
    status          TEXT NOT NULL DEFAULT 'processing',
    started_at      TEXT NOT NULL DEFAULT (datetime('now')),
    finished_at     TEXT,
    error_code      TEXT,
    error_summary   TEXT
);

CREATE TABLE deliveries (
    id              TEXT PRIMARY KEY,
    event_id        TEXT NOT NULL,
    event_run_id    TEXT NOT NULL,
    rule_id         TEXT NOT NULL,
    destination_id  TEXT NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    status          TEXT NOT NULL DEFAULT 'pending',
    attempt_count   INTEGER NOT NULL DEFAULT 0,
    max_attempts    INTEGER NOT NULL DEFAULT 5,
    next_attempt_at TEXT NOT NULL DEFAULT (datetime('now')),
    lease_until     TEXT,
    lease_owner     TEXT,
    rendered_title  TEXT,
    rendered_body   TEXT,
    rendered_format TEXT,
    last_error_code TEXT,
    last_error_summary TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE delivery_attempts (
    id              TEXT PRIMARY KEY,
    delivery_id     TEXT NOT NULL,
    attempt_no      INTEGER NOT NULL,
    started_at      TEXT NOT NULL DEFAULT (datetime('now')),
    finished_at     TEXT,
    request_summary TEXT,
    response_status INTEGER,
    response_summary TEXT,
    error_code      TEXT,
    error_summary   TEXT,
    duration_ms     INTEGER
);

CREATE TABLE audit_logs (
    id              TEXT PRIMARY KEY,
    operator        TEXT NOT NULL,
    action          TEXT NOT NULL,
    target_type     TEXT NOT NULL,
    target_id       TEXT NOT NULL,
    snapshot_before TEXT,
    summary         TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE admin_users (
    id              TEXT PRIMARY KEY,
    username        TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE sessions (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL,
    token           TEXT NOT NULL UNIQUE,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at      TEXT NOT NULL
);

-- Indexes

CREATE INDEX idx_events_source_status ON events(source_id, status, received_at);
CREATE INDEX idx_events_status_received ON events(status, received_at);
CREATE INDEX idx_events_source_event_id ON events(source_id, source_event_id);
CREATE INDEX idx_events_payload_hash ON events(payload_hash);

CREATE INDEX idx_event_runs_event_id ON event_runs(event_id);

CREATE INDEX idx_deliveries_status_next ON deliveries(status, next_attempt_at);
CREATE INDEX idx_deliveries_lease ON deliveries(status, lease_until);
CREATE INDEX idx_deliveries_event_id ON deliveries(event_id);

CREATE INDEX idx_delivery_attempts_delivery ON delivery_attempts(delivery_id, attempt_no);

CREATE INDEX idx_audit_logs_target ON audit_logs(target_type, target_id);
CREATE INDEX idx_audit_logs_created ON audit_logs(created_at);

-- +goose Down

DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS delivery_attempts;
DROP TABLE IF EXISTS deliveries;
DROP TABLE IF EXISTS event_runs;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS admin_users;
DROP TABLE IF EXISTS destinations;
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS rules;
DROP TABLE IF EXISTS mappings;
DROP TABLE IF EXISTS sources;
