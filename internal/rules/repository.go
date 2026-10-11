package rules

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

// ErrNotFound is returned when a rule is not found.
var ErrNotFound = errors.New("rule not found")

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

// Repository provides CRUD operations for rules.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new rule repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new rule and returns it.
func (r *Repository) Create(ctx context.Context, rule *Rule) (*Rule, error) {
	id := newID()
	nowStr := storage.FormatNow()

	conditionsJSON, err := json.Marshal(rule.Conditions)
	if err != nil {
		return nil, fmt.Errorf("marshal conditions: %w", err)
	}
	destIDsJSON, err := json.Marshal(rule.DestinationIDs)
	if err != nil {
		return nil, fmt.Errorf("marshal destination_ids: %w", err)
	}
	var dedupeJSON *string
	if rule.DedupeConfig != nil {
		b, err := json.Marshal(rule.DedupeConfig)
		if err != nil {
			return nil, fmt.Errorf("marshal dedupe_config: %w", err)
		}
		s := string(b)
		dedupeJSON = &s
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO rules (id, name, enabled, priority, conditions, condition_mode, continue_on_match, template_id, destination_ids, dedupe_config, revision, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		id, rule.Name, boolToInt(rule.Enabled), rule.Priority, string(conditionsJSON),
		rule.ConditionMode, boolToInt(rule.ContinueOnMatch), rule.TemplateID,
		string(destIDsJSON), dedupeJSON, nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert rule: %w", err)
	}

	return r.GetByID(ctx, id)
}

// GetByID retrieves a rule by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Rule, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, enabled, priority, conditions, condition_mode, continue_on_match, template_id, destination_ids, dedupe_config, revision, created_at, updated_at
		 FROM rules WHERE id = ?`, id)
	return scanRule(row)
}

// List returns a paginated list of rules and the total count.
func (r *Repository) List(ctx context.Context, page, size int) ([]Rule, int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM rules`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count rules: %w", err)
	}
	if total == 0 {
		return []Rule{}, 0, nil
	}

	offset := (page - 1) * size
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, enabled, priority, conditions, condition_mode, continue_on_match, template_id, destination_ids, dedupe_config, revision, created_at, updated_at
		 FROM rules ORDER BY priority DESC, created_at DESC LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list rules: %w", err)
	}
	defer rows.Close()

	var result []Rule
	for rows.Next() {
		rule, err := scanRuleFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *rule)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate rules: %w", err)
	}

	return result, total, nil
}

// ListEnabled returns all enabled rules ordered by priority.
func (r *Repository) ListEnabled(ctx context.Context) ([]Rule, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, enabled, priority, conditions, condition_mode, continue_on_match, template_id, destination_ids, dedupe_config, revision, created_at, updated_at
		 FROM rules WHERE enabled = 1 ORDER BY priority DESC, created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list enabled rules: %w", err)
	}
	defer rows.Close()

	var result []Rule
	for rows.Next() {
		rule, err := scanRuleFromRows(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *rule)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enabled rules: %w", err)
	}

	return result, nil
}

// Update modifies an existing rule, incrementing revision, and returns the updated version.
func (r *Repository) Update(ctx context.Context, id string, rule *Rule) (*Rule, error) {
	existing, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	conditionsJSON, err := json.Marshal(rule.Conditions)
	if err != nil {
		return nil, fmt.Errorf("marshal conditions: %w", err)
	}
	destIDsJSON, err := json.Marshal(rule.DestinationIDs)
	if err != nil {
		return nil, fmt.Errorf("marshal destination_ids: %w", err)
	}
	var dedupeJSON *string
	if rule.DedupeConfig != nil {
		b, err := json.Marshal(rule.DedupeConfig)
		if err != nil {
			return nil, fmt.Errorf("marshal dedupe_config: %w", err)
		}
		s := string(b)
		dedupeJSON = &s
	}

	newRevision := existing.Revision + 1
	nowStr := storage.FormatNow()
	_, err = r.db.ExecContext(ctx,
		`UPDATE rules SET name=?, enabled=?, priority=?, conditions=?, condition_mode=?, continue_on_match=?, template_id=?, destination_ids=?, dedupe_config=?, revision=?, updated_at=?
		 WHERE id=?`,
		rule.Name, boolToInt(rule.Enabled), rule.Priority, string(conditionsJSON),
		rule.ConditionMode, boolToInt(rule.ContinueOnMatch), rule.TemplateID,
		string(destIDsJSON), dedupeJSON, newRevision, nowStr, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update rule: %w", err)
	}

	return r.GetByID(ctx, id)
}

// Delete removes a rule by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanRule(row *sql.Row) (*Rule, error) {
	var r Rule
	var enabled int
	var continueOnMatch int
	var conditionsJSON string
	var destIDsJSON string
	var dedupeJSON sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(
		&r.ID, &r.Name, &enabled, &r.Priority, &conditionsJSON, &r.ConditionMode,
		&continueOnMatch, &r.TemplateID, &destIDsJSON, &dedupeJSON, &r.Revision,
		&createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan rule: %w", err)
	}

	r.Enabled = intToBool(enabled)
	r.ContinueOnMatch = intToBool(continueOnMatch)

	if err := json.Unmarshal([]byte(conditionsJSON), &r.Conditions); err != nil {
		return nil, fmt.Errorf("unmarshal conditions: %w", err)
	}
	if err := json.Unmarshal([]byte(destIDsJSON), &r.DestinationIDs); err != nil {
		return nil, fmt.Errorf("unmarshal destination_ids: %w", err)
	}
	if dedupeJSON.Valid && dedupeJSON.String != "" {
		if err := json.Unmarshal([]byte(dedupeJSON.String), &r.DedupeConfig); err != nil {
			return nil, fmt.Errorf("unmarshal dedupe_config: %w", err)
		}
	}

	r.CreatedAt, _ = storage.ParseTime(createdAt)
	r.UpdatedAt, _ = storage.ParseTime(updatedAt)
	return &r, nil
}

func scanRuleFromRows(rows *sql.Rows) (*Rule, error) {
	var r Rule
	var enabled int
	var continueOnMatch int
	var conditionsJSON string
	var destIDsJSON string
	var dedupeJSON sql.NullString
	var createdAt, updatedAt string

	err := rows.Scan(
		&r.ID, &r.Name, &enabled, &r.Priority, &conditionsJSON, &r.ConditionMode,
		&continueOnMatch, &r.TemplateID, &destIDsJSON, &dedupeJSON, &r.Revision,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan rule: %w", err)
	}

	r.Enabled = intToBool(enabled)
	r.ContinueOnMatch = intToBool(continueOnMatch)

	if err := json.Unmarshal([]byte(conditionsJSON), &r.Conditions); err != nil {
		return nil, fmt.Errorf("unmarshal conditions: %w", err)
	}
	if err := json.Unmarshal([]byte(destIDsJSON), &r.DestinationIDs); err != nil {
		return nil, fmt.Errorf("unmarshal destination_ids: %w", err)
	}
	if dedupeJSON.Valid && dedupeJSON.String != "" {
		if err := json.Unmarshal([]byte(dedupeJSON.String), &r.DedupeConfig); err != nil {
			return nil, fmt.Errorf("unmarshal dedupe_config: %w", err)
		}
	}

	r.CreatedAt, _ = storage.ParseTime(createdAt)
	r.UpdatedAt, _ = storage.ParseTime(updatedAt)
	return &r, nil
}
