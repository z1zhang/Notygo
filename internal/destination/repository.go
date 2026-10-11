package destination

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"zPush/internal/storage"
)

// ErrNotFound is returned when a destination is not found.
var ErrNotFound = errors.New("destination not found")

func newID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i != 0
}

// Repository provides CRUD operations for destinations.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new destination repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new destination and returns it.
func (r *Repository) Create(ctx context.Context, req CreateDestinationRequest) (*Destination, error) {
	id := newID()
	nowStr := storage.FormatNow()

	name := ""
	if req.Name != nil {
		name = *req.Name
	}
	destType := DestinationType("http_webhook")
	if req.Type != nil {
		destType = *req.Type
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	var cbJSON *string
	if req.CircuitBreaker != nil {
		b, err := json.Marshal(req.CircuitBreaker)
		if err != nil {
			return nil, fmt.Errorf("marshal circuit_breaker: %w", err)
		}
		s := string(b)
		cbJSON = &s
	}
	timeoutMs := 10000
	if req.TimeoutMs != nil {
		timeoutMs = *req.TimeoutMs
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO destinations (id, name, type, enabled, config, secret_ref, timeout_ms, rate_limit, circuit_breaker, revision, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		id, name, string(destType), boolToInt(enabled),
		string(configJSON), req.SecretRef, timeoutMs, req.RateLimit,
		cbJSON, nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert destination: %w", err)
	}

	return r.GetByID(ctx, id)
}

// GetByID retrieves a destination by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Destination, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, type, enabled, config, secret_ref, timeout_ms, rate_limit, circuit_breaker, revision, created_at, updated_at
		 FROM destinations WHERE id = ?`, id)
	return scanDestination(row)
}

// List returns a paginated list of destinations and the total count.
func (r *Repository) List(ctx context.Context, page, size int) ([]Destination, int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM destinations`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count destinations: %w", err)
	}
	if total == 0 {
		return []Destination{}, 0, nil
	}

	offset := (page - 1) * size
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, type, enabled, config, secret_ref, timeout_ms, rate_limit, circuit_breaker, revision, created_at, updated_at
		 FROM destinations ORDER BY created_at DESC LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list destinations: %w", err)
	}
	defer rows.Close()

	var result []Destination
	for rows.Next() {
		d, err := scanDestinationFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *d)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate destinations: %w", err)
	}

	return result, total, nil
}

// Update modifies an existing destination, incrementing revision, and returns the updated version.
// Only non-nil fields in the request are updated.
func (r *Repository) Update(ctx context.Context, id string, req UpdateDestinationRequest) (*Destination, error) {
	existing, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	name := existing.Name
	if req.Name != nil {
		name = *req.Name
	}
	destType := existing.Type
	if req.Type != nil {
		destType = *req.Type
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	config := existing.Config
	if req.Config != nil {
		config = req.Config
	}
	secretRef := existing.SecretRef
	if req.SecretRef != nil {
		secretRef = req.SecretRef
	}
	timeoutMs := existing.TimeoutMs
	if req.TimeoutMs != nil {
		timeoutMs = *req.TimeoutMs
	}
	rateLimit := existing.RateLimit
	if req.RateLimit != nil {
		rateLimit = req.RateLimit
	}
	circuitBreaker := existing.CircuitBreaker
	if req.CircuitBreaker != nil {
		circuitBreaker = req.CircuitBreaker
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	var cbJSON *string
	if circuitBreaker != nil {
		b, err := json.Marshal(circuitBreaker)
		if err != nil {
			return nil, fmt.Errorf("marshal circuit_breaker: %w", err)
		}
		s := string(b)
		cbJSON = &s
	}

	newRevision := existing.Revision + 1
	nowStr := storage.FormatNow()
	_, err = r.db.ExecContext(ctx,
		`UPDATE destinations SET name=?, type=?, enabled=?, config=?, secret_ref=?, timeout_ms=?, rate_limit=?, circuit_breaker=?, revision=?, updated_at=?
		 WHERE id=?`,
		name, string(destType), boolToInt(enabled),
		string(configJSON), secretRef, timeoutMs, rateLimit,
		cbJSON, newRevision, nowStr, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update destination: %w", err)
	}

	return r.GetByID(ctx, id)
}

// Delete removes a destination by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM destinations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete destination: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanDestination(row *sql.Row) (*Destination, error) {
	var d Destination
	var enabled int
	var configJSON string
	var cbJSON sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(
		&d.ID, &d.Name, &d.Type, &enabled, &configJSON, &d.SecretRef,
		&d.TimeoutMs, &d.RateLimit, &cbJSON, &d.Revision,
		&createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan destination: %w", err)
	}

	d.Enabled = intToBool(enabled)
	if err := json.Unmarshal([]byte(configJSON), &d.Config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if cbJSON.Valid && cbJSON.String != "" {
		if err := json.Unmarshal([]byte(cbJSON.String), &d.CircuitBreaker); err != nil {
			return nil, fmt.Errorf("unmarshal circuit_breaker: %w", err)
		}
	}

	d.CreatedAt, _ = storage.ParseTime(createdAt)
	d.UpdatedAt, _ = storage.ParseTime(updatedAt)
	return &d, nil
}

func scanDestinationFromRows(rows *sql.Rows) (*Destination, error) {
	var d Destination
	var enabled int
	var configJSON string
	var cbJSON sql.NullString
	var createdAt, updatedAt string

	err := rows.Scan(
		&d.ID, &d.Name, &d.Type, &enabled, &configJSON, &d.SecretRef,
		&d.TimeoutMs, &d.RateLimit, &cbJSON, &d.Revision,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan destination: %w", err)
	}

	d.Enabled = intToBool(enabled)
	if err := json.Unmarshal([]byte(configJSON), &d.Config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if cbJSON.Valid && cbJSON.String != "" {
		if err := json.Unmarshal([]byte(cbJSON.String), &d.CircuitBreaker); err != nil {
			return nil, fmt.Errorf("unmarshal circuit_breaker: %w", err)
		}
	}

	d.CreatedAt, _ = storage.ParseTime(createdAt)
	d.UpdatedAt, _ = storage.ParseTime(updatedAt)
	return &d, nil
}
