package delivery

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
	"notygo/internal/storage"
)

// ErrNotFound is returned when a delivery is not found.
var ErrNotFound = errors.New("delivery not found")

func newID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// Repository provides operations for deliveries and delivery attempts.
type Repository struct {
	db storage.DBTX
}

// NewRepository creates a new delivery repository.
func NewRepository(db storage.DBTX) *Repository {
	return &Repository{db: db}
}

// WithTx returns a copy of the repository bound to the given transaction, so
// callers can run the same statements atomically (delivery row writes and
// the River job insert share one transaction).
func (r *Repository) WithTx(db storage.DBTX) *Repository {
	return &Repository{db: db}
}

// Create inserts a new delivery and returns it.
func (r *Repository) Create(ctx context.Context, d *Delivery) (*Delivery, error) {
	id := newID()
	if d.ID == "" {
		d.ID = id
	}
	now := time.Now().UTC()
	if d.NextAttemptAt.IsZero() {
		d.NextAttemptAt = now
	}
	statusCode, err := statusToCode(d.Status)
	if err != nil {
		return nil, fmt.Errorf("insert delivery: %w", err)
	}
	var renderedFormatCode *int16
	if d.RenderedFormat != nil {
		var c int16
		c, err = renderedFormatToCode(*d.RenderedFormat)
		if err != nil {
			return nil, fmt.Errorf("insert delivery: %w", err)
		}
		renderedFormatCode = &c
	}

	result, err := r.db.Exec(ctx,
		`INSERT INTO notygo_delivery (id, event_id, event_run_id, rule_id, channel_id, idempotency_key, status, attempt_count, max_attempts, next_attempt_at, rendered_title, rendered_body, rendered_format, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		 ON CONFLICT DO NOTHING`,
		d.ID, d.EventID, d.EventRunID, d.RuleID, d.ChannelID,
		d.IdempotencyKey, statusCode, d.AttemptCount, d.MaxAttempts,
		d.NextAttemptAt,
		d.RenderedTitle, d.RenderedBody, renderedFormatCode, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert delivery: %w", err)
	}

	n := result.RowsAffected()
	if n == 0 {
		// ON CONFLICT DO NOTHING skipped due to duplicate idempotency_key —
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
	row := r.db.QueryRow(ctx,
		`SELECT id, event_id, event_run_id, rule_id, channel_id, idempotency_key, status, attempt_count, max_attempts, next_attempt_at, rendered_title, rendered_body, rendered_format, last_error_code, last_error_summary, created_at, updated_at
		 FROM notygo_delivery WHERE id = $1`, id)
	return scanDelivery(row)
}

// GetByIdempotencyKey retrieves a delivery by its idempotency key.
func (r *Repository) GetByIdempotencyKey(ctx context.Context, key string) (*Delivery, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, event_id, event_run_id, rule_id, channel_id, idempotency_key, status, attempt_count, max_attempts, next_attempt_at, rendered_title, rendered_body, rendered_format, last_error_code, last_error_summary, created_at, updated_at
		 FROM notygo_delivery WHERE idempotency_key = $1`, key)
	return scanDelivery(row)
}

// List returns a paginated list of deliveries and the total count.
// Filters by status, channel_id, and event_id if provided.
func (r *Repository) List(ctx context.Context, page, size int, statusFilter, destIDFilter, eventIDFilter string) ([]Delivery, int64, error) {
	var total int64
	var args []any

	countQuery := `SELECT COUNT(*) FROM notygo_delivery WHERE 1=1`
	listQuery := `SELECT id, event_id, event_run_id, rule_id, channel_id, idempotency_key, status, attempt_count, max_attempts, next_attempt_at, rendered_title, rendered_body, rendered_format, last_error_code, last_error_summary, created_at, updated_at
		 FROM notygo_delivery WHERE 1=1`

	if statusFilter != "" {
		code, cerr := statusToCode(statusFilter)
		if cerr != nil {
			// Unknown filter value matches nothing, mirroring the old
			// `status = 'garbage'` behavior of an empty result page.
			return []Delivery{}, 0, nil
		}
		countQuery += fmt.Sprintf(` AND status = $%d`, len(args)+1)
		listQuery += fmt.Sprintf(` AND status = $%d`, len(args)+1)
		args = append(args, code)
	}
	if destIDFilter != "" {
		countQuery += fmt.Sprintf(` AND channel_id = $%d`, len(args)+1)
		listQuery += fmt.Sprintf(` AND channel_id = $%d`, len(args)+1)
		args = append(args, destIDFilter)
	}
	if eventIDFilter != "" {
		countQuery += fmt.Sprintf(` AND event_id = $%d`, len(args)+1)
		listQuery += fmt.Sprintf(` AND event_id = $%d`, len(args)+1)
		args = append(args, eventIDFilter)
	}

	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count deliveries: %w", err)
	}
	if total == 0 {
		return []Delivery{}, 0, nil
	}

	listQuery += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	offset := (page - 1) * size
	queryArgs := append(args, size, offset)

	rows, err := r.db.Query(ctx, listQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list deliveries: %w", err)
	}
	defer rows.Close()

	var result []Delivery
	for rows.Next() {
		d, err := scanDelivery(rows)
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

// MarkAttemptInProgress is the delivery worker's claim guard: it flips a
// pending/retry_wait delivery to processing and stamps the attempt number
// (the River job's attempt count). Returns false when the delivery is in a
// terminal state or another job is already on it — the caller should no-op.
func (r *Repository) MarkAttemptInProgress(ctx context.Context, id string, attemptNo int) (bool, error) {
	nowExpr := storage.NowExpr()
	result, err := r.db.Exec(ctx,
		"UPDATE notygo_delivery SET status = $1, attempt_count = $2, updated_at = "+nowExpr+
			" WHERE id = $3 AND status IN (0, 1)", // 0=pending 1=retry_wait
		deliveryStatusCodeProcessing, attemptNo, id,
	)
	if err != nil {
		return false, fmt.Errorf("mark delivery in progress: %w", err)
	}
	return result.RowsAffected() > 0, nil
}

// MarkDelivered marks a delivery as successfully delivered.
func (r *Repository) MarkDelivered(ctx context.Context, id string) error {
	nowExpr := storage.NowExpr()
	_, err := r.db.Exec(ctx,
		"UPDATE notygo_delivery SET status = $1, updated_at = "+nowExpr+" WHERE id = $2",
		deliveryStatusCodeDelivered, id,
	)
	if err != nil {
		return fmt.Errorf("mark delivery delivered: %w", err)
	}
	return nil
}

// MarkRetryWait records a transient failure: the delivery waits for the
// queue's next scheduled attempt. nextAttemptAt is the display-only
// prediction computed from the same backoff formula the queue uses.
func (r *Repository) MarkRetryWait(ctx context.Context, id string, nextAttemptAt time.Time, errorCode, errorSummary string) error {
	nowExpr := storage.NowExpr()
	_, err := r.db.Exec(ctx,
		"UPDATE notygo_delivery SET status = $1, next_attempt_at = $2, last_error_code = $3, last_error_summary = $4, updated_at = "+nowExpr+" WHERE id = $5",
		deliveryStatusCodeRetryWait, nextAttemptAt, errorCode, errorSummary, id,
	)
	if err != nil {
		return fmt.Errorf("mark delivery retry_wait: %w", err)
	}
	return nil
}

// MarkFailed marks a delivery as permanently failed (attempt budget
// exhausted or a non-retryable error).
func (r *Repository) MarkFailed(ctx context.Context, id string, errorCode, errorSummary string) error {
	nowExpr := storage.NowExpr()
	_, err := r.db.Exec(ctx,
		"UPDATE notygo_delivery SET status = $1, last_error_code = $2, last_error_summary = $3, updated_at = "+nowExpr+" WHERE id = $4",
		deliveryStatusCodeFailed, errorCode, errorSummary, id,
	)
	if err != nil {
		return fmt.Errorf("mark delivery failed: %w", err)
	}
	return nil
}

// UpdateError sets the error fields on a delivery without changing status.
func (r *Repository) UpdateError(ctx context.Context, id string, errorCode, errorSummary string) error {
	nowExpr := storage.NowExpr()
	_, err := r.db.Exec(ctx,
		"UPDATE notygo_delivery SET last_error_code = $1, last_error_summary = $2, updated_at = "+nowExpr+" WHERE id = $3",
		errorCode, errorSummary, id,
	)
	if err != nil {
		return fmt.Errorf("update delivery error: %w", err)
	}
	return nil
}

// ResetForRetry resets a failed/retry_wait delivery for a fresh manual
// retry: pending status, cleared errors, zero attempt count. The caller
// re-enqueues the queue job in the same transaction.
func (r *Repository) ResetForRetry(ctx context.Context, id string) (bool, error) {
	nowExpr := storage.NowExpr()
	result, err := r.db.Exec(ctx,
		"UPDATE notygo_delivery SET status = $1, attempt_count = 0, last_error_code = NULL, last_error_summary = NULL, next_attempt_at = "+nowExpr+", updated_at = "+nowExpr+
			" WHERE id = $2 AND status IN (4, 1)", // 4=failed 1=retry_wait
		deliveryStatusCodePending, id,
	)
	if err != nil {
		return false, fmt.Errorf("reset delivery for retry: %w", err)
	}
	return result.RowsAffected() > 0, nil
}

// RecordDeliveryAttempt inserts a delivery attempt record.
func (r *Repository) RecordDeliveryAttempt(ctx context.Context, attempt *DeliveryAttempt) (*DeliveryAttempt, error) {
	if attempt.ID == "" {
		attempt.ID = newID()
	}
	if attempt.StartedAt.IsZero() {
		attempt.StartedAt = time.Now().UTC()
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO notygo_delivery_attempt (id, delivery_id, attempt_no, started_at, finished_at, request_summary, response_status, response_summary, error_code, error_summary, duration_ms)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		attempt.ID, attempt.DeliveryID, attempt.AttemptNo,
		attempt.StartedAt,
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
	rows, err := r.db.Query(ctx,
		`SELECT id, delivery_id, attempt_no, started_at, finished_at, request_summary, response_status, response_summary, error_code, error_summary, duration_ms
		 FROM notygo_delivery_attempt WHERE delivery_id = $1 ORDER BY attempt_no ASC`, deliveryID)
	if err != nil {
		return nil, fmt.Errorf("list delivery_attempts: %w", err)
	}
	defer rows.Close()

	var result []DeliveryAttempt
	for rows.Next() {
		var a DeliveryAttempt
		var finishedAt sql.NullTime

		err := rows.Scan(
			&a.ID, &a.DeliveryID, &a.AttemptNo, &a.StartedAt, &finishedAt,
			&a.RequestSummary, &a.ResponseStatus, &a.ResponseSummary,
			&a.ErrorCode, &a.ErrorSummary, &a.DurationMs,
		)
		if err != nil {
			return nil, fmt.Errorf("scan delivery_attempt: %w", err)
		}

		if finishedAt.Valid {
			a.FinishedAt = &finishedAt.Time
		}
		result = append(result, a)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate delivery_attempts: %w", err)
	}

	return result, nil
}

// ResetItem identifies one delivery reset for retry, with the retry budget
// the caller needs to enqueue its queue job.
type ResetItem struct {
	ID          string
	MaxAttempts int
}

// BulkResetForRetry resets multiple failed/retry_wait deliveries for retry
// and returns the reset items, so the caller can re-enqueue a queue job per
// delivery in the same transaction.
func (r *Repository) BulkResetForRetry(ctx context.Context, statusFilter, destIDFilter string) ([]ResetItem, error) {
	nowExpr := storage.NowExpr()
	// Retry is limited to failed(4)/retry_wait(1) — other statuses are never reset
	query := "UPDATE notygo_delivery SET status = 0, attempt_count = 0, last_error_code = NULL, last_error_summary = NULL, next_attempt_at = " + nowExpr + ", updated_at = " + nowExpr +
		" WHERE status IN (4, 1)"
	var args []any

	if statusFilter != "" {
		// Translate the status name to its numeric code — the status column
		// is a smallint, so comparing against the raw string is a type error.
		code, cerr := statusToCode(statusFilter)
		if cerr != nil {
			return nil, fmt.Errorf("bulk reset deliveries: invalid status filter %q: %w", statusFilter, cerr)
		}
		query += fmt.Sprintf(` AND status = $%d`, len(args)+1)
		args = append(args, code)
	}
	if destIDFilter != "" {
		query += fmt.Sprintf(` AND channel_id = $%d`, len(args)+1)
		args = append(args, destIDFilter)
	}
	query += " RETURNING id, max_attempts"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("bulk reset deliveries: %w", err)
	}
	defer rows.Close()

	var items []ResetItem
	for rows.Next() {
		var it ResetItem
		if err := rows.Scan(&it.ID, &it.MaxAttempts); err != nil {
			return nil, fmt.Errorf("scan bulk reset delivery: %w", err)
		}
		if it.MaxAttempts < 1 {
			it.MaxAttempts = 1
		}
		items = append(items, it)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bulk reset deliveries: %w", err)
	}

	return items, nil
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func scanDelivery(row pgx.Row) (*Delivery, error) {
	var d Delivery
	var statusCode int16
	var renderedFormat sql.NullInt16

	err := row.Scan(
		&d.ID, &d.EventID, &d.EventRunID, &d.RuleID, &d.ChannelID,
		&d.IdempotencyKey, &statusCode, &d.AttemptCount, &d.MaxAttempts,
		&d.NextAttemptAt,
		&d.RenderedTitle, &d.RenderedBody, &renderedFormat,
		&d.LastErrorCode, &d.LastErrorSummary,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan delivery: %w", err)
	}

	d.Status, err = statusFromCode(statusCode)
	if err != nil {
		return nil, fmt.Errorf("scan delivery: %w", err)
	}
	if renderedFormat.Valid {
		format, ferr := renderedFormatFromCode(renderedFormat.Int16)
		if ferr != nil {
			return nil, fmt.Errorf("scan delivery: %w", ferr)
		}
		d.RenderedFormat = &format
	}

	return &d, nil
}

// DedupeCheckResult represents the result of a deduplication check.
type DedupeCheckResult struct {
	IsDuplicate bool // true = key already exists within the window (duplicate)
}

// CheckAndInsertDedupeKey checks whether a dedupe key already exists within its
// time window for the given rule, recording it otherwise — one INSERT ...
// ON CONFLICT statement, so the old two-step race (two workers both seeing
// "not found" and both sending the notification) is arbitrated by the unique
// index uq_dedupe_keys_rule_hash instead of an application-level gap.
//
// Window semantics, identical to the previous two-step version:
//   - duplicate = a record with expires_at > now exists. The DO UPDATE arm
//     carries a WHERE that only lets it fire on an expired row, so an
//     unexpired hit performs no write at all — the fixed window (from the
//     first hit) is structural, not reconstructed from timestamps;
//   - first hit (no record, or the record expired) → the window is armed
//     (or re-armed) for windowSeconds from now.
//
// Discriminator: a row comes back only on the insert and re-arm paths;
// a suppressed (unexpired) hit returns zero rows, which pgx surfaces as
// pgx.ErrNoRows. Under READ COMMITTED, a concurrent insert into the same
// key makes the conflicting statement wait for it, then re-evaluate the
// WHERE against the committed row — so exactly one caller ever gets first
// hit, even when both race in the same instant.
func (r *Repository) CheckAndInsertDedupeKey(ctx context.Context, ruleID, dedupeHash string, windowSeconds int, now time.Time) (*DedupeCheckResult, error) {
	expiresAt := now.Add(time.Duration(windowSeconds) * time.Second)

	var reArmed bool
	err := r.db.QueryRow(ctx,
		`INSERT INTO notygo_dedupe_key (id, rule_id, dedupe_hash, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (rule_id, dedupe_hash)
		 DO UPDATE SET expires_at = EXCLUDED.expires_at,
		               created_at = EXCLUDED.created_at
		 WHERE notygo_dedupe_key.expires_at <= $6  -- re-arm only once the old window is gone
		 RETURNING TRUE`,
		newID(), ruleID, dedupeHash, expiresAt, now, now,
	).Scan(&reArmed)
	if errors.Is(err, pgx.ErrNoRows) {
		return &DedupeCheckResult{IsDuplicate: true}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("upsert dedupe key: %w", err)
	}

	return &DedupeCheckResult{IsDuplicate: false}, nil
}

// CleanExpiredDedupeKeys removes expired dedupe key records.
// Called periodically by the queue's dedupe-cleanup job to prevent
// unbounded table growth.
// PostgreSQL has no DELETE ... LIMIT; the bounded subquery keeps each
// cleanup pass cheap on large tables.
func (r *Repository) CleanExpiredDedupeKeys(ctx context.Context) (int64, error) {
	result, err := r.db.Exec(ctx,
		`DELETE FROM notygo_dedupe_key WHERE id IN (
			SELECT id FROM notygo_dedupe_key WHERE expires_at < now() ORDER BY expires_at LIMIT 1000
		)`,
	)
	if err != nil {
		return 0, fmt.Errorf("clean expired dedupe keys: %w", err)
	}
	n := result.RowsAffected()
	return n, nil
}
