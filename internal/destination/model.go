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
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	Type           DestinationType       `json:"type"`
	Enabled        bool                  `json:"enabled"`
	Config         map[string]any        `json:"config"`
	SecretRef      *string               `json:"secret_ref,omitempty"`
	TimeoutMs      int                   `json:"timeout_ms"`
	RateLimit      *string               `json:"rate_limit,omitempty"`
	CircuitBreaker *CircuitBreakerConfig `json:"circuit_breaker,omitempty"`
	Revision       int                   `json:"revision"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

// CreateDestinationRequest is the DTO for creating a new destination.
type CreateDestinationRequest struct {
	Name           *string               `json:"name,omitempty"`
	Type           *DestinationType      `json:"type,omitempty"`
	Enabled        *bool                 `json:"enabled,omitempty"`
	Config         map[string]any        `json:"config,omitempty"`
	SecretRef      *string               `json:"secret_ref,omitempty"`
	TimeoutMs      *int                  `json:"timeout_ms,omitempty"`
	RateLimit      *string               `json:"rate_limit,omitempty"`
	CircuitBreaker *CircuitBreakerConfig `json:"circuit_breaker,omitempty"`
}

// UpdateDestinationRequest is the DTO for updating an existing destination.
// All fields are optional — only non-nil fields will be updated.
type UpdateDestinationRequest struct {
	Name           *string               `json:"name,omitempty"`
	Type           *DestinationType      `json:"type,omitempty"`
	Enabled        *bool                 `json:"enabled,omitempty"`
	Config         map[string]any        `json:"config,omitempty"`
	SecretRef      *string               `json:"secret_ref,omitempty"`
	TimeoutMs      *int                  `json:"timeout_ms,omitempty"`
	RateLimit      *string               `json:"rate_limit,omitempty"`
	CircuitBreaker *CircuitBreakerConfig `json:"circuit_breaker,omitempty"`
}

// DestinationResponse is the DTO returned by the API, with masked secret.
type DestinationResponse struct {
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	Type           DestinationType       `json:"type"`
	Enabled        bool                  `json:"enabled"`
	Config         map[string]any        `json:"config"`
	SecretRef      *string               `json:"secret_ref,omitempty"`
	TimeoutMs      int                   `json:"timeout_ms"`
	RateLimit      *string               `json:"rate_limit,omitempty"`
	CircuitBreaker *CircuitBreakerConfig `json:"circuit_breaker,omitempty"`
	Revision       int                   `json:"revision"`
	CreatedAt      string                `json:"created_at"`
	UpdatedAt      string                `json:"updated_at"`
}

// CircuitBreakerConfig configures circuit breaker behaviour for a destination.
type CircuitBreakerConfig struct {
	FailureThreshold int `json:"failure_threshold"`
	SuccessThreshold int `json:"success_threshold"`
	TimeoutSeconds   int `json:"timeout_seconds"`
	HalfOpenMaxCalls int `json:"half_open_max_calls,omitempty"`
}
