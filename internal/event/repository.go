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

	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
	"notygo/internal/storage"
)

// ErrNotFound is returned when an event is not found.
var ErrNotFound = errors.New("event not found")

func newID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// Repository provides operations for events and event runs.
type Repository struct {
	db storage.DBTX
}

// NewRepository creates a new event repository.
func NewRepository(db storage.DBTX) *Repository {
	return &Repository{db: db}
}

// WithTx returns a copy of the repository bound to the given transaction, so
// callers can run the same statements atomically (the River job insert and
// the event row writes share one transaction).
func (r *Repository) WithTx(db storage.DBTX) *Repository {
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
	statusCode, err := StatusToCode(e.Status)
	if err != nil {
		return nil, fmt.Errorf("insert event: %w", err)
	}
	runTypeCode, err := RunTypeToCode(e.RunType)
	if err != nil {
		return nil, fmt.Errorf("insert event: %w", err)
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO notygo_events (id, endpoint_id, upstream_event_id, request_id, received_at, content_type, payload_raw, payload_hash, status, run_type, error_code, error_summary)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		e.ID, e.EndpointID, e.UpstreamEventID, e.RequestID,
		e.ReceivedAt,
		e.ContentType, e.PayloadRaw, e.PayloadHash,
		statusCode, runTypeCode, e.ErrorCode, e.ErrorSummary,
	)
	if err != nil {
		return nil, fmt.Errorf("insert event: %w", err)
	}

	return r.GetByID(ctx, e.ID)
}

// GetByID retrieves an event by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Event, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, endpoint_id, upstream_event_id, request_id, received_at, content_type, payload_raw, payload_hash, status, run_type, error_code, error_summary
		 FROM notygo_events WHERE id = $1`, id)
	return scanEvent(row)
}

// List returns a paginated list of events and the total count.
// If statusFilter is non-empty, only events with that status are returned.
func (r *Repository) List(ctx context.Context, page, size int, statusFilter, endpointIDFilter string) ([]Event, int64, error) {
	var total int64
	var args []any

	countQuery := `SELECT COUNT(*) FROM notygo_events WHERE 1=1`
	listQuery := `SELECT id, endpoint_id, upstream_event_id, request_id, received_at, content_type, payload_raw, payload_hash, status, run_type, error_code, error_summary
		 FROM notygo_events WHERE 1=1`

	if statusFilter != "" {
		code, cerr := StatusToCode(statusFilter)
		if cerr != nil {
			// Unknown filter value matches nothing, mirroring the old
			// `status = 'garbage'` behavior of an empty result page.
			return []Event{}, 0, nil
		}
		countQuery += fmt.Sprintf(` AND status = $%d`, len(args)+1)
		listQuery += fmt.Sprintf(` AND status = $%d`, len(args)+1)
		args = append(args, code)
	}
	if endpointIDFilter != "" {
		countQuery += fmt.Sprintf(` AND endpoint_id = $%d`, len(args)+1)
		listQuery += fmt.Sprintf(` AND endpoint_id = $%d`, len(args)+1)
		args = append(args, endpointIDFilter)
	}

	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count events: %w", err)
	}
	if total == 0 {
		return []Event{}, 0, nil
	}

	listQuery += fmt.Sprintf(` ORDER BY received_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	offset := (page - 1) * size
	queryArgs := append(args, size, offset)

	rows, err := r.db.Query(ctx, listQuery, queryArgs...)
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

// MarkProcessing flips an event from received to processing (the claim guard
// for the River event_process worker). Returns false when another job
// already owns the event — the caller should no-op.
func (r *Repository) MarkProcessing(ctx context.Context, id string) (bool, error) {
	result, err := r.db.Exec(ctx,
		`UPDATE notygo_events SET status = $1, updated_at = now() WHERE id = $2 AND status = $3`,
		eventStatusCodeProcessing, id, eventStatusCodeReceived,
	)
	if err != nil {
		return false, fmt.Errorf("mark event processing: %w", err)
	}
	return result.RowsAffected() > 0, nil
}

// MarkProcessed marks an event as completed.
// Includes a status guard (AND status = processing) so a concurrently
// replayed event is not overwritten mid-run.
func (r *Repository) MarkProcessed(ctx context.Context, id string) error {
	result, err := r.db.Exec(ctx,
		`UPDATE notygo_events SET status = $1, updated_at = now() WHERE id = $2 AND status = $3`,
		eventStatusCodeCompleted, id, eventStatusCodeProcessing,
	)
	if err != nil {
		return fmt.Errorf("mark event processed: %w", err)
	}
	n := result.RowsAffected()
	if n == 0 {
		slog.Warn("mark event processed skipped (not in processing status)", "event_id", id)
	}
	return nil
}

// MarkDeduped marks an event as deduped (all matched rules were dedupe-skipped).
func (r *Repository) MarkDeduped(ctx context.Context, id string) error {
	result, err := r.db.Exec(ctx,
		`UPDATE notygo_events SET status = $1, updated_at = now() WHERE id = $2 AND status = $3`,
		eventStatusCodeDeduped, id, eventStatusCodeProcessing,
	)
	if err != nil {
		return fmt.Errorf("mark event deduped: %w", err)
	}
	n := result.RowsAffected()
	if n == 0 {
		slog.Warn("mark event deduped skipped (not in processing status)", "event_id", id)
	}
	return nil
}

// MarkFailed marks an event as failed.
func (r *Repository) MarkFailed(ctx context.Context, id string, errorCode, errorSummary string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE notygo_events SET status = $1, error_code = $2, error_summary = $3, updated_at = now() WHERE id = $4`,
		eventStatusCodeFailed, errorCode, errorSummary, id,
	)
	if err != nil {
		return fmt.Errorf("mark event failed: %w", err)
	}
	return nil
}

// SetStatus updates the event status.
func (r *Repository) SetStatus(ctx context.Context, id string, status string) error {
	code, err := StatusToCode(status)
	if err != nil {
		return fmt.Errorf("set event status: %w", err)
	}
	_, err = r.db.Exec(ctx,
		`UPDATE notygo_events SET status = $1, updated_at = now() WHERE id = $2`,
		code, id,
	)
	if err != nil {
		return fmt.Errorf("set event status: %w", err)
	}
	return nil
}

// SetForReplay resets the event for replay: sets status to received and run_type to replay.
func (r *Repository) SetForReplay(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE notygo_events SET status = $1, run_type = $2, error_code = NULL, error_summary = NULL, updated_at = now() WHERE id = $3`,
		eventStatusCodeReceived, runTypeCodeReplay, id,
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
	now := time.Now().UTC()

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

	runTypeCode, err := RunTypeToCode(run.RunType)
	if err != nil {
		return nil, fmt.Errorf("insert event_run: %w", err)
	}
	statusCode, err := RunStatusToCode(run.Status)
	if err != nil {
		return nil, fmt.Errorf("insert event_run: %w", err)
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO notygo_event_runs (id, event_id, run_type, mapping_id, mapping_revision, rule_set_snapshot, parsed_fields, validation_result, matched_rule_ids, status, started_at)
		 VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, $10, $11)`,
		run.ID, run.EventID, runTypeCode, run.MappingID, run.MappingRevision,
		string(ruleSnapshotJSON), string(parsedFieldsJSON), string(validationJSON),
		string(matchedRuleIDsJSON), statusCode, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert event_run: %w", err)
	}

	return r.GetEventRunByID(ctx, run.ID)
}

// GetEventRunByID retrieves an event run by its ID.
func (r *Repository) GetEventRunByID(ctx context.Context, id string) (*EventRun, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, event_id, run_type, mapping_id, mapping_revision, rule_set_snapshot, parsed_fields, validation_result, matched_rule_ids, status, started_at, finished_at, error_code, error_summary
		 FROM notygo_event_runs WHERE id = $1`, id)
	return scanEventRun(row)
}

// GetLatestEventRun retrieves the most recent event run for a given event.
func (r *Repository) GetLatestEventRun(ctx context.Context, eventID string) (*EventRun, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, event_id, run_type, mapping_id, mapping_revision, rule_set_snapshot, parsed_fields, validation_result, matched_rule_ids, status, started_at, finished_at, error_code, error_summary
		 FROM notygo_event_runs WHERE event_id = $1 ORDER BY started_at DESC LIMIT 1`, eventID)
	return scanEventRun(row)
}

// CompleteEventRun marks an event run as completed.
func (r *Repository) CompleteEventRun(ctx context.Context, id string, matchedRuleIDs []string) error {
	now := time.Now().UTC()
	matchedJSON, err := json.Marshal(matchedRuleIDs)
	if err != nil {
		return fmt.Errorf("marshal matched_rule_ids: %w", err)
	}

	_, err = r.db.Exec(ctx,
		`UPDATE notygo_event_runs SET status = $1, finished_at = $2, matched_rule_ids = $3::jsonb WHERE id = $4`,
		eventRunStatusCodeCompleted, now, string(matchedJSON), id,
	)
	if err != nil {
		return fmt.Errorf("complete event_run: %w", err)
	}
	return nil
}

// FailEventRun marks an event run as failed.
func (r *Repository) FailEventRun(ctx context.Context, id string, errorCode, errorSummary string) error {
	now := time.Now().UTC()

	_, err := r.db.Exec(ctx,
		`UPDATE notygo_event_runs SET status = $1, finished_at = $2, error_code = $3, error_summary = $4 WHERE id = $5`,
		eventRunStatusCodeFailed, now, errorCode, errorSummary, id,
	)
	if err != nil {
		return fmt.Errorf("fail event_run: %w", err)
	}
	return nil
}

func scanEvent(row pgx.Row) (*Event, error) {
	var e Event
	var statusCode, runTypeCode int16

	err := row.Scan(
		&e.ID, &e.EndpointID, &e.UpstreamEventID, &e.RequestID, &e.ReceivedAt,
		&e.ContentType, &e.PayloadRaw, &e.PayloadHash, &statusCode, &runTypeCode,
		&e.ErrorCode, &e.ErrorSummary,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan event: %w", err)
	}

	e.Status, err = StatusFromCode(statusCode)
	if err != nil {
		return nil, fmt.Errorf("scan event: %w", err)
	}
	e.RunType, err = RunTypeFromCode(runTypeCode)
	if err != nil {
		return nil, fmt.Errorf("scan event: %w", err)
	}

	return &e, nil
}

func scanEventValue(row pgx.Row) (Event, error) {
	e, err := scanEvent(row)
	if err != nil {
		return Event{}, err
	}
	return *e, nil
}

func scanEventFromRows(rows pgx.Rows) (*Event, error) {
	// pgx.Rows satisfies the pgx.Row interface (both expose Scan).
	return scanEvent(rows)
}

func scanEventRun(row pgx.Row) (*EventRun, error) {
	var run EventRun
	var finishedAt sql.NullTime
	var parsedFieldsJSON, validationJSON, ruleSnapshotJSON, matchedRuleIDsJSON string
	var statusCode, runTypeCode int16

	err := row.Scan(
		&run.ID, &run.EventID, &runTypeCode, &run.MappingID, &run.MappingRevision,
		&ruleSnapshotJSON, &parsedFieldsJSON, &validationJSON, &matchedRuleIDsJSON,
		&statusCode, &run.StartedAt, &finishedAt, &run.ErrorCode, &run.ErrorSummary,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan event_run: %w", err)
	}

	run.RunType, err = RunTypeFromCode(runTypeCode)
	if err != nil {
		return nil, fmt.Errorf("scan event_run: %w", err)
	}
	run.Status, err = RunStatusFromCode(statusCode)
	if err != nil {
		return nil, fmt.Errorf("scan event_run: %w", err)
	}

	if finishedAt.Valid {
		run.FinishedAt = &finishedAt.Time
	}

	if err := json.Unmarshal([]byte(parsedFieldsJSON), &run.ParsedFields); err != nil {
		run.ParsedFields = map[string]any{}
	}
	if err := json.Unmarshal([]byte(validationJSON), &run.ValidationResult); err != nil {
		run.ValidationResult = map[string]any{}
	}
	if err := json.Unmarshal([]byte(ruleSnapshotJSON), &run.RuleSetSnapshot); err != nil || run.RuleSetSnapshot == nil {
		run.RuleSetSnapshot = []RuleSnapshot{}
	}
	if err := json.Unmarshal([]byte(matchedRuleIDsJSON), &run.MatchedRuleIDs); err != nil || run.MatchedRuleIDs == nil {
		run.MatchedRuleIDs = []string{}
	}

	return &run, nil
}
