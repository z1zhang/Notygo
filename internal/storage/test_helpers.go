package storage

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// OpenTestDB creates an in-memory SQLite database with migrations applied
// for use in unit/integration tests.
func OpenTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	db.SetMaxOpenConns(1)

	// Apply pragmas
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			t.Fatalf("exec pragma %q: %v", p, err)
		}
	}

	// Apply schema directly (no goose in test mode for simplicity)
	schema := `
	CREATE TABLE IF NOT EXISTS sources (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, slug TEXT NOT NULL UNIQUE,
		enabled INTEGER NOT NULL DEFAULT 1, auth_type TEXT NOT NULL DEFAULT 'none',
		secret_ref TEXT, payload_content_type TEXT NOT NULL DEFAULT 'application/json',
		max_body_bytes INTEGER NOT NULL DEFAULT 1048576, rate_limit TEXT,
		mapping_id TEXT, expected_response TEXT,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	);
	CREATE TABLE IF NOT EXISTS mappings (
		id TEXT PRIMARY KEY, name TEXT NOT NULL,
		revision INTEGER NOT NULL DEFAULT 1,
		fields TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	);
	CREATE TABLE IF NOT EXISTS rules (
		id TEXT PRIMARY KEY, name TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1, priority INTEGER NOT NULL DEFAULT 0,
		conditions TEXT NOT NULL DEFAULT '{}', condition_mode TEXT NOT NULL DEFAULT 'and',
		continue_on_match INTEGER NOT NULL DEFAULT 1,
		template_id TEXT, destination_ids TEXT NOT NULL DEFAULT '[]',
		dedupe_config TEXT, revision INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	);
	CREATE TABLE IF NOT EXISTS templates (
		id TEXT PRIMARY KEY, name TEXT NOT NULL,
		format TEXT NOT NULL DEFAULT 'markdown',
		title_template TEXT NOT NULL DEFAULT '', body_template TEXT NOT NULL DEFAULT '',
		revision INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	);
	CREATE TABLE IF NOT EXISTS destinations (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1, config TEXT NOT NULL DEFAULT '{}',
		secret_ref TEXT, timeout_ms INTEGER NOT NULL DEFAULT 5000,
		rate_limit TEXT, circuit_breaker TEXT,
		revision INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	);
	CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY, source_id TEXT NOT NULL,
		source_event_id TEXT, request_id TEXT NOT NULL,
		received_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		content_type TEXT NOT NULL DEFAULT 'application/json',
		payload_raw TEXT NOT NULL, payload_hash TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'received',
		error_code TEXT, error_summary TEXT,
		expire_at TEXT, lease_until TEXT, lease_owner TEXT
	);
	CREATE TABLE IF NOT EXISTS event_runs (
		id TEXT PRIMARY KEY, event_id TEXT NOT NULL,
		run_type TEXT NOT NULL DEFAULT 'initial',
		mapping_id TEXT, mapping_revision INTEGER,
		rule_set_snapshot TEXT NOT NULL DEFAULT '[]',
		parsed_fields TEXT NOT NULL DEFAULT '{}',
		validation_result TEXT NOT NULL DEFAULT '{}',
		matched_rule_ids TEXT NOT NULL DEFAULT '[]',
		status TEXT NOT NULL DEFAULT 'processing',
		started_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		finished_at TEXT, error_code TEXT, error_summary TEXT
	);
	CREATE TABLE IF NOT EXISTS deliveries (
		id TEXT PRIMARY KEY, event_id TEXT NOT NULL,
		event_run_id TEXT NOT NULL, rule_id TEXT NOT NULL,
		destination_id TEXT NOT NULL,
		idempotency_key TEXT NOT NULL UNIQUE,
		status TEXT NOT NULL DEFAULT 'pending',
		attempt_count INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL DEFAULT 5,
		next_attempt_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		lease_until TEXT, lease_owner TEXT,
		rendered_title TEXT, rendered_body TEXT, rendered_format TEXT,
		last_error_code TEXT, last_error_summary TEXT,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	);
	CREATE TABLE IF NOT EXISTS delivery_attempts (
		id TEXT PRIMARY KEY, delivery_id TEXT NOT NULL,
		attempt_no INTEGER NOT NULL,
		started_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		finished_at TEXT, request_summary TEXT,
		response_status INTEGER, response_summary TEXT,
		error_code TEXT, error_summary TEXT,
		duration_ms INTEGER
	);
	CREATE TABLE IF NOT EXISTS audit_logs (
		id TEXT PRIMARY KEY, operator TEXT NOT NULL,
		action TEXT NOT NULL, target_type TEXT NOT NULL,
		target_id TEXT NOT NULL, snapshot_before TEXT,
		summary TEXT,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	);
	CREATE TABLE IF NOT EXISTS admin_users (
		id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	);
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY, user_id TEXT NOT NULL,
		token TEXT NOT NULL UNIQUE,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
		expires_at TEXT NOT NULL
	);
	-- Indexes
	CREATE INDEX IF NOT EXISTS idx_events_source_status ON events(source_id, status, received_at);
	CREATE INDEX IF NOT EXISTS idx_events_status_received ON events(status, received_at);
	CREATE INDEX IF NOT EXISTS idx_events_source_event_id ON events(source_id, source_event_id);
	CREATE INDEX IF NOT EXISTS idx_events_payload_hash ON events(payload_hash);
	CREATE INDEX IF NOT EXISTS idx_event_runs_event_id ON event_runs(event_id);
	CREATE INDEX IF NOT EXISTS idx_deliveries_status_next ON deliveries(status, next_attempt_at);
	CREATE INDEX IF NOT EXISTS idx_deliveries_lease ON deliveries(status, lease_until);
	CREATE INDEX IF NOT EXISTS idx_deliveries_event_id ON deliveries(event_id);
	CREATE INDEX IF NOT EXISTS idx_delivery_attempts_delivery ON delivery_attempts(delivery_id, attempt_no);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_target ON audit_logs(target_type, target_id);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("apply test schema: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}
