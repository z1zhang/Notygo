package event

import "time"

// Event status constants.
const (
	EventStatusReceived   = "received"
	EventStatusProcessing = "processing"
	EventStatusCompleted  = "completed"
	EventStatusFailed     = "failed"
	EventStatusExpired    = "expired"
)

// EventRun status constants.
const (
	EventRunStatusProcessing = "processing"
	EventRunStatusCompleted  = "completed"
	EventRunStatusFailed     = "failed"
)

// Event represents an incoming webhook event.
type Event struct {
	ID            string     `json:"id"`
	SourceID      string     `json:"source_id"`
	SourceEventID *string    `json:"source_event_id,omitempty"`
	RequestID     string     `json:"request_id"`
	ReceivedAt time.Time `json:"received_at,omitempty"`
	ContentType   string     `json:"content_type"`
	PayloadRaw    string     `json:"payload_raw"`
	PayloadHash   string     `json:"payload_hash"`
	Status        string     `json:"status"`
	ErrorCode     *string    `json:"error_code,omitempty"`
	ErrorSummary  *string    `json:"error_summary,omitempty"`
	ExpireAt      *time.Time  `json:"expire_at,omitempty"`
	LeaseUntil    *time.Time  `json:"lease_until,omitempty"`
	LeaseOwner    *string    `json:"lease_owner,omitempty"`
}

// EventRun represents a single processing run of an event.
type EventRun struct {
	ID               string          `json:"id"`
	EventID          string          `json:"event_id"`
	RunType          string          `json:"run_type"`
	MappingID        *string         `json:"mapping_id,omitempty"`
	MappingRevision  *int            `json:"mapping_revision,omitempty"`
	RuleSetSnapshot  []RuleSnapshot  `json:"rule_set_snapshot"`
	ParsedFields     map[string]any  `json:"parsed_fields"`
	ValidationResult map[string]any  `json:"validation_result"`
	MatchedRuleIDs   []string        `json:"matched_rule_ids"`
	Status           string          `json:"status"`
	StartedAt time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time  `json:"finished_at,omitempty"`
	ErrorCode        *string         `json:"error_code,omitempty"`
	ErrorSummary     *string         `json:"error_summary,omitempty"`
}

// RuleSnapshot captures a rule at the time of the run for audit purposes.
type RuleSnapshot struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Priority   int      `json:"priority"`
	Conditions any      `json:"conditions"`
	Destinations []string `json:"destinations"`
}
