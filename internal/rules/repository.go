package rules

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

// ErrNotFound is returned when a rule is not found.
var ErrNotFound = errors.New("rule not found")

func newID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// ruleColumns is the standard column list for rule queries.
const ruleColumns = `id, name, enabled, priority, conditions, template_id, channel_ids, dedupe_config, revision, created_at, updated_at`

// Repository provides CRUD operations for rules.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new rule repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new rule and returns it.
func (r *Repository) Create(ctx context.Context, rule *Rule) (*Rule, error) {
	id := newID()
	now := time.Now().UTC()

	conditionsJSON, err := json.Marshal(rule.Conditions)
	if err != nil {
		return nil, fmt.Errorf("marshal conditions: %w", err)
	}
	destIDsJSON, err := json.Marshal(rule.ChannelIDs)
	if err != nil {
		return nil, fmt.Errorf("marshal channel_ids: %w", err)
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

	_, err = r.db.Exec(ctx,
		`INSERT INTO notygo_rules (id, name, enabled, priority, conditions, template_id, channel_ids, dedupe_config, revision, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7::jsonb, $8::jsonb, 1, $9, $10)`,
		id, rule.Name, rule.Enabled, rule.Priority, string(conditionsJSON),
		rule.TemplateID,
		string(destIDsJSON), dedupeJSON, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert rule: %w", err)
	}

	return r.GetByID(ctx, id)
}

// GetByID retrieves a rule by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Rule, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+ruleColumns+` FROM notygo_rules WHERE id = $1`, id)
	return scanRule(row)
}

// List returns a paginated list of rules and the total count.
func (r *Repository) List(ctx context.Context, page, size int) ([]Rule, int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM notygo_rules`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count rules: %w", err)
	}
	if total == 0 {
		return []Rule{}, 0, nil
	}

	offset := (page - 1) * size
	rows, err := r.db.Query(ctx,
		`SELECT `+ruleColumns+` FROM notygo_rules ORDER BY priority DESC, created_at DESC LIMIT $1 OFFSET $2`, size, offset)
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
	rows, err := r.db.Query(ctx,
		`SELECT `+ruleColumns+` FROM notygo_rules WHERE enabled = TRUE ORDER BY priority DESC, created_at ASC`)
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

// ListEnabledByIDs returns enabled rules matching the given IDs, ordered by priority.
// Returns empty slice if ids is empty.
func (r *Repository) ListEnabledByIDs(ctx context.Context, ids []string) ([]Rule, error) {
	if len(ids) == 0 {
		return []Rule{}, nil
	}

	// pgx passes []string natively as a PG array: id = ANY($1)
	query := fmt.Sprintf(
		`SELECT %s FROM notygo_rules WHERE enabled = TRUE AND id = ANY($1) ORDER BY priority DESC, created_at ASC`,
		ruleColumns,
	)

	rows, err := r.db.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("list enabled rules by ids: %w", err)
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
		return nil, fmt.Errorf("iterate enabled rules by ids: %w", err)
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
	destIDsJSON, err := json.Marshal(rule.ChannelIDs)
	if err != nil {
		return nil, fmt.Errorf("marshal channel_ids: %w", err)
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
	now := time.Now().UTC()
	_, err = r.db.Exec(ctx,
		`UPDATE notygo_rules SET name=$1, enabled=$2, priority=$3, conditions=$4::jsonb, template_id=$5, channel_ids=$6::jsonb, dedupe_config=$7::jsonb, revision=$8, updated_at=$9
		 WHERE id=$10`,
		rule.Name, rule.Enabled, rule.Priority, string(conditionsJSON),
		rule.TemplateID,
		string(destIDsJSON), dedupeJSON, newRevision, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update rule: %w", err)
	}

	return r.GetByID(ctx, id)
}

// Delete removes a rule by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.Exec(ctx, `DELETE FROM notygo_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}
	n := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanRule(row pgx.Row) (*Rule, error) {
	var r Rule
	var conditionsJSON string
	var destIDsJSON string
	var dedupeJSON sql.NullString

	err := row.Scan(
		&r.ID, &r.Name, &r.Enabled, &r.Priority, &conditionsJSON,
		&r.TemplateID, &destIDsJSON, &dedupeJSON, &r.Revision,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan rule: %w", err)
	}

	if err := json.Unmarshal([]byte(conditionsJSON), &r.Conditions); err != nil {
		return nil, fmt.Errorf("unmarshal conditions: %w", err)
	}
	if err := json.Unmarshal([]byte(destIDsJSON), &r.ChannelIDs); err != nil {
		return nil, fmt.Errorf("unmarshal channel_ids: %w", err)
	}
	if dedupeJSON.Valid && dedupeJSON.String != "" {
		if err := json.Unmarshal([]byte(dedupeJSON.String), &r.DedupeConfig); err != nil {
			return nil, fmt.Errorf("unmarshal dedupe_config: %w", err)
		}
	}

	return &r, nil
}

func scanRuleFromRows(rows pgx.Rows) (*Rule, error) {
	var r Rule
	var conditionsJSON string
	var destIDsJSON string
	var dedupeJSON sql.NullString

	err := rows.Scan(
		&r.ID, &r.Name, &r.Enabled, &r.Priority, &conditionsJSON,
		&r.TemplateID, &destIDsJSON, &dedupeJSON, &r.Revision,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan rule: %w", err)
	}

	if err := json.Unmarshal([]byte(conditionsJSON), &r.Conditions); err != nil {
		return nil, fmt.Errorf("unmarshal conditions: %w", err)
	}
	if err := json.Unmarshal([]byte(destIDsJSON), &r.ChannelIDs); err != nil {
		return nil, fmt.Errorf("unmarshal channel_ids: %w", err)
	}
	if dedupeJSON.Valid && dedupeJSON.String != "" {
		if err := json.Unmarshal([]byte(dedupeJSON.String), &r.DedupeConfig); err != nil {
			return nil, fmt.Errorf("unmarshal dedupe_config: %w", err)
		}
	}

	return &r, nil
}
