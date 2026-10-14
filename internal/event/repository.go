package event

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"
	"zpush/internal/storage"
)

// ErrNotFound is returned when an event is not found.
var ErrNotFound = errors.New("event not found")

// ErrAlreadyClaimed is returned when an event cannot be claimed.
var ErrAlreadyClaimed = errors.New("event already claimed")

func newID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// Repository provides operations for events and event runs.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new event repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Insert inserts a new event and returns it.
func (r *Repository) Insert(ctx context.Context, e *Event) (*Event, error) {
	if e.ID == "" {
		e.ID = newID()
	}
	if e.ReceivedAt.IsZero() {
		e.ReceivedAt = time.Now().UTC()
	}

	var expireAt *string
	if e.ExpireAt != nil {
		s := storage.FormatTime(*e.ExpireAt)
		expireAt = &s
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO events (id, source_id, source_event_id, request_id, received_at, content_type, payload_raw, payload_hash, status, run_type, error_code, error_summary, expire_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.SourceID, e.SourceEventID, e.RequestID,
		storage.FormatTime(e.ReceivedAt),
		e.ContentType, e.PayloadRaw, e.PayloadHash,
		e.Status, e.RunType, e.ErrorCode, e.ErrorSummary, expireAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert event: %w", err)
	}

	return r.GetByID(ctx, e.ID)
}

// GetByID retrieves an event by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Event, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, source_id, source_event_id, request_id, received_at, content_type, payload_raw, payload_hash, status, run_type, error_code, error_summary, expire_at, lease_until, lease_owner
		 FROM events WHERE id = ?`, id)
	return scanEvent(row)
}

// List returns a paginated list of events and the total count.
// If statusFilter is non-empty, only events with that status are returned.
func (r *Repository) List(ctx context.Context, page, size int, statusFilter string) ([]Event, int64, error) {
	var total int64
	countQuery := `SELECT COUNT(*) FROM events`
	listQuery := `SELECT id, source_id, source_event_id, request_id, received_at, content_type, payload_raw, payload_hash, status, run_type, error_code, error_summary, expire_at, lease_until, lease_owner
		 FROM events`
	var args []any

	if statusFilter != "" {
		countQuery += ` WHERE status = ?`
		listQuery += ` WHERE status = ?`
		args = append(args, statusFilter)
	}

	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count events: %w", err)
	}
	if total == 0 {
		return []Event{}, 0, nil
	}

	listQuery += ` ORDER BY received_at DESC LIMIT ? OFFSET ?`
	offset := (page - 1) * size
	queryArgs := append(args, size, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var result []Event
	for rows.Next() {
		e, err := scanEventFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *e)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate events: %w", err)
	}

	return result, total, nil
}

// ClaimEvent attempts to claim an event for processing by the given worker.
// It returns the claimed event or ErrAlreadyClaimed if none available.
func (r *Repository) ClaimEvent(ctx context.Context, workerID string, leaseTimeout time.Duration) (*Event, error) {
	leaseUntil := storage.FormatTime(time.Now().UTC().Add(leaseTimeout))

	// Step 1: Find the oldest claimable event ID
	var id string
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM events WHERE status = 'received' ORDER BY received_at ASC LIMIT 1`,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAlreadyClaimed
		}
		return nil, fmt.Errorf("find claimable event: %w", err)
	}

	// Step 2: Claim it by ID (atomic, status guard prevents double-claiming)
	result, err := r.db.ExecContext(ctx,
		`UPDATE events SET status = ?, lease_until = ?, lease_owner = ? WHERE id = ? AND status = 'received'`,
		EventStatusProcessing, leaseUntil, workerID, id,
	)
	if err != nil {
		return nil, fmt.Errorf("claim event: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return nil, ErrAlreadyClaimed // someone else claimed it between step 1 and 2
	}

	// Step 3: Fetch by ID — guaranteed to be the right one
	return r.GetByID(ctx, id)
}

// MarkProcessed marks an event as completed.
// Includes a status guard (AND status = 'processing') to prevent marking an
// event as completed if lease recovery already reset it to 'received'.
func (r *Repository) MarkProcessed(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE events SET status = ?, lease_until = NULL, lease_owner = NULL WHERE id = ? AND status = ?`,
		EventStatusCompleted, id, EventStatusProcessing,
	)
	if err != nil {
		return fmt.Errorf("mark event processed: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		slog.Warn("mark event processed skipped (not in processing status)", "event_id", id)
	}
	return nil
}

// MarkDeduped marks an event as deduped (all matched rules were dedupe-skipped).
func (r *Repository) MarkDeduped(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE events SET status = ?, lease_until = NULL, lease_owner = NULL WHERE id = ? AND status = ?`,
		EventStatusDeduped, id, EventStatusProcessing,
	)
	if err != nil {
		return fmt.Errorf("mark event deduped: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		slog.Warn("mark event deduped skipped (not in processing status)", "event_id", id)
	}
	return nil
}

// MarkFailed marks an event as failed.
func (r *Repository) MarkFailed(ctx context.Context, id string, errorCode, errorSummary string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE events SET status = ?, error_code = ?, error_summary = ?, lease_until = NULL, lease_owner = NULL WHERE id = ?`,
		EventStatusFailed, errorCode, errorSummary, id,
	)
	if err != nil {
		return fmt.Errorf("mark event failed: %w", err)
	}
	return nil
}

// SetStatus updates the event status.
func (r *Repository) SetStatus(ctx context.Context, id string, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE events SET status = ? WHERE id = ?`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("set event status: %w", err)
	}
	return nil
}

// SetForReplay resets the event for replay: sets status to 'received' and run_type to 'replay'.
func (r *Repository) SetForReplay(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE events SET status = ?, run_type = ?, lease_until = NULL, lease_owner = NULL WHERE id = ?`,
		EventStatusReceived, EventRunTypeReplay, id,
	)
	if err != nil {
		return fmt.Errorf("set event for replay: %w", err)
	}
	return nil
}

// CreateEventRun inserts a new event run and returns it.
func (r *Repository) CreateEventRun(ctx context.Context, run *EventRun) (*EventRun, error) {
	id := newID()
	if run.ID == "" {
		run.ID = id
	}
	nowStr := storage.FormatNow()

	parsedFieldsJSON, err := json.Marshal(run.ParsedFields)
	if err != nil {
		return nil, fmt.Errorf("marshal parsed_fields: %w", err)
	}
	validationJSON, err := json.Marshal(run.ValidationResult)
	if err != nil {
		return nil, fmt.Errorf("marshal validation_result: %w", err)
	}
	ruleSnapshotJSON, err := json.Marshal(run.RuleSetSnapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal rule_set_snapshot: %w", err)
	}
	matchedRuleIDsJSON, err := json.Marshal(run.MatchedRuleIDs)
	if err != nil {
		return nil, fmt.Errorf("marshal matched_rule_ids: %w", err)
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO event_runs (id, event_id, run_type, mapping_id, mapping_revision, rule_set_snapshot, parsed_fields, validation_result, matched_rule_ids, status, started_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.EventID, run.RunType, run.MappingID, run.MappingRevision,
		string(ruleSnapshotJSON), string(parsedFieldsJSON), string(validationJSON),
		string(matchedRuleIDsJSON), run.Status, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert event_run: %w", err)
	}

	return r.GetEventRunByID(ctx, run.ID)
}

// GetEventRunByID retrieves an event run by its ID.
func (r *Repository) GetEventRunByID(ctx context.Context, id string) (*EventRun, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, event_id, run_type, mapping_id, mapping_revision, rule_set_snapshot, parsed_fields, validation_result, matched_rule_ids, status, started_at, finished_at, error_code, error_summary
		 FROM event_runs WHERE id = ?`, id)
	return scanEventRun(row)
}

// GetLatestEventRun retrieves the most recent event run for a given event.
func (r *Repository) GetLatestEventRun(ctx context.Context, eventID string) (*EventRun, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, event_id, run_type, mapping_id, mapping_revision, rule_set_snapshot, parsed_fields, validation_result, matched_rule_ids, status, started_at, finished_at, error_code, error_summary
		 FROM event_runs WHERE event_id = ? ORDER BY started_at DESC LIMIT 1`, eventID)
	return scanEventRun(row)
}

// CompleteEventRun marks an event run as completed.
func (r *Repository) CompleteEventRun(ctx context.Context, id string, matchedRuleIDs []string) error {
	nowStr := storage.FormatNow()
	matchedJSON, err := json.Marshal(matchedRuleIDs)
	if err != nil {
		return fmt.Errorf("marshal matched_rule_ids: %w", err)
	}

	_, err = r.db.ExecContext(ctx,
		`UPDATE event_runs SET status = ?, finished_at = ?, matched_rule_ids = ? WHERE id = ?`,
		EventRunStatusCompleted, nowStr, string(matchedJSON), id,
	)
	if err != nil {
		return fmt.Errorf("complete event_run: %w", err)
	}
	return nil
}

// FailEventRun marks an event run as failed.
func (r *Repository) FailEventRun(ctx context.Context, id string, errorCode, errorSummary string) error {
	nowStr := storage.FormatNow()

	_, err := r.db.ExecContext(ctx,
		`UPDATE event_runs SET status = ?, finished_at = ?, error_code = ?, error_summary = ? WHERE id = ?`,
		EventRunStatusFailed, nowStr, errorCode, errorSummary, id,
	)
	if err != nil {
		return fmt.Errorf("fail event_run: %w", err)
	}
	return nil
}

func scanEvent(row *sql.Row) (*Event, error) {
	var e Event
	var receivedAt string
	var expireAt, leaseUntil sql.NullString
	var leaseOwner sql.NullString

	err := row.Scan(
		&e.ID, &e.SourceID, &e.SourceEventID, &e.RequestID, &receivedAt,
		&e.ContentType, &e.PayloadRaw, &e.PayloadHash, &e.Status, &e.RunType,
		&e.ErrorCode, &e.ErrorSummary, &expireAt, &leaseUntil, &leaseOwner,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan event: %w", err)
	}

	e.ReceivedAt, _ = storage.ParseTime(receivedAt)
	if expireAt.Valid {
		t, _ := storage.ParseTime(expireAt.String)
		e.ExpireAt = &t
	}
	if leaseUntil.Valid {
		t, _ := storage.ParseTime(leaseUntil.String)
		e.LeaseUntil = &t
	}
	if leaseOwner.Valid {
		e.LeaseOwner = &leaseOwner.String
	}

	return &e, nil
}

func scanEventValue(row *sql.Row) (Event, error) {
	e, err := scanEvent(row)
	if err != nil {
		return Event{}, err
	}
	return *e, nil
}

func scanEventFromRows(rows *sql.Rows) (*Event, error) {
	var e Event
	var receivedAt string
	var expireAt, leaseUntil sql.NullString
	var leaseOwner sql.NullString

	err := rows.Scan(
		&e.ID, &e.SourceID, &e.SourceEventID, &e.RequestID, &receivedAt,
		&e.ContentType, &e.PayloadRaw, &e.PayloadHash, &e.Status, &e.RunType,
		&e.ErrorCode, &e.ErrorSummary, &expireAt, &leaseUntil, &leaseOwner,
	)
	if err != nil {
		return nil, fmt.Errorf("scan event: %w", err)
	}

	e.ReceivedAt, _ = storage.ParseTime(receivedAt)
	if expireAt.Valid {
		t, _ := storage.ParseTime(expireAt.String)
		e.ExpireAt = &t
	}
	if leaseUntil.Valid {
		t, _ := storage.ParseTime(leaseUntil.String)
		e.LeaseUntil = &t
	}
	if leaseOwner.Valid {
		e.LeaseOwner = &leaseOwner.String
	}

	return &e, nil
}

func scanEventRun(row *sql.Row) (*EventRun, error) {
	var run EventRun
	var startedAt string
	var finishedAt sql.NullString
	var parsedFieldsJSON, validationJSON, ruleSnapshotJSON, matchedRuleIDsJSON string

	err := row.Scan(
		&run.ID, &run.EventID, &run.RunType, &run.MappingID, &run.MappingRevision,
		&ruleSnapshotJSON, &parsedFieldsJSON, &validationJSON, &matchedRuleIDsJSON,
		&run.Status, &startedAt, &finishedAt, &run.ErrorCode, &run.ErrorSummary,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan event_run: %w", err)
	}

	run.StartedAt, _ = storage.ParseTime(startedAt)
	if finishedAt.Valid {
		t, _ := storage.ParseTime(finishedAt.String)
		run.FinishedAt = &t
	}

	if err := json.Unmarshal([]byte(parsedFieldsJSON), &run.ParsedFields); err != nil {
		run.ParsedFields = map[string]any{}
	}
	if err := json.Unmarshal([]byte(validationJSON), &run.ValidationResult); err != nil {
		run.ValidationResult = map[string]any{}
	}
	if err := json.Unmarshal([]byte(ruleSnapshotJSON), &run.RuleSetSnapshot); err != nil {
		run.RuleSetSnapshot = []RuleSnapshot{}
	}
	if err := json.Unmarshal([]byte(matchedRuleIDsJSON), &run.MatchedRuleIDs); err != nil {
		run.MatchedRuleIDs = []string{}
	}

	return &run, nil
}
