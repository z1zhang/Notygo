package audit

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

// ErrNotFound is returned when an audit log is not found.
var ErrNotFound = errors.New("audit log not found")

func newID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// Repository provides data access for audit logs.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new audit repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Insert creates a new audit log entry.
func (r *Repository) Insert(ctx context.Context, a *AuditLog) error {
	if a.ID == "" {
		a.ID = newID()
	}

	var snapshotBefore *string
	if a.SnapshotBefore != nil {
		// If the snapshot is already a JSON string, store it directly.
		// Otherwise marshal it.
		snapshotBefore = a.SnapshotBefore
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO notygo_audit_logs (id, operator, action, target_type, target_id, snapshot_before, summary, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)`,
		a.ID, a.Operator, a.Action, a.TargetType, a.TargetID,
		snapshotBefore, a.Summary, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert audit_log: %w", err)
	}
	return nil
}

// List returns a paginated list of audit logs with optional filtering, and the total count.
func (r *Repository) List(ctx context.Context, page, size int, action, targetType, startTime, endTime string) ([]AuditLog, int64, error) {
	where := "WHERE 1=1"
	var args []any
	if action != "" {
		where += fmt.Sprintf(" AND action = $%d", len(args)+1)
		args = append(args, action)
	}
	if targetType != "" {
		where += fmt.Sprintf(" AND target_type = $%d", len(args)+1)
		args = append(args, targetType)
	}
	if startTime != "" {
		where += fmt.Sprintf(" AND created_at >= $%d::timestamptz", len(args)+1)
		args = append(args, startTime)
	}
	if endTime != "" {
		where += fmt.Sprintf(" AND created_at <= $%d::timestamptz", len(args)+1)
		args = append(args, endTime+"T23:59:59.999Z")
	}

	var total int64
	countSQL := "SELECT COUNT(*) FROM notygo_audit_logs " + where
	err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count audit_logs: %w", err)
	}
	if total == 0 {
		return []AuditLog{}, 0, nil
	}

	offset := (page - 1) * size
	listSQL := `SELECT id, operator, action, target_type, target_id, snapshot_before, summary, created_at
		FROM notygo_audit_logs ` + where + fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	listArgs := append(args, size, offset)

	rows, err := r.db.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit_logs: %w", err)
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		a, err := scanAuditLogFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, *a)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate audit_logs: %w", err)
	}

	return logs, total, nil
}

func scanAuditLogFromRows(rows pgx.Rows) (*AuditLog, error) {
	var a AuditLog
	err := rows.Scan(
		&a.ID, &a.Operator, &a.Action, &a.TargetType, &a.TargetID,
		&a.SnapshotBefore, &a.Summary, &a.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan audit_log: %w", err)
	}
	return &a, nil
}

// marshalJSON is a helper to marshal any value to JSON string.
func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
