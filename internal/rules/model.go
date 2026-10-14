package rules

import "time"

// Rule represents a routing/matching rule.
type Rule struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Enabled         bool          `json:"enabled"`
	Priority        int           `json:"priority"`
	Conditions      ConditionGroup `json:"conditions"`
	ConditionMode   string        `json:"condition_mode"`
	TemplateID      *string       `json:"template_id,omitempty"`
	DestinationIDs  []string      `json:"destination_ids"`
	DedupeConfig    *DedupeConfig `json:"dedupe_config,omitempty"`
	Revision        int           `json:"revision"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// ConditionGroup is the top-level container for rule conditions.
// It can hold either a logical operator with nested groups/conditions,
// or a single condition.
type ConditionGroup struct {
	Operator   string        `json:"operator,omitempty"` // "and", "or"
	Conditions []Condition   `json:"conditions,omitempty"`
	Groups     []ConditionGroup `json:"groups,omitempty"`
}

// Condition represents a single match condition.
type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"` // "eq", "neq", "contains", "regex", "gt", "lt", "gte", "lte", "exists"
	Value    string `json:"value"`
}

// DedupeConfig configures deduplication behaviour for a rule.
// When enabled (WindowSeconds > 0), duplicate payloads within the window are skipped.
// The dedupe key is always payload_hash (SHA-256 of the raw request body).
type DedupeConfig struct {
	WindowSeconds int `json:"window_seconds"`
}
