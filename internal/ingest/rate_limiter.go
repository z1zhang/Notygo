package ingest

import (
	"encoding/json"
	"sync"

	"golang.org/x/time/rate"
)

// SourceRateLimit describes the per-source rate limit configuration.
type SourceRateLimit struct {
	Rate  float64 `json:"rate"`
	Burst int     `json:"burst"`
}

// RateLimiter provides global and per-source rate limiting.
type RateLimiter struct {
	sourceLimiters sync.Map
	globalLimiter  *rate.Limiter
}

// NewRateLimiter creates a new RateLimiter with the given global rate and burst.
func NewRateLimiter(globalRate float64, globalBurst int) *RateLimiter {
	return &RateLimiter{
		globalLimiter: rate.NewLimiter(rate.Limit(globalRate), globalBurst),
	}
}

// Allow checks whether a request from the given source is allowed.
// It checks both the global limiter and the per-source limiter.
// If sourceRateLimit is empty or invalid, only the global limiter is checked.
func (rl *RateLimiter) Allow(slug string, sourceRateLimit string) bool {
	// Check global rate limit first
	if !rl.globalLimiter.Allow() {
		return false
	}

	// If no per-source limit configured, allow
	if sourceRateLimit == "" {
		return true
	}

	// Parse source rate limit
	var srl SourceRateLimit
	if err := json.Unmarshal([]byte(sourceRateLimit), &srl); err != nil {
		// If invalid JSON, allow the request (fail open for source limit)
		return true
	}
	if srl.Rate <= 0 || srl.Burst <= 0 {
		// Invalid rate/burst, allow
		return true
	}

	// Get or create per-source limiter
	limiter, _ := rl.sourceLimiters.LoadOrStore(slug, rate.NewLimiter(rate.Limit(srl.Rate), srl.Burst))
	return limiter.(*rate.Limiter).Allow()
}
