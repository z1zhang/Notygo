package destination

import "time"

// DestinationType enumerates supported destination kinds.
type DestinationType string

const (
	DestinationTypeWebhook DestinationType = "webhook"
	DestinationTypeSlack   DestinationType = "slack"
	DestinationTypeEmail   DestinationType = "email"
	DestinationTypeDiscord DestinationType = "discord"
	DestinationTypeCustom  DestinationType = "custom"
)

// Destination represents a delivery target.
type Destination struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Type           DestinationType    `json:"type"`
	Enabled        bool               `json:"enabled"`
	Config         map[string]any     `json:"config"`
	SecretRef      *string            `json:"secret_ref,omitempty"`
	TimeoutMs      int                `json:"timeout_ms"`
	RateLimit      *string            `json:"rate_limit,omitempty"`
	CircuitBreaker *CircuitBreakerConfig `json:"circuit_breaker,omitempty"`
	Revision       int                `json:"revision"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// CircuitBreakerConfig configures circuit breaker behaviour for a destination.
type CircuitBreakerConfig struct {
	FailureThreshold int `json:"failure_threshold"`
	SuccessThreshold int `json:"success_threshold"`
	TimeoutSeconds   int `json:"timeout_seconds"`
	HalfOpenMaxCalls int `json:"half_open_max_calls,omitempty"`
}
