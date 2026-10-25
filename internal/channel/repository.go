package channel

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

// ErrNotFound is returned when a channel is not found.
var ErrNotFound = errors.New("channel not found")

func newID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// Repository provides CRUD operations for channels.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new channel repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new channel and returns it.
func (r *Repository) Create(ctx context.Context, req CreateChannelRequest) (*Channel, error) {
	id := newID()
	now := time.Now().UTC()

	name := ""
	if req.Name != nil {
		name = *req.Name
	}
	channelType := ChannelType("http")
	if req.Type != nil {
		channelType = *req.Type
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	timeoutMs := 10000
	if req.TimeoutMs != nil {
		timeoutMs = *req.TimeoutMs
	}

	maxAttempts := 5
	if req.MaxAttempts != nil {
		maxAttempts = *req.MaxAttempts
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO notygo_channels (id, name, type, enabled, config, encrypted_secret, timeout_ms, max_attempts, revision, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, 1, $9, $10)`,
		id, name, string(channelType), enabled,
		string(configJSON), req.Secret, timeoutMs, maxAttempts,
		now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert channel: %w", err)
	}

	return r.GetByID(ctx, id)
}

// GetByID retrieves a channel by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Channel, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, name, type, enabled, config, encrypted_secret, timeout_ms, max_attempts, revision, created_at, updated_at
		 FROM notygo_channels WHERE id = $1`, id)
	return scanChannel(row)
}

// List returns a paginated list of channels and the total count.
func (r *Repository) List(ctx context.Context, page, size int) ([]Channel, int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM notygo_channels`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count channels: %w", err)
	}
	if total == 0 {
		return []Channel{}, 0, nil
	}

	offset := (page - 1) * size
	rows, err := r.db.Query(ctx,
		`SELECT id, name, type, enabled, config, encrypted_secret, timeout_ms, max_attempts, revision, created_at, updated_at
		 FROM notygo_channels ORDER BY created_at DESC LIMIT $1 OFFSET $2`, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()

	var result []Channel
	for rows.Next() {
		d, err := scanChannelFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *d)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate channels: %w", err)
	}

	return result, total, nil
}

// Update modifies an existing channel, incrementing revision, and returns the updated version.
// Only non-nil fields in the request are updated.
func (r *Repository) Update(ctx context.Context, id string, req UpdateChannelRequest) (*Channel, error) {
	existing, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	name := existing.Name
	if req.Name != nil {
		name = *req.Name
	}
	channelType := existing.Type
	if req.Type != nil {
		channelType = *req.Type
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	config := existing.Config
	if req.Config != nil {
		config = req.Config
	}
	secretRef := existing.EncryptedSecret
	if req.Secret != nil {
		secretRef = req.Secret
	}
	timeoutMs := existing.TimeoutMs
	if req.TimeoutMs != nil {
		timeoutMs = *req.TimeoutMs
	}
	maxAttempts := existing.MaxAttempts
	if req.MaxAttempts != nil {
		maxAttempts = *req.MaxAttempts
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	newRevision := existing.Revision + 1
	now := time.Now().UTC()
	_, err = r.db.Exec(ctx,
		`UPDATE notygo_channels SET name=$1, type=$2, enabled=$3, config=$4::jsonb, encrypted_secret=$5, timeout_ms=$6, max_attempts=$7, revision=$8, updated_at=$9
		 WHERE id=$10`,
		name, string(channelType), enabled,
		string(configJSON), secretRef, timeoutMs, maxAttempts,
		newRevision, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update channel: %w", err)
	}

	return r.GetByID(ctx, id)
}

// Delete removes a channel by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.Exec(ctx, `DELETE FROM notygo_channels WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	n := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanChannel(row pgx.Row) (*Channel, error) {
	var d Channel
	var configJSON string

	err := row.Scan(
		&d.ID, &d.Name, &d.Type, &d.Enabled, &configJSON, &d.EncryptedSecret,
		&d.TimeoutMs, &d.MaxAttempts, &d.Revision,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan channel: %w", err)
	}

	if err := json.Unmarshal([]byte(configJSON), &d.Config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &d, nil
}

func scanChannelFromRows(rows pgx.Rows) (*Channel, error) {
	var d Channel
	var configJSON string

	err := rows.Scan(
		&d.ID, &d.Name, &d.Type, &d.Enabled, &configJSON, &d.EncryptedSecret,
		&d.TimeoutMs, &d.MaxAttempts, &d.Revision,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan channel: %w", err)
	}

	if err := json.Unmarshal([]byte(configJSON), &d.Config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &d, nil
}
