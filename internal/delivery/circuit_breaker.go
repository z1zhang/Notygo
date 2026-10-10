package delivery

import (
	"fmt"
	"sync"
	"time"
)

// CircuitBreakerConfig configures circuit breaker behavior for a destination.
type CircuitBreakerConfig struct {
	FailureThreshold int    `json:"failure_threshold"`
	RecoveryInterval string `json:"recovery_interval"`
	HalfOpenMaxProbe int    `json:"half_open_max_probe"`
}

// ParseRecoveryInterval parses the RecoveryInterval string into a time.Duration.
func (c CircuitBreakerConfig) ParseRecoveryInterval() time.Duration {
	if c.RecoveryInterval == "" {
		return 30 * time.Second
	}
	d, err := time.ParseDuration(c.RecoveryInterval)
	if err != nil {
		// Try as seconds
		return 30 * time.Second
	}
	return d
}

// CircuitBreakerManager manages per-destination circuit breaker state.
type CircuitBreakerManager struct {
	states sync.Map // destID -> *circuitState
}

// NewCircuitBreakerManager creates a new CircuitBreakerManager.
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{}
}

type circuitState struct {
	mu                  sync.Mutex
	state               string // "closed", "open", "half_open"
	consecutiveFailures int
	lastFailureAt       time.Time
	circuitOpenedAt     time.Time
	halfOpenProbeCount  int
}

// IsOpen checks if the circuit breaker is open for the given destination.
// If the circuit is open but recovery_interval has elapsed, it transitions to half_open.
func (m *CircuitBreakerManager) IsOpen(destID string, cfg CircuitBreakerConfig) bool {
	st := m.getOrCreateState(destID)
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.state == "closed" {
		return false
	}

	if st.state == "open" {
		recoveryInterval := cfg.ParseRecoveryInterval()
		if time.Since(st.circuitOpenedAt) >= recoveryInterval {
			// Transition to half_open
			st.state = "half_open"
			st.halfOpenProbeCount = 0
			return false
		}
		return true
	}

	// half_open: allow limited probes
	if st.state == "half_open" {
		maxProbe := cfg.HalfOpenMaxProbe
		if maxProbe <= 0 {
			maxProbe = 1
		}
		if st.halfOpenProbeCount < maxProbe {
			return false
		}
		return true
	}

	return false
}

// RecordFailure records a failure for the given destination's circuit breaker.
func (m *CircuitBreakerManager) RecordFailure(destID string, cfg CircuitBreakerConfig) {
	st := m.getOrCreateState(destID)
	st.mu.Lock()
	defer st.mu.Unlock()

	st.consecutiveFailures++
	st.lastFailureAt = time.Now()

	if st.state == "half_open" {
		// Failure in half_open -> back to open
		st.state = "open"
		st.circuitOpenedAt = time.Now()
		st.halfOpenProbeCount = 0
		return
	}

	// closed state: check threshold
	if st.state == "closed" && st.consecutiveFailures >= cfg.FailureThreshold {
		st.state = "open"
		st.circuitOpenedAt = time.Now()
	}
}

// RecordSuccess records a success for the given destination's circuit breaker.
func (m *CircuitBreakerManager) RecordSuccess(destID string) {
	st := m.getOrCreateState(destID)
	st.mu.Lock()
	defer st.mu.Unlock()

	st.consecutiveFailures = 0
	st.state = "closed"
	st.halfOpenProbeCount = 0
}

// RecordHalfOpenProbe records that a half-open probe was attempted.
func (m *CircuitBreakerManager) RecordHalfOpenProbe(destID string) {
	st := m.getOrCreateState(destID)
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.state == "half_open" {
		st.halfOpenProbeCount++
	}
}

// GetState returns the current circuit breaker state for debugging.
func (m *CircuitBreakerManager) GetState(destID string) string {
	st := m.getOrCreateState(destID)
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.state
}

func (m *CircuitBreakerManager) getOrCreateState(destID string) *circuitState {
	val, ok := m.states.Load(destID)
	if ok {
		return val.(*circuitState)
	}
	st := &circuitState{state: "closed"}
	actual, _ := m.states.LoadOrStore(destID, st)
	return actual.(*circuitState)
}

// String returns a human-readable representation.
func (c CircuitBreakerConfig) String() string {
	return fmt.Sprintf("CircuitBreakerConfig{threshold=%d, recovery=%s, half_open_max=%d}",
		c.FailureThreshold, c.RecoveryInterval, c.HalfOpenMaxProbe)
}
