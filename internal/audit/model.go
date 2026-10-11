package audit

import "time"

// AuditLog represents an immutable audit trail entry.
type AuditLog struct {
	ID             string     `json:"id"`
	Operator       string     `json:"operator"`
	Action         string     `json:"action"`
	TargetType     string     `json:"target_type"`
	TargetID       string     `json:"target_id"`
	SnapshotBefore *string    `json:"snapshot_before,omitempty"`
	Summary        *string    `json:"summary,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}
