package mapping

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

// ErrNotFound is returned when a mapping is not found.
var ErrNotFound = errors.New("mapping not found")

func newID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// mappingColumns is the standard column list for mapping queries.
const mappingColumns = `id, name, revision, rule_ids, continue_on_match, fields, created_at, updated_at`

// Repository provides CRUD operations for mappings.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new mapping repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new mapping and returns it.
func (r *Repository) Create(ctx context.Context, name string, ruleIDs []string, continueOnMatch bool, fields []MappingEntry) (*Mapping, error) {
	id := newID()
	now := time.Now().UTC()
	fieldsJSON, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("marshal fields: %w", err)
	}
	var ruleIDsJSON *string
	if len(ruleIDs) > 0 {
		b, err := json.Marshal(ruleIDs)
		if err != nil {
			return nil, fmt.Errorf("marshal rule_ids: %w", err)
		}
		s := string(b)
		ruleIDsJSON = &s
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO notygo_mapping (id, name, revision, rule_ids, continue_on_match, fields, created_at, updated_at) VALUES ($1, $2, 1, $3::jsonb, $4, $5::jsonb, $6, $7)`,
		id, name, ruleIDsJSON, continueOnMatch, string(fieldsJSON), now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert mapping: %w", err)
	}

	return r.GetByID(ctx, id)
}

// GetByID retrieves a mapping by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Mapping, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+mappingColumns+` FROM notygo_mapping WHERE id = $1`, id)
	return scanMapping(row)
}

// List returns a paginated list of mappings and the total count.
func (r *Repository) List(ctx context.Context, page, size int) ([]Mapping, int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM notygo_mapping`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count mappings: %w", err)
	}
	if total == 0 {
		return []Mapping{}, 0, nil
	}

	offset := (page - 1) * size
	rows, err := r.db.Query(ctx,
		`SELECT `+mappingColumns+` FROM notygo_mapping ORDER BY created_at DESC LIMIT $1 OFFSET $2`, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list mappings: %w", err)
	}
	defer rows.Close()

	var mappings []Mapping
	for rows.Next() {
		m, err := scanMappingFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		mappings = append(mappings, *m)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate mappings: %w", err)
	}

	return mappings, total, nil
}

// Update modifies an existing mapping, incrementing revision, and returns the updated version.
func (r *Repository) Update(ctx context.Context, id string, name *string, ruleIDs *[]string, continueOnMatch *bool, fields *[]MappingEntry) (*Mapping, error) {
	existing, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	newName := existing.Name
	if name != nil {
		newName = *name
	}

	newRuleIDs := existing.RuleIDs
	if ruleIDs != nil {
		newRuleIDs = *ruleIDs
	}

	newContinueOnMatch := existing.ContinueOnMatch
	if continueOnMatch != nil {
		newContinueOnMatch = *continueOnMatch
	}

	newFields := existing.Fields
	if fields != nil {
		newFields = *fields
	}

	fieldsJSON, err := json.Marshal(newFields)
	if err != nil {
		return nil, fmt.Errorf("marshal fields: %w", err)
	}

	var ruleIDsJSON *string
	if len(newRuleIDs) > 0 {
		b, err := json.Marshal(newRuleIDs)
		if err != nil {
			return nil, fmt.Errorf("marshal rule_ids: %w", err)
		}
		s := string(b)
		ruleIDsJSON = &s
	}

	newRevision := existing.Revision + 1
	now := time.Now().UTC()
	_, err = r.db.Exec(ctx,
		`UPDATE notygo_mapping SET name=$1, revision=$2, rule_ids=$3::jsonb, continue_on_match=$4, fields=$5::jsonb, updated_at=$6 WHERE id=$7`,
		newName, newRevision, ruleIDsJSON, newContinueOnMatch, string(fieldsJSON), now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update mapping: %w", err)
	}

	return r.GetByID(ctx, id)
}

// Delete removes a mapping by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.Exec(ctx, `DELETE FROM notygo_mapping WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete mapping: %w", err)
	}
	n := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanMapping(row pgx.Row) (*Mapping, error) {
	var m Mapping
	var fieldsJSON string
	var ruleIDsJSON sql.NullString
	err := row.Scan(&m.ID, &m.Name, &m.Revision, &ruleIDsJSON, &m.ContinueOnMatch, &fieldsJSON, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan mapping: %w", err)
	}
	if err := json.Unmarshal([]byte(fieldsJSON), &m.Fields); err != nil {
		return nil, fmt.Errorf("unmarshal fields: %w", err)
	}
	if ruleIDsJSON.Valid && ruleIDsJSON.String != "" {
		if err := json.Unmarshal([]byte(ruleIDsJSON.String), &m.RuleIDs); err != nil {
			m.RuleIDs = []string{}
		}
	}
	return &m, nil
}

func scanMappingFromRows(rows pgx.Rows) (*Mapping, error) {
	var m Mapping
	var fieldsJSON string
	var ruleIDsJSON sql.NullString
	err := rows.Scan(&m.ID, &m.Name, &m.Revision, &ruleIDsJSON, &m.ContinueOnMatch, &fieldsJSON, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan mapping: %w", err)
	}
	if err := json.Unmarshal([]byte(fieldsJSON), &m.Fields); err != nil {
		return nil, fmt.Errorf("unmarshal fields: %w", err)
	}
	if ruleIDsJSON.Valid && ruleIDsJSON.String != "" {
		if err := json.Unmarshal([]byte(ruleIDsJSON.String), &m.RuleIDs); err != nil {
			m.RuleIDs = []string{}
		}
	}
	return &m, nil
}
