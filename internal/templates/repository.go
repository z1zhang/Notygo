package templates

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

// ErrNotFound is returned when a template is not found.
var ErrNotFound = errors.New("template not found")

func newID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// Repository provides CRUD operations for templates.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new template repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new template and returns it.
func (r *Repository) Create(ctx context.Context, name, format, titleTemplate, bodyTemplate string) (*Template, error) {
	id := newID()
	now := time.Now().UTC()
	formatCode, err := formatToCode(format)
	if err != nil {
		return nil, fmt.Errorf("insert template: %w", err)
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO notygo_templates (id, name, format, title_template, body_template, revision, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 1, $6, $7)`,
		id, name, formatCode, titleTemplate, bodyTemplate, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert template: %w", err)
	}

	return r.GetByID(ctx, id)
}

// GetByID retrieves a template by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Template, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, name, format, title_template, body_template, revision, created_at, updated_at FROM notygo_templates WHERE id = $1`, id)
	return scanTemplate(row)
}

// List returns a paginated list of templates and the total count.
func (r *Repository) List(ctx context.Context, page, size int) ([]Template, int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM notygo_templates`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count templates: %w", err)
	}
	if total == 0 {
		return []Template{}, 0, nil
	}

	offset := (page - 1) * size
	rows, err := r.db.Query(ctx,
		`SELECT id, name, format, title_template, body_template, revision, created_at, updated_at FROM notygo_templates ORDER BY created_at DESC LIMIT $1 OFFSET $2`, size, offset)
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
	now := time.Now().UTC()
	formatCode, err := formatToCode(newFormat)
	if err != nil {
		return nil, fmt.Errorf("update template: %w", err)
	}
	_, err = r.db.Exec(ctx,
		`UPDATE notygo_templates SET name=$1, format=$2, title_template=$3, body_template=$4, revision=$5, updated_at=$6 WHERE id=$7`,
		newName, formatCode, newTitle, newBody, newRevision, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update template: %w", err)
	}

	return r.GetByID(ctx, id)
}

// Delete removes a template by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.Exec(ctx, `DELETE FROM notygo_templates WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete template: %w", err)
	}
	n := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanTemplate(row pgx.Row) (*Template, error) {
	var t Template
	var formatCode int16
	err := row.Scan(&t.ID, &t.Name, &formatCode, &t.TitleTemplate, &t.BodyTemplate, &t.Revision, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan template: %w", err)
	}
	t.Format, err = formatFromCode(formatCode)
	if err != nil {
		return nil, fmt.Errorf("scan template: %w", err)
	}
	return &t, nil
}

func scanTemplateFromRows(rows pgx.Rows) (*Template, error) {
	var t Template
	var formatCode int16
	err := rows.Scan(&t.ID, &t.Name, &formatCode, &t.TitleTemplate, &t.BodyTemplate, &t.Revision, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan template: %w", err)
	}
	t.Format, err = formatFromCode(formatCode)
	if err != nil {
		return nil, fmt.Errorf("scan template: %w", err)
	}
	return &t, nil
}
