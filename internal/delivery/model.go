package delivery

import "time"

// Delivery status constants.
const (
	DeliveryStatusPending    = "pending"
	DeliveryStatusRetryWait  = "retry_wait"
	DeliveryStatusProcessing = "processing"
	DeliveryStatusDelivered  = "delivered"
	DeliveryStatusFailed     = "failed"
	DeliveryStatusSkipped    = "skipped"
)

// Delivery represents a single delivery attempt pipeline for a destination.
type Delivery struct {
	ID               string     `json:"id"`
	EventID          string     `json:"event_id"`
	EventRunID       string     `json:"event_run_id"`
	RuleID           string     `json:"rule_id"`
	DestinationID    string     `json:"destination_id"`
	IdempotencyKey   string     `json:"idempotency_key"`
	Status           string     `json:"status"`
	AttemptCount     int        `json:"attempt_count"`
	MaxAttempts      int        `json:"max_attempts"`
	NextAttemptAt time.Time `json:"next_attempt_at,omitempty"`
	LeaseUntil       *time.Time  `json:"lease_until,omitempty"`
	LeaseOwner       *string    `json:"lease_owner,omitempty"`
	RenderedTitle    *string    `json:"rendered_title,omitempty"`
	RenderedBody     *string    `json:"rendered_body,omitempty"`
	RenderedFormat   *string    `json:"rendered_format,omitempty"`
	LastErrorCode    *string    `json:"last_error_code,omitempty"`
	LastErrorSummary *string    `json:"last_error_summary,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// DeliveryAttempt represents a single HTTP delivery attempt.
type DeliveryAttempt struct {
	ID              string     `json:"id"`
	DeliveryID      string     `json:"delivery_id"`
	AttemptNo       int        `json:"attempt_no"`
	StartedAt time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time  `json:"finished_at,omitempty"`
	RequestSummary  *string    `json:"request_summary,omitempty"`
	ResponseStatus  *int       `json:"response_status,omitempty"`
	ResponseSummary *string    `json:"response_summary,omitempty"`
	ErrorCode       *string    `json:"error_code,omitempty"`
	ErrorSummary    *string    `json:"error_summary,omitempty"`
	DurationMs      *int       `json:"duration_ms,omitempty"`
}
