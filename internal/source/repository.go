package source

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"zpush/internal/storage"

	"github.com/oklog/ulid/v2"
)

// ErrNotFound is returned when a source is not found.
var ErrNotFound = errors.New("source not found")

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

// Repository provides CRUD operations for sources.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new source repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new source and returns it.
func (r *Repository) Create(ctx context.Context, req CreateSourceRequest) (*Source, error) {
	id := newID()
	nowStr := storage.FormatNow()

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	authType := "none"
	if req.AuthType != nil {
		authType = *req.AuthType
	}
	payloadContentType := "application/json"
	if req.PayloadContentType != nil {
		payloadContentType = *req.PayloadContentType
	}
	maxBodyBytes := 1048576
	if req.MaxBodyBytes != nil {
		maxBodyBytes = *req.MaxBodyBytes
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sources (id, name, slug, enabled, auth_type, secret_ref, payload_content_type, max_body_bytes, rate_limit, mapping_id, expected_response, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.Name, req.Slug, boolToInt(enabled), authType, req.SecretRef, payloadContentType, maxBodyBytes, req.RateLimit, req.MappingID, req.ExpectedResponse, nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert source: %w", err)
	}

	return r.GetByID(ctx, id)
}

// GetByID retrieves a source by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Source, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, slug, enabled, auth_type, secret_ref, payload_content_type, max_body_bytes, rate_limit, mapping_id, expected_response, created_at, updated_at
		 FROM sources WHERE id = ?`, id)
	return scanSource(row)
}

// GetBySlug retrieves a source by its slug.
func (r *Repository) GetBySlug(ctx context.Context, slug string) (*Source, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, slug, enabled, auth_type, secret_ref, payload_content_type, max_body_bytes, rate_limit, mapping_id, expected_response, created_at, updated_at
		 FROM sources WHERE slug = ?`, slug)
	return scanSource(row)
}

// Update modifies an existing source and returns the updated version.
func (r *Repository) Update(ctx context.Context, id string, req UpdateSourceRequest) (*Source, error) {
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
	secretRef := existing.SecretRef
	if req.SecretRef != nil {
		secretRef = req.SecretRef
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
	expectedResponse := existing.ExpectedResponse
	if req.ExpectedResponse != nil {
		expectedResponse = req.ExpectedResponse
	}

	nowStr := storage.FormatNow()
	_, err = r.db.ExecContext(ctx,
		`UPDATE sources SET name=?, slug=?, enabled=?, auth_type=?, secret_ref=?, payload_content_type=?, max_body_bytes=?, rate_limit=?, mapping_id=?, expected_response=?, updated_at=?
		 WHERE id=?`,
		name, slug, boolToInt(enabled), authType, secretRef, payloadContentType, maxBodyBytes, rateLimit, mappingID, expectedResponse, nowStr, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update source: %w", err)
	}

	return r.GetByID(ctx, id)
}

// Delete removes a source by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM sources WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete source: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns a paginated list of sources and the total count.
func (r *Repository) List(ctx context.Context, page, size int) ([]Source, int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sources`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count sources: %w", err)
	}
	if total == 0 {
		return []Source{}, 0, nil
	}

	offset := (page - 1) * size
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, slug, enabled, auth_type, secret_ref, payload_content_type, max_body_bytes, rate_limit, mapping_id, expected_response, created_at, updated_at
		 FROM sources ORDER BY created_at DESC LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		s, err := scanSourceFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		sources = append(sources, *s)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate sources: %w", err)
	}

	return sources, total, nil
}

// CheckDuplicateSourceEventID returns true if an event with the given source_id + source_event_id exists.
func (r *Repository) CheckDuplicateSourceEventID(ctx context.Context, sourceID, sourceEventID string) (bool, error) {
	const q = `SELECT 1 FROM events WHERE source_id = ? AND source_event_id = ? LIMIT 1`
	var dummy int
	err := r.db.QueryRowContext(ctx, q, sourceID, sourceEventID).Scan(&dummy)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check duplicate source_event_id: %w", err)
	}
	return true, nil
}

func scanSource(row *sql.Row) (*Source, error) {
	var s Source
	var enabled int
	var createdAt, updatedAt string
	err := row.Scan(
		&s.ID, &s.Name, &s.Slug, &enabled, &s.AuthType, &s.SecretRef,
		&s.PayloadContentType, &s.MaxBodyBytes, &s.RateLimit, &s.MappingID, &s.ExpectedResponse,
		&createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan source: %w", err)
	}
	s.Enabled = intToBool(enabled)
	s.CreatedAt, _ = storage.ParseTime(createdAt)
	s.UpdatedAt, _ = storage.ParseTime(updatedAt)
	return &s, nil
}

func scanSourceFromRows(rows *sql.Rows) (*Source, error) {
	var s Source
	var enabled int
	var createdAt, updatedAt string
	err := rows.Scan(
		&s.ID, &s.Name, &s.Slug, &enabled, &s.AuthType, &s.SecretRef,
		&s.PayloadContentType, &s.MaxBodyBytes, &s.RateLimit, &s.MappingID, &s.ExpectedResponse,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan source: %w", err)
	}
	s.Enabled = intToBool(enabled)
	s.CreatedAt, _ = storage.ParseTime(createdAt)
	s.UpdatedAt, _ = storage.ParseTime(updatedAt)
	return &s, nil
}
