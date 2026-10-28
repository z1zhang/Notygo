// Package backoff holds the retry backoff math shared by the queue's retry
// policy (the actual schedule) and the delivery service (the display-only
// next_attempt_at prediction), so both stay on the same curve.
package backoff

import (
	"math/rand/v2"
	"time"
)

// Config parameterizes the backoff curve.
type Config struct {
	Base          time.Duration // delay before the first retry
	Max           time.Duration // delay cap
	JitterPercent int           // ±spread in percent around the delay
}

// Delay computes the retry delay for an upcoming attempt number (1-based):
// base * 2^(n-1), capped at max, with ±jitterPercent spread.
func Delay(attempt int, cfg Config) time.Duration {
	// avoid overflowing time.Duration on degenerate attempt counts
	shift := min(max(attempt-1, 0), 20)
	d := cfg.Base << uint(shift)
	if d <= 0 || d > cfg.Max { // negative after overflow → also cap
		d = cfg.Max
	}

	jitterRange := float64(d) * float64(cfg.JitterPercent) / 100.0
	jitter := (rand.Float64()*2 - 1) * jitterRange // ±jitterPercent
	return d + time.Duration(jitter)
}
