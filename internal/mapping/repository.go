package mapping

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

// ErrNotFound is returned when a mapping is not found.
var ErrNotFound = errors.New("mapping not found")

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

// mappingColumns is the standard column list for mapping queries.
const mappingColumns = `id, name, revision, rule_ids, continue_on_match, fields, created_at, updated_at`

// Repository provides CRUD operations for mappings.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new mapping repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new mapping and returns it.
func (r *Repository) Create(ctx context.Context, name string, ruleIDs []string, continueOnMatch bool, fields []MappingEntry) (*Mapping, error) {
	id := newID()
	nowStr := storage.FormatNow()
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

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO mappings (id, name, revision, rule_ids, continue_on_match, fields, created_at, updated_at) VALUES (?, ?, 1, ?, ?, ?, ?, ?)`,
		id, name, ruleIDsJSON, boolToInt(continueOnMatch), string(fieldsJSON), nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert mapping: %w", err)
	}

	return r.GetByID(ctx, id)
}

// GetByID retrieves a mapping by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Mapping, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+mappingColumns+` FROM mappings WHERE id = ?`, id)
	return scanMapping(row)
}

// List returns a paginated list of mappings and the total count.
func (r *Repository) List(ctx context.Context, page, size int) ([]Mapping, int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mappings`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count mappings: %w", err)
	}
	if total == 0 {
		return []Mapping{}, 0, nil
	}

	offset := (page - 1) * size
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+mappingColumns+` FROM mappings ORDER BY created_at DESC LIMIT ? OFFSET ?`, size, offset)
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
	nowStr := storage.FormatNow()
	_, err = r.db.ExecContext(ctx,
		`UPDATE mappings SET name=?, revision=?, rule_ids=?, continue_on_match=?, fields=?, updated_at=? WHERE id=?`,
		newName, newRevision, ruleIDsJSON, boolToInt(newContinueOnMatch), string(fieldsJSON), nowStr, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update mapping: %w", err)
	}

	return r.GetByID(ctx, id)
}

// Delete removes a mapping by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM mappings WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete mapping: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanMapping(row *sql.Row) (*Mapping, error) {
	var m Mapping
	var fieldsJSON string
	var ruleIDsJSON sql.NullString
	var continueOnMatch int
	var createdAt, updatedAt string
	err := row.Scan(&m.ID, &m.Name, &m.Revision, &ruleIDsJSON, &continueOnMatch, &fieldsJSON, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
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
	m.ContinueOnMatch = intToBool(continueOnMatch)
	m.CreatedAt, _ = storage.ParseTime(createdAt)
	m.UpdatedAt, _ = storage.ParseTime(updatedAt)
	return &m, nil
}

func scanMappingFromRows(rows *sql.Rows) (*Mapping, error) {
	var m Mapping
	var fieldsJSON string
	var ruleIDsJSON sql.NullString
	var continueOnMatch int
	var createdAt, updatedAt string
	err := rows.Scan(&m.ID, &m.Name, &m.Revision, &ruleIDsJSON, &continueOnMatch, &fieldsJSON, &createdAt, &updatedAt)
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
	m.ContinueOnMatch = intToBool(continueOnMatch)
	m.CreatedAt, _ = storage.ParseTime(createdAt)
	m.UpdatedAt, _ = storage.ParseTime(updatedAt)
	return &m, nil
}
