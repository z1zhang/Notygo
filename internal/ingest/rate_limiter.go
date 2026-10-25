package ingest

import (
	"encoding/json"
	"sync"

	"golang.org/x/time/rate"
)

// EndpointRateLimit describes the per-endpoint rate limit configuration.
type EndpointRateLimit struct {
	Rate  float64 `json:"rate"`
	Burst int     `json:"burst"`
}

// RateLimiter provides global and per-endpoint rate limiting.
type RateLimiter struct {
	endpointLimiters sync.Map
	globalLimiter    *rate.Limiter
}

// NewRateLimiter creates a new RateLimiter with the given global rate and burst.
func NewRateLimiter(globalRate float64, globalBurst int) *RateLimiter {
	return &RateLimiter{
		globalLimiter: rate.NewLimiter(rate.Limit(globalRate), globalBurst),
	}
}

// Allow checks whether a request from the given endpoint is allowed.
// It checks both the global limiter and the per-endpoint limiter.
// If endpointRateLimit is empty or invalid, only the global limiter is checked.
func (rl *RateLimiter) Allow(slug string, endpointRateLimit string) bool {
	// Check global rate limit first
	if !rl.globalLimiter.Allow() {
		return false
	}

	// If no per-endpoint limit configured, allow
	if endpointRateLimit == "" {
		return true
	}

	// Parse endpoint rate limit
	var srl EndpointRateLimit
	if err := json.Unmarshal([]byte(endpointRateLimit), &srl); err != nil {
		// If invalid JSON, allow the request (fail open for endpoint limit)
		return true
	}
	if srl.Rate <= 0 || srl.Burst <= 0 {
		// Invalid rate/burst, allow
		return true
	}

	// Get or create per-endpoint limiter
	limiter, _ := rl.endpointLimiters.LoadOrStore(slug, rate.NewLimiter(rate.Limit(srl.Rate), srl.Burst))
	return limiter.(*rate.Limiter).Allow()
}
