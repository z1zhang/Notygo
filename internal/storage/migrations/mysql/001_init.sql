-- zPush V1 Schema (MySQL)
-- Consolidated baseline: the final schema after squashing the incremental
-- migrations 002-011. Fresh databases run this single file.
-- +goose Up

CREATE TABLE sources (
    id              VARCHAR(36) PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) NOT NULL UNIQUE,
    enabled         TINYINT(1) NOT NULL DEFAULT 1,
    auth_type       VARCHAR(50) NOT NULL DEFAULT 'none',
    secret_ref      TEXT,
    payload_content_type VARCHAR(100) NOT NULL DEFAULT 'application/json',
    max_body_bytes  INT NOT NULL DEFAULT 1048576,
    rate_limit      TEXT,
    mapping_id      VARCHAR(36),
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE mappings (
    id              VARCHAR(36) PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    revision        INT NOT NULL DEFAULT 1,
    rule_ids        JSON NULL,
    continue_on_match TINYINT(1) NOT NULL DEFAULT 0,
    fields          JSON NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE rules (
    id              VARCHAR(36) PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    enabled         TINYINT(1) NOT NULL DEFAULT 1,
    priority        INT NOT NULL DEFAULT 0,
    conditions      JSON NOT NULL,
    template_id     VARCHAR(36),
    destination_ids JSON NOT NULL,
    dedupe_config   JSON,
    revision        INT NOT NULL DEFAULT 1,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE templates (
    id              VARCHAR(36) PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    format          VARCHAR(20) NOT NULL DEFAULT 'markdown',
    title_template  TEXT NOT NULL,
    body_template   TEXT NOT NULL,
    revision        INT NOT NULL DEFAULT 1,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE destinations (
    id              VARCHAR(36) PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    type            VARCHAR(50) NOT NULL,
    enabled         TINYINT(1) NOT NULL DEFAULT 1,
    config          JSON NOT NULL,
    secret_ref      TEXT,
    timeout_ms      INT NOT NULL DEFAULT 10000,
    max_attempts    INT NOT NULL DEFAULT 5,
    revision        INT NOT NULL DEFAULT 1,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE events (
    id              VARCHAR(36) PRIMARY KEY,
    source_id       VARCHAR(36) NOT NULL,
    source_event_id VARCHAR(255),
    request_id      VARCHAR(255) NOT NULL,
    received_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    content_type    VARCHAR(100) NOT NULL DEFAULT 'application/json',
    payload_raw     LONGTEXT NOT NULL,
    payload_hash    VARCHAR(64) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'received',
    run_type        VARCHAR(20) NOT NULL DEFAULT 'initial',
    error_code      VARCHAR(20),
    error_summary   TEXT,
    lease_until     DATETIME,
    lease_owner     VARCHAR(64)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE event_runs (
    id              VARCHAR(36) PRIMARY KEY,
    event_id        VARCHAR(36) NOT NULL,
    run_type        VARCHAR(20) NOT NULL DEFAULT 'initial',
    mapping_id      VARCHAR(36),
    mapping_revision INT,
    rule_set_snapshot JSON NOT NULL,
    parsed_fields   JSON NOT NULL,
    validation_result JSON NOT NULL,
    matched_rule_ids JSON NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'processing',
    started_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at     DATETIME,
    error_code      VARCHAR(20),
    error_summary   TEXT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE deliveries (
    id              VARCHAR(36) PRIMARY KEY,
    event_id        VARCHAR(36) NOT NULL,
    event_run_id    VARCHAR(36) NOT NULL,
    rule_id         VARCHAR(36) NOT NULL,
    destination_id  VARCHAR(36) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL UNIQUE,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempt_count   INT NOT NULL DEFAULT 0,
    max_attempts    INT NOT NULL DEFAULT 5,
    next_attempt_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    lease_until     DATETIME,
    lease_owner     VARCHAR(64),
    rendered_title  TEXT,
    rendered_body   TEXT,
    rendered_format VARCHAR(20),
    last_error_code VARCHAR(20),
    last_error_summary TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE delivery_attempts (
    id              VARCHAR(36) PRIMARY KEY,
    delivery_id     VARCHAR(36) NOT NULL,
    attempt_no      INT NOT NULL,
    started_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at     DATETIME,
    request_summary TEXT,
    response_status INT,
    response_summary TEXT,
    error_code      VARCHAR(20),
    error_summary   TEXT,
    duration_ms     INT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE dedupe_keys (
    id          VARCHAR(36) PRIMARY KEY,
    rule_id     VARCHAR(36) NOT NULL,
    dedupe_hash VARCHAR(64) NOT NULL,
    expires_at  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_dedupe_lookup (rule_id, dedupe_hash, expires_at),
    INDEX idx_dedupe_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE audit_logs (
    id              VARCHAR(36) PRIMARY KEY,
    operator        VARCHAR(255) NOT NULL,
    action          VARCHAR(50) NOT NULL,
    target_type     VARCHAR(50) NOT NULL,
    target_id       VARCHAR(36) NOT NULL,
    snapshot_before JSON,
    summary         TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE admin_users (
    id              VARCHAR(36) PRIMARY KEY,
    username        VARCHAR(255) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE sessions (
    id              VARCHAR(36) PRIMARY KEY,
    user_id         VARCHAR(36) NOT NULL,
    token           VARCHAR(255) NOT NULL UNIQUE,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at      DATETIME NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

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
DROP TABLE IF EXISTS dedupe_keys;
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
