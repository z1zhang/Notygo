package queue

import (
	"time"

	"github.com/riverqueue/river/rivertype"

	"notygo/internal/backoff"
)

// RetryPolicy is Notygo's client-level retry policy: exponential backoff
// (see backoff.Delay) starting at Base (30s by default), capped at Max (1h),
// with ±JitterPercent spread to avoid retry thundering herds.
//
// The delivery service mirrors this formula when it writes the (display-only)
// deliveries.next_attempt_at, so the UI prediction stays roughly in sync with
// River's actual schedule.
type RetryPolicy struct {
	backoff.Config
}

// NextRetry implements river.ClientRetryPolicy.
func (p *RetryPolicy) NextRetry(job *rivertype.JobRow) time.Time {
	attempt := max(job.Attempt, 1)
	delay := backoff.Delay(attempt, p.Config)

	// Base the next attempt on when the job was attempted, but never in the past.
	basis := timeNowUTC()
	if job.AttemptedAt != nil && job.AttemptedAt.After(basis) {
		basis = *job.AttemptedAt
	}
	return basis.Add(delay)
}
