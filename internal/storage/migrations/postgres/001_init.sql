-- Notygo V1 Schema (PostgreSQL)
-- Port of the squashed MySQL baseline to PostgreSQL dialect:
-- JSON → JSONB, DATETIME → TIMESTAMPTZ, TINYINT(1) → BOOLEAN, LONGTEXT → TEXT.
--
-- Enum-semantic columns (status / run_type / format / auth_type) are SMALLINT
-- codes guarded by CHECK constraints. Code tables (keep in sync with the
-- per-package code maps in internal/<package>/codes.go):
--   notygo_events.status        0=received 1=processing 2=completed 3=deduped 4=failed
--   notygo_event_runs.status    0=processing 1=completed 2=failed
--   run_type (both)      0=initial 1=replay
--   notygo_deliveries.status    0=pending 1=retry_wait 2=processing 3=delivered 4=failed 5=skipped
--   format (both)        0=text 1=markdown 2=html
--   notygo_endpoints.auth_type    0=none 1=bearer
-- The API and Go models keep the string names; conversion happens only at the
-- repository layer.
--
-- Retry scheduling lives in River (river_job, created/managed by River's own
-- migrator at boot — see internal/queue/manager.go Migrate). The business
-- tables below only record outcomes: notygo_deliveries.next_attempt_at is a
-- display-only prediction, not the schedule.
-- +goose Up

CREATE TABLE notygo_endpoints (
    id              VARCHAR(36) PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) NOT NULL UNIQUE,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    auth_type       SMALLINT NOT NULL DEFAULT 0 CHECK (auth_type IN (0, 1)),
    encrypted_secret TEXT,
    payload_content_type VARCHAR(100) NOT NULL DEFAULT 'application/json',
    max_body_bytes  INT NOT NULL DEFAULT 1048576,
    rate_limit      TEXT,
    mapping_id      VARCHAR(36),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notygo_mappings (
    id              VARCHAR(36) PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    revision        INT NOT NULL DEFAULT 1,
    rule_ids        JSONB,
    continue_on_match BOOLEAN NOT NULL DEFAULT FALSE,
    fields          JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notygo_rules (
    id              VARCHAR(36) PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    priority        INT NOT NULL DEFAULT 0,
    conditions      JSONB NOT NULL,
    template_id     VARCHAR(36),
    channel_ids JSONB NOT NULL,
    dedupe_config   JSONB,
    revision        INT NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notygo_templates (
    id              VARCHAR(36) PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    format          SMALLINT NOT NULL DEFAULT 1 CHECK (format IN (0, 1, 2)),
    title_template  TEXT NOT NULL,
    body_template   TEXT NOT NULL,
    revision        INT NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notygo_channels (
    id              VARCHAR(36) PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    type            VARCHAR(50) NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    config          JSONB NOT NULL,
    encrypted_secret TEXT,
    timeout_ms      INT NOT NULL DEFAULT 10000,
    max_attempts    INT NOT NULL DEFAULT 5,
    revision        INT NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notygo_events (
    id              VARCHAR(36) PRIMARY KEY,
    endpoint_id       VARCHAR(36) NOT NULL,
    upstream_event_id VARCHAR(255),
    request_id      VARCHAR(255) NOT NULL,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    content_type    VARCHAR(100) NOT NULL DEFAULT 'application/json',
    payload_raw     TEXT NOT NULL,
    payload_hash    VARCHAR(64) NOT NULL,
    status          SMALLINT NOT NULL DEFAULT 0 CHECK (status IN (0, 1, 2, 3, 4)),
    run_type        SMALLINT NOT NULL DEFAULT 0 CHECK (run_type IN (0, 1)),
    error_code      VARCHAR(20),
    error_summary   TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notygo_event_runs (
    id              VARCHAR(36) PRIMARY KEY,
    event_id        VARCHAR(36) NOT NULL,
    run_type        SMALLINT NOT NULL DEFAULT 0 CHECK (run_type IN (0, 1)),
    mapping_id      VARCHAR(36),
    mapping_revision INT,
    rule_set_snapshot JSONB NOT NULL,
    parsed_fields   JSONB NOT NULL,
    validation_result JSONB NOT NULL,
    matched_rule_ids JSONB NOT NULL,
    status          SMALLINT NOT NULL DEFAULT 0 CHECK (status IN (0, 1, 2)),
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at     TIMESTAMPTZ,
    error_code      VARCHAR(20),
    error_summary   TEXT
);

CREATE TABLE notygo_deliveries (
    id              VARCHAR(36) PRIMARY KEY,
    event_id        VARCHAR(36) NOT NULL,
    event_run_id    VARCHAR(36) NOT NULL,
    rule_id         VARCHAR(36) NOT NULL,
    channel_id  VARCHAR(36) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL UNIQUE,
    status          SMALLINT NOT NULL DEFAULT 0 CHECK (status IN (0, 1, 2, 3, 4, 5)),
    attempt_count   INT NOT NULL DEFAULT 0,
    max_attempts    INT NOT NULL DEFAULT 5,
    -- display-only prediction of River's next attempt, for the UI
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    rendered_title  TEXT,
    rendered_body   TEXT,
    rendered_format SMALLINT CHECK (rendered_format IN (0, 1, 2)),
    last_error_code VARCHAR(20),
    last_error_summary TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notygo_delivery_attempts (
    id              VARCHAR(36) PRIMARY KEY,
    delivery_id     VARCHAR(36) NOT NULL,
    attempt_no      INT NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at     TIMESTAMPTZ,
    request_summary TEXT,
    response_status INT,
    response_summary TEXT,
    error_code      VARCHAR(20),
    error_summary   TEXT,
    duration_ms     INT
);

CREATE TABLE notygo_dedupe_keys (
    id          VARCHAR(36) PRIMARY KEY,
    rule_id     VARCHAR(36) NOT NULL,
    dedupe_hash VARCHAR(64) NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notygo_audit_logs (
    id              VARCHAR(36) PRIMARY KEY,
    operator        VARCHAR(255) NOT NULL,
    action          VARCHAR(50) NOT NULL,
    target_type     VARCHAR(50) NOT NULL,
    target_id       VARCHAR(36) NOT NULL,
    snapshot_before JSONB,
    summary         TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notygo_users (
    id              VARCHAR(36) PRIMARY KEY,
    username        VARCHAR(255) NOT NULL UNIQUE,
    password        VARCHAR(255) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notygo_sessions (
    id              VARCHAR(36) PRIMARY KEY,
    user_id         VARCHAR(36) NOT NULL,
    token_hash      VARCHAR(64) NOT NULL UNIQUE, -- sha256 hex of the bearer token; raw token only ever lives in the browser
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL
);

-- Indexes

CREATE INDEX idx_events_endpoint_status ON notygo_events(endpoint_id, status, received_at);
CREATE INDEX idx_events_status_received ON notygo_events(status, received_at);
CREATE INDEX idx_events_upstream_event_id ON notygo_events(endpoint_id, upstream_event_id);
CREATE INDEX idx_events_payload_hash ON notygo_events(payload_hash);

CREATE INDEX idx_event_runs_event_id ON notygo_event_runs(event_id);

CREATE INDEX idx_deliveries_status_next ON notygo_deliveries(status, next_attempt_at);
CREATE INDEX idx_deliveries_event_id ON notygo_deliveries(event_id);

CREATE INDEX idx_delivery_attempts_delivery ON notygo_delivery_attempts(delivery_id, attempt_no);

CREATE INDEX idx_dedupe_lookup ON notygo_dedupe_keys(rule_id, dedupe_hash, expires_at);
CREATE INDEX idx_dedupe_expires ON notygo_dedupe_keys(expires_at);

CREATE INDEX idx_audit_logs_target ON notygo_audit_logs(target_type, target_id);
CREATE INDEX idx_audit_logs_created ON notygo_audit_logs(created_at);

-- +goose Down

DROP TABLE IF EXISTS notygo_sessions;
DROP TABLE IF EXISTS notygo_audit_logs;
DROP TABLE IF EXISTS notygo_dedupe_keys;
DROP TABLE IF EXISTS notygo_delivery_attempts;
DROP TABLE IF EXISTS notygo_deliveries;
DROP TABLE IF EXISTS notygo_event_runs;
DROP TABLE IF EXISTS notygo_events;
DROP TABLE IF EXISTS notygo_users;
DROP TABLE IF EXISTS notygo_channels;
DROP TABLE IF EXISTS notygo_templates;
DROP TABLE IF EXISTS notygo_rules;
DROP TABLE IF EXISTS notygo_mappings;
DROP TABLE IF EXISTS notygo_endpoints;
