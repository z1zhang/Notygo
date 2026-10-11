package audit

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

// ErrNotFound is returned when an audit log is not found.
var ErrNotFound = errors.New("audit log not found")

func newID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// Repository provides data access for audit logs.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new audit repository.
func NewRepository(db *sql.DB) *Repository {
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

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_logs (id, operator, action, target_type, target_id, snapshot_before, summary, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Operator, a.Action, a.TargetType, a.TargetID,
		snapshotBefore, a.Summary, storage.FormatNow(),
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
		where += " AND action = ?"
		args = append(args, action)
	}
	if targetType != "" {
		where += " AND target_type = ?"
		args = append(args, targetType)
	}
	if startTime != "" {
		where += " AND created_at >= ?"
		args = append(args, startTime)
	}
	if endTime != "" {
		where += " AND created_at <= ?"
		args = append(args, endTime+"T23:59:59.999Z")
	}

	var total int64
	countSQL := "SELECT COUNT(*) FROM audit_logs " + where
	err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count audit_logs: %w", err)
	}
	if total == 0 {
		return []AuditLog{}, 0, nil
	}

	offset := (page - 1) * size
	listSQL := `SELECT id, operator, action, target_type, target_id, snapshot_before, summary, created_at
		FROM audit_logs ` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	listArgs := append(args, size, offset)

	rows, err := r.db.QueryContext(ctx, listSQL, listArgs...)
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

func scanAuditLogFromRows(rows *sql.Rows) (*AuditLog, error) {
	var a AuditLog
	var createdAt string
	err := rows.Scan(
		&a.ID, &a.Operator, &a.Action, &a.TargetType, &a.TargetID,
		&a.SnapshotBefore, &a.Summary, &createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan audit_log: %w", err)
	}
	a.CreatedAt, _ = storage.ParseTime(createdAt)
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
