package templates

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"zpush/internal/storage"
)

// ErrNotFound is returned when a template is not found.
var ErrNotFound = errors.New("template not found")

func newID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// Repository provides CRUD operations for templates.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new template repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new template and returns it.
func (r *Repository) Create(ctx context.Context, name, format, titleTemplate, bodyTemplate string) (*Template, error) {
	id := newID()
	nowStr := storage.FormatNow()

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO templates (id, name, format, title_template, body_template, revision, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
		id, name, format, titleTemplate, bodyTemplate, nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert template: %w", err)
	}

	return r.GetByID(ctx, id)
}

// GetByID retrieves a template by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Template, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, format, title_template, body_template, revision, created_at, updated_at FROM templates WHERE id = ?`, id)
	return scanTemplate(row)
}

// List returns a paginated list of templates and the total count.
func (r *Repository) List(ctx context.Context, page, size int) ([]Template, int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM templates`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count templates: %w", err)
	}
	if total == 0 {
		return []Template{}, 0, nil
	}

	offset := (page - 1) * size
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, format, title_template, body_template, revision, created_at, updated_at FROM templates ORDER BY created_at DESC LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	var result []Template
	for rows.Next() {
		t, err := scanTemplateFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *t)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate templates: %w", err)
	}

	return result, total, nil
}

// Update modifies an existing template, incrementing revision, and returns the updated version.
func (r *Repository) Update(ctx context.Context, id string, name *string, format *string, titleTemplate *string, bodyTemplate *string) (*Template, error) {
	existing, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	newName := existing.Name
	if name != nil {
		newName = *name
	}
	newFormat := existing.Format
	if format != nil {
		newFormat = *format
	}
	newTitle := existing.TitleTemplate
	if titleTemplate != nil {
		newTitle = *titleTemplate
	}
	newBody := existing.BodyTemplate
	if bodyTemplate != nil {
		newBody = *bodyTemplate
	}

	newRevision := existing.Revision + 1
	nowStr := storage.FormatNow()
	_, err = r.db.ExecContext(ctx,
		`UPDATE templates SET name=?, format=?, title_template=?, body_template=?, revision=?, updated_at=? WHERE id=?`,
		newName, newFormat, newTitle, newBody, newRevision, nowStr, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update template: %w", err)
	}

	return r.GetByID(ctx, id)
}

// Delete removes a template by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM templates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete template: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanTemplate(row *sql.Row) (*Template, error) {
	var t Template
	var createdAt, updatedAt string
	err := row.Scan(&t.ID, &t.Name, &t.Format, &t.TitleTemplate, &t.BodyTemplate, &t.Revision, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan template: %w", err)
	}
	t.CreatedAt, _ = storage.ParseTime(createdAt)
	t.UpdatedAt, _ = storage.ParseTime(updatedAt)
	return &t, nil
}

func scanTemplateFromRows(rows *sql.Rows) (*Template, error) {
	var t Template
	var createdAt, updatedAt string
	err := rows.Scan(&t.ID, &t.Name, &t.Format, &t.TitleTemplate, &t.BodyTemplate, &t.Revision, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan template: %w", err)
	}
	t.CreatedAt, _ = storage.ParseTime(createdAt)
	t.UpdatedAt, _ = storage.ParseTime(updatedAt)
	return &t, nil
}
