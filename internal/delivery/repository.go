package delivery

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"zpush/internal/storage"
)

// ErrNotFound is returned when a delivery is not found.
var ErrNotFound = errors.New("delivery not found")

// ErrAlreadyClaimed is returned when a delivery cannot be claimed.
var ErrAlreadyClaimed = errors.New("delivery already claimed")

func newID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// Repository provides operations for deliveries and delivery attempts.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new delivery repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new delivery and returns it.
func (r *Repository) Create(ctx context.Context, d *Delivery) (*Delivery, error) {
	id := newID()
	if d.ID == "" {
		d.ID = id
	}
	nowStr := storage.FormatNow()
	if d.NextAttemptAt.IsZero() {
		d.NextAttemptAt = time.Now().UTC()
	}

	result, err := r.db.ExecContext(ctx,
		`INSERT IGNORE INTO deliveries (id, event_id, event_run_id, rule_id, destination_id, idempotency_key, status, attempt_count, max_attempts, next_attempt_at, rendered_title, rendered_body, rendered_format, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.EventID, d.EventRunID, d.RuleID, d.DestinationID,
		d.IdempotencyKey, d.Status, d.AttemptCount, d.MaxAttempts,
		storage.FormatTime(d.NextAttemptAt),
		d.RenderedTitle, d.RenderedBody, d.RenderedFormat, nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert delivery: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		// INSERT IGNORE skipped due to duplicate idempotency_key —
		// delivery already exists from a prior processing run, return it
		existing, ferr := r.GetByIdempotencyKey(ctx, d.IdempotencyKey)
		if ferr != nil {
			return nil, fmt.Errorf("insert delivery ignored but failed to fetch existing: %w", ferr)
		}
		return existing, nil
	}

	return r.GetByID(ctx, d.ID)
}

// GetByID retrieves a delivery by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Delivery, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, event_id, event_run_id, rule_id, destination_id, idempotency_key, status, attempt_count, max_attempts, next_attempt_at, lease_until, lease_owner, rendered_title, rendered_body, rendered_format, last_error_code, last_error_summary, created_at, updated_at
		 FROM deliveries WHERE id = ?`, id)
	return scanDelivery(row)
}

// GetByIdempotencyKey retrieves a delivery by its idempotency key.
func (r *Repository) GetByIdempotencyKey(ctx context.Context, key string) (*Delivery, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, event_id, event_run_id, rule_id, destination_id, idempotency_key, status, attempt_count, max_attempts, next_attempt_at, lease_until, lease_owner, rendered_title, rendered_body, rendered_format, last_error_code, last_error_summary, created_at, updated_at
		 FROM deliveries WHERE idempotency_key = ?`, key)
	return scanDelivery(row)
}

// List returns a paginated list of deliveries and the total count.
// Filters by status, destination_id, and event_id if provided.
func (r *Repository) List(ctx context.Context, page, size int, statusFilter, destIDFilter, eventIDFilter string) ([]Delivery, int64, error) {
	var total int64
	var args []any

	countQuery := `SELECT COUNT(*) FROM deliveries WHERE 1=1`
	listQuery := `SELECT id, event_id, event_run_id, rule_id, destination_id, idempotency_key, status, attempt_count, max_attempts, next_attempt_at, lease_until, lease_owner, rendered_title, rendered_body, rendered_format, last_error_code, last_error_summary, created_at, updated_at
		 FROM deliveries WHERE 1=1`

	if statusFilter != "" {
		countQuery += ` AND status = ?`
		listQuery += ` AND status = ?`
		args = append(args, statusFilter)
	}
	if destIDFilter != "" {
		countQuery += ` AND destination_id = ?`
		listQuery += ` AND destination_id = ?`
		args = append(args, destIDFilter)
	}
	if eventIDFilter != "" {
		countQuery += ` AND event_id = ?`
		listQuery += ` AND event_id = ?`
		args = append(args, eventIDFilter)
	}

	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count deliveries: %w", err)
	}
	if total == 0 {
		return []Delivery{}, 0, nil
	}

	listQuery += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	offset := (page - 1) * size
	queryArgs := append(args, size, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list deliveries: %w", err)
	}
	defer rows.Close()

	var result []Delivery
	for rows.Next() {
		d, err := scanDeliveryFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *d)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate deliveries: %w", err)
	}

	return result, total, nil
}

// ClaimDelivery attempts to claim a delivery for sending by the given worker.
func (r *Repository) ClaimDelivery(ctx context.Context, workerID string, leaseTimeout time.Duration) (*Delivery, error) {
	leaseUntil := storage.FormatTime(time.Now().UTC().Add(leaseTimeout))
	nowExpr := storage.NowExpr()

	// Step 1: Find the oldest claimable delivery ID
	var id string
	err := r.db.QueryRowContext(ctx,
		"SELECT id FROM deliveries WHERE status IN ('pending', 'retry_wait') AND next_attempt_at <= "+nowExpr+
			" ORDER BY next_attempt_at ASC LIMIT 1",
	).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAlreadyClaimed
		}
		return nil, fmt.Errorf("find claimable delivery: %w", err)
	}

	// Step 2: Claim it by ID (atomic, status guard prevents double-claiming)
	result, err := r.db.ExecContext(ctx,
		"UPDATE deliveries SET status = ?, lease_until = ?, lease_owner = ?, attempt_count = attempt_count + 1, updated_at = ?"+
			" WHERE id = ? AND status IN ('pending', 'retry_wait')",
		DeliveryStatusProcessing, leaseUntil, workerID, storage.FormatNow(), id,
	)
	if err != nil {
		return nil, fmt.Errorf("claim delivery: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return nil, ErrAlreadyClaimed // someone else claimed it between step 1 and 2
	}

	// Step 3: Fetch by ID — guaranteed to be the right one
	return r.GetByID(ctx, id)
}

// UpdateStatus updates the delivery status and clears lease fields.
func (r *Repository) UpdateStatus(ctx context.Context, id string, status string) error {
	nowExpr := storage.NowExpr()
	_, err := r.db.ExecContext(ctx,
		"UPDATE deliveries SET status = ?, lease_until = NULL, lease_owner = NULL, updated_at = "+nowExpr+" WHERE id = ?",
		status, id,
	)
	if err != nil {
		return fmt.Errorf("update delivery status: %w", err)
	}
	return nil
}

// ResetToPending resets a delivery back to pending status (e.g., when circuit breaker is open).
func (r *Repository) ResetToPending(ctx context.Context, id string) error {
	nowExpr := storage.NowExpr()
	_, err := r.db.ExecContext(ctx,
		"UPDATE deliveries SET status = ?, attempt_count = attempt_count - 1, lease_until = NULL, lease_owner = NULL, updated_at = "+nowExpr+" WHERE id = ?",
		DeliveryStatusPending, id,
	)
	if err != nil {
		return fmt.Errorf("reset delivery to pending: %w", err)
	}
	return nil
}

// UpdateNextAttempt sets the next_attempt_at for a delivery.
func (r *Repository) UpdateNextAttempt(ctx context.Context, id string, nextAttemptAt time.Time) error {
	nowExpr := storage.NowExpr()
	_, err := r.db.ExecContext(ctx,
		"UPDATE deliveries SET next_attempt_at = ?, updated_at = "+nowExpr+" WHERE id = ?",
		storage.FormatTime(nextAttemptAt), id,
	)
	if err != nil {
		return fmt.Errorf("update next attempt: %w", err)
	}
	return nil
}

// UpdateError sets the error fields on a delivery.
func (r *Repository) UpdateError(ctx context.Context, id string, errorCode, errorSummary string) error {
	nowExpr := storage.NowExpr()
	_, err := r.db.ExecContext(ctx,
		"UPDATE deliveries SET last_error_code = ?, last_error_summary = ?, updated_at = "+nowExpr+" WHERE id = ?",
		errorCode, errorSummary, id,
	)
	if err != nil {
		return fmt.Errorf("update delivery error: %w", err)
	}
	return nil
}

// MarkDelivered marks a delivery as successfully delivered.
func (r *Repository) MarkDelivered(ctx context.Context, id string) error {
	nowExpr := storage.NowExpr()
	_, err := r.db.ExecContext(ctx,
		"UPDATE deliveries SET status = ?, lease_until = NULL, lease_owner = NULL, updated_at = "+nowExpr+" WHERE id = ?",
		DeliveryStatusDelivered, id,
	)
	if err != nil {
		return fmt.Errorf("mark delivery delivered: %w", err)
	}
	return nil
}

// RecordDeliveryAttempt inserts a delivery attempt record.
func (r *Repository) RecordDeliveryAttempt(ctx context.Context, attempt *DeliveryAttempt) (*DeliveryAttempt, error) {
	if attempt.ID == "" {
		attempt.ID = newID()
	}
	if attempt.StartedAt.IsZero() {
		attempt.StartedAt = time.Now().UTC()
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO delivery_attempts (id, delivery_id, attempt_no, started_at, finished_at, request_summary, response_status, response_summary, error_code, error_summary, duration_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		attempt.ID, attempt.DeliveryID, attempt.AttemptNo,
		storage.FormatTime(attempt.StartedAt),
		nullTime(attempt.FinishedAt), attempt.RequestSummary,
		attempt.ResponseStatus, attempt.ResponseSummary,
		attempt.ErrorCode, attempt.ErrorSummary, attempt.DurationMs,
	)
	if err != nil {
		return nil, fmt.Errorf("insert delivery_attempt: %w", err)
	}

	return attempt, nil
}

// ListDeliveryAttempts returns all attempts for a delivery.
func (r *Repository) ListDeliveryAttempts(ctx context.Context, deliveryID string) ([]DeliveryAttempt, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, delivery_id, attempt_no, started_at, finished_at, request_summary, response_status, response_summary, error_code, error_summary, duration_ms
		 FROM delivery_attempts WHERE delivery_id = ? ORDER BY attempt_no ASC`, deliveryID)
	if err != nil {
		return nil, fmt.Errorf("list delivery_attempts: %w", err)
	}
	defer rows.Close()

	var result []DeliveryAttempt
	for rows.Next() {
		var a DeliveryAttempt
		var startedAt string
		var finishedAt sql.NullString

		err := rows.Scan(
			&a.ID, &a.DeliveryID, &a.AttemptNo, &startedAt, &finishedAt,
			&a.RequestSummary, &a.ResponseStatus, &a.ResponseSummary,
			&a.ErrorCode, &a.ErrorSummary, &a.DurationMs,
		)
		if err != nil {
			return nil, fmt.Errorf("scan delivery_attempt: %w", err)
		}

		a.StartedAt, _ = storage.ParseTime(startedAt)
		if finishedAt.Valid {
			t, _ := storage.ParseTime(finishedAt.String)
			a.FinishedAt = &t
		}
		result = append(result, a)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate delivery_attempts: %w", err)
	}

	return result, nil
}

// BulkResetPending resets multiple deliveries back to pending status based on filters.
func (r *Repository) BulkResetPending(ctx context.Context, statusFilter, destIDFilter string) (int64, error) {
	nowExpr := storage.NowExpr()
	query := "UPDATE deliveries SET status = 'pending', last_error_code = NULL, last_error_summary = NULL, next_attempt_at = " + nowExpr + ", updated_at = " + nowExpr + " WHERE 1=1"
	var args []any

	if statusFilter != "" {
		query += ` AND status = ?`
		args = append(args, statusFilter)
	}
	if destIDFilter != "" {
		query += ` AND destination_id = ?`
		args = append(args, destIDFilter)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("bulk reset deliveries: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return storage.FormatTime(*t)
}

func scanDelivery(row *sql.Row) (*Delivery, error) {
	var d Delivery
	var nextAttemptAt string
	var leaseUntil sql.NullString
	var leaseOwner sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(
		&d.ID, &d.EventID, &d.EventRunID, &d.RuleID, &d.DestinationID,
		&d.IdempotencyKey, &d.Status, &d.AttemptCount, &d.MaxAttempts,
		&nextAttemptAt, &leaseUntil, &leaseOwner,
		&d.RenderedTitle, &d.RenderedBody, &d.RenderedFormat,
		&d.LastErrorCode, &d.LastErrorSummary,
		&createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan delivery: %w", err)
	}

	d.NextAttemptAt, _ = storage.ParseTime(nextAttemptAt)
	if leaseUntil.Valid {
		t, _ := storage.ParseTime(leaseUntil.String)
		d.LeaseUntil = &t
	}
	if leaseOwner.Valid {
		d.LeaseOwner = &leaseOwner.String
	}
	d.CreatedAt, _ = storage.ParseTime(createdAt)
	d.UpdatedAt, _ = storage.ParseTime(updatedAt)

	return &d, nil
}

func scanDeliveryValue(row *sql.Row) (Delivery, error) {
	d, err := scanDelivery(row)
	if err != nil {
		return Delivery{}, err
	}
	return *d, nil
}

func scanDeliveryFromRows(rows *sql.Rows) (*Delivery, error) {
	var d Delivery
	var nextAttemptAt string
	var leaseUntil sql.NullString
	var leaseOwner sql.NullString
	var createdAt, updatedAt string

	err := rows.Scan(
		&d.ID, &d.EventID, &d.EventRunID, &d.RuleID, &d.DestinationID,
		&d.IdempotencyKey, &d.Status, &d.AttemptCount, &d.MaxAttempts,
		&nextAttemptAt, &leaseUntil, &leaseOwner,
		&d.RenderedTitle, &d.RenderedBody, &d.RenderedFormat,
		&d.LastErrorCode, &d.LastErrorSummary,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan delivery: %w", err)
	}

	d.NextAttemptAt, _ = storage.ParseTime(nextAttemptAt)
	if leaseUntil.Valid {
		t, _ := storage.ParseTime(leaseUntil.String)
		d.LeaseUntil = &t
	}
	if leaseOwner.Valid {
		d.LeaseOwner = &leaseOwner.String
	}
	d.CreatedAt, _ = storage.ParseTime(createdAt)
	d.UpdatedAt, _ = storage.ParseTime(updatedAt)

	return &d, nil
}

// DedupeCheckResult represents the result of a deduplication check.
type DedupeCheckResult struct {
	IsDuplicate bool // true = key already exists within the window (duplicate)
}

// CheckAndInsertDedupeKey checks whether a dedupe key already exists within its
// time window for the given rule. If not found, it atomically inserts a new record
// and returns IsDuplicate=false. If found (still within window), it returns
// IsDuplicate=true. Expired records are ignored.
//
// Two queries without transaction: under MySQL autocommit, each statement is
// individually atomic. A rare race where two workers both see "not found" and
// both INSERT is acceptable — it just means an extra dedupe record; the second
// delivery is still created, which is safer than accidentally blocking it.
func (r *Repository) CheckAndInsertDedupeKey(ctx context.Context, ruleID, dedupeHash string, windowSeconds int, now time.Time) (*DedupeCheckResult, error) {
	nowStr := storage.FormatTime(now)

	// Step 1: Check if an unexpired dedupe key exists
	var existingID string
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM dedupe_keys WHERE rule_id = ? AND dedupe_hash = ? AND expires_at > ? LIMIT 1`,
		ruleID, dedupeHash, nowStr,
	).Scan(&existingID)
	if err == nil {
		// Key found within window — this is a duplicate
		return &DedupeCheckResult{IsDuplicate: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("check dedupe key: %w", err)
	}

	// Step 2: No unexpired key found — insert a new record
	id := newID()
	expiresAt := now.Add(time.Duration(windowSeconds) * time.Second)
	expiresAtStr := storage.FormatTime(expiresAt)

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO dedupe_keys (id, rule_id, dedupe_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, ruleID, dedupeHash, expiresAtStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert dedupe key: %w", err)
	}

	return &DedupeCheckResult{IsDuplicate: false}, nil
}

// CleanExpiredDedupeKeys removes expired dedupe key records.
// Called periodically from lease recovery to prevent unbounded table growth.
func (r *Repository) CleanExpiredDedupeKeys(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM dedupe_keys WHERE expires_at < NOW() LIMIT 1000`,
	)
	if err != nil {
		return 0, fmt.Errorf("clean expired dedupe keys: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// marshalJSON helper
func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
