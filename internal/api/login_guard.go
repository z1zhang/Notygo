package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// loginGuard tracks failed login attempts per key (IP and username) and
// locks out keys that exceed the configured failure count within the
// lockout window. It is in-memory by design: Notygo runs as a single
// instance, and losing counters on restart is an acceptable trade-off.
type loginGuard struct {
	mu          sync.Mutex
	maxFailures int
	lockout     time.Duration
	entries     map[string][]time.Time
}

// maxTrackedKeys bounds the entries map so spoofed IPs cannot grow it
// without limit (RealIP trusts proxy headers).
const maxTrackedKeys = 65536

func NewLoginGuard(maxFailures int, lockout time.Duration) *loginGuard {
	if maxFailures < 1 {
		maxFailures = 1
	}
	return &loginGuard{
		maxFailures: maxFailures,
		lockout:     lockout,
		entries:     make(map[string][]time.Time),
	}
}

// isLocked reports whether the key has hit the failure threshold within
// the lockout window.
func (g *loginGuard) isLocked(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	failures := g.pruneLocked(key, time.Now())
	return len(failures) >= g.maxFailures
}

// recordFailure appends a failure timestamp for the key.
func (g *loginGuard) recordFailure(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.entries) >= maxTrackedKeys {
		g.sweepLocked(time.Now())
	}
	failures := g.pruneLocked(key, time.Now())
	g.entries[key] = append(failures, time.Now())
}

// reset clears the failure history for the key (called on successful login).
func (g *loginGuard) reset(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, key)
}

// pruneLocked drops timestamps older than the lockout window; caller holds the lock.
func (g *loginGuard) pruneLocked(key string, now time.Time) []time.Time {
	cutoff := now.Add(-g.lockout)
	kept := g.entries[key][:0]
	for _, ts := range g.entries[key] {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) == 0 {
		delete(g.entries, key)
		return nil
	}
	g.entries[key] = kept
	return kept
}

// sweepLocked drops all fully aged-out entries; caller holds the lock.
func (g *loginGuard) sweepLocked(now time.Time) {
	for key := range g.entries {
		g.pruneLocked(key, now)
	}
}

// clientIP extracts the client IP from RemoteAddr (RealIP middleware has
// already resolved proxy headers into it).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
