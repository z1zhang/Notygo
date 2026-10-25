package endpoint

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

// ErrNotFound is returned when a endpoint is not found.
var ErrNotFound = errors.New("endpoint not found")

func newID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// Repository provides CRUD operations for endpoints.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new endpoint repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new endpoint and returns it.
func (r *Repository) Create(ctx context.Context, req CreateEndpointRequest) (*Endpoint, error) {
	id := newID()
	now := time.Now().UTC()

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	authType := "none"
	if req.AuthType != nil {
		authType = *req.AuthType
	}
	authTypeCode, err := authTypeToCode(authType)
	if err != nil {
		return nil, fmt.Errorf("insert endpoint: %w", err)
	}
	payloadContentType := "application/json"
	if req.PayloadContentType != nil {
		payloadContentType = *req.PayloadContentType
	}
	maxBodyBytes := 1048576
	if req.MaxBodyBytes != nil {
		maxBodyBytes = *req.MaxBodyBytes
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO notygo_endpoints (id, name, slug, enabled, auth_type, encrypted_secret, payload_content_type, max_body_bytes, rate_limit, mapping_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		id, req.Name, req.Slug, enabled, authTypeCode, req.Secret, payloadContentType, maxBodyBytes, req.RateLimit, req.MappingID, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert endpoint: %w", err)
	}

	return r.GetByID(ctx, id)
}

// GetByID retrieves a endpoint by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Endpoint, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, name, slug, enabled, auth_type, encrypted_secret, payload_content_type, max_body_bytes, rate_limit, mapping_id, created_at, updated_at
		 FROM notygo_endpoints WHERE id = $1`, id)
	return scanEndpoint(row)
}

// GetBySlug retrieves a endpoint by its slug.
func (r *Repository) GetBySlug(ctx context.Context, slug string) (*Endpoint, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, name, slug, enabled, auth_type, encrypted_secret, payload_content_type, max_body_bytes, rate_limit, mapping_id, created_at, updated_at
		 FROM notygo_endpoints WHERE slug = $1`, slug)
	return scanEndpoint(row)
}

// Update modifies an existing endpoint and returns the updated version.
func (r *Repository) Update(ctx context.Context, id string, req UpdateEndpointRequest) (*Endpoint, error) {
	existing, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	name := existing.Name
	if req.Name != nil {
		name = *req.Name
	}
	slug := existing.Slug
	if req.Slug != nil {
		slug = *req.Slug
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	authType := existing.AuthType
	if req.AuthType != nil {
		authType = *req.AuthType
	}
	secretRef := existing.EncryptedSecret
	if req.Secret != nil {
		secretRef = req.Secret
	}
	payloadContentType := existing.PayloadContentType
	if req.PayloadContentType != nil {
		payloadContentType = *req.PayloadContentType
	}
	maxBodyBytes := existing.MaxBodyBytes
	if req.MaxBodyBytes != nil {
		maxBodyBytes = *req.MaxBodyBytes
	}
	rateLimit := existing.RateLimit
	if req.RateLimit != nil {
		rateLimit = req.RateLimit
	}
	mappingID := existing.MappingID
	if req.MappingID != nil {
		mappingID = req.MappingID
	}
	now := time.Now().UTC()
	authTypeCode, err := authTypeToCode(authType)
	if err != nil {
		return nil, fmt.Errorf("update endpoint: %w", err)
	}
	_, err = r.db.Exec(ctx,
		`UPDATE notygo_endpoints SET name=$1, slug=$2, enabled=$3, auth_type=$4, encrypted_secret=$5, payload_content_type=$6, max_body_bytes=$7, rate_limit=$8, mapping_id=$9, updated_at=$10
		 WHERE id=$11`,
		name, slug, enabled, authTypeCode, secretRef, payloadContentType, maxBodyBytes, rateLimit, mappingID, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update endpoint: %w", err)
	}

	return r.GetByID(ctx, id)
}

// Delete removes a endpoint by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.Exec(ctx, `DELETE FROM notygo_endpoints WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete endpoint: %w", err)
	}
	n := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns a paginated list of endpoints and the total count.
func (r *Repository) List(ctx context.Context, page, size int) ([]Endpoint, int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM notygo_endpoints`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count endpoints: %w", err)
	}
	if total == 0 {
		return []Endpoint{}, 0, nil
	}

	offset := (page - 1) * size
	rows, err := r.db.Query(ctx,
		`SELECT id, name, slug, enabled, auth_type, encrypted_secret, payload_content_type, max_body_bytes, rate_limit, mapping_id, created_at, updated_at
		 FROM notygo_endpoints ORDER BY created_at DESC LIMIT $1 OFFSET $2`, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []Endpoint
	for rows.Next() {
		s, err := scanEndpointFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		endpoints = append(endpoints, *s)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate endpoints: %w", err)
	}

	return endpoints, total, nil
}

// CheckDuplicateUpstreamEventID returns true if an event with the given endpoint_id + upstream_event_id exists.
func (r *Repository) CheckDuplicateUpstreamEventID(ctx context.Context, endpointID, upstreamEventID string) (bool, error) {
	const q = `SELECT 1 FROM notygo_events WHERE endpoint_id = $1 AND upstream_event_id = $2 LIMIT 1`
	var dummy int
	err := r.db.QueryRow(ctx, q, endpointID, upstreamEventID).Scan(&dummy)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check duplicate upstream_event_id: %w", err)
	}
	return true, nil
}

func scanEndpoint(row pgx.Row) (*Endpoint, error) {
	var s Endpoint
	var authTypeCode int16
	err := row.Scan(
		&s.ID, &s.Name, &s.Slug, &s.Enabled, &authTypeCode, &s.EncryptedSecret,
		&s.PayloadContentType, &s.MaxBodyBytes, &s.RateLimit, &s.MappingID,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan endpoint: %w", err)
	}
	s.AuthType, err = authTypeFromCode(authTypeCode)
	if err != nil {
		return nil, fmt.Errorf("scan endpoint: %w", err)
	}
	return &s, nil
}

func scanEndpointFromRows(rows pgx.Rows) (*Endpoint, error) {
	var s Endpoint
	var authTypeCode int16
	err := rows.Scan(
		&s.ID, &s.Name, &s.Slug, &s.Enabled, &authTypeCode, &s.EncryptedSecret,
		&s.PayloadContentType, &s.MaxBodyBytes, &s.RateLimit, &s.MappingID,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan endpoint: %w", err)
	}
	s.AuthType, err = authTypeFromCode(authTypeCode)
	if err != nil {
		return nil, fmt.Errorf("scan endpoint: %w", err)
	}
	return &s, nil
}
