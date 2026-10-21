package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// SendResult holds the outcome of a send operation.
type SendResult struct {
	StatusCode   int    `json:"status_code"`
	Body         string `json:"body"`
	ResponseBody string `json:"response_body"` // alias for Body, used by delivery worker
	IsRetryable  bool   `json:"is_retryable"`
	BusinessCode string `json:"business_code,omitempty"`
	// RequestPayload is the exact request body sent on the wire (JSON text,
	// MIME message, …). Senders fill it so delivery attempts can record the
	// actual payload alongside the pre-send summary.
	RequestPayload string `json:"request_payload,omitempty"`
}

// Sender is the interface that destination senders must implement.
type Sender interface {
	Send(ctx context.Context, destConfig DestinationConfig, msg RenderedMessage) (*SendResult, error)
}

// DestinationConfig holds the resolved configuration for a destination.
type DestinationConfig struct {
	ID        string
	Type      string
	URL       string
	Method    string
	Headers   map[string]string
	Secret    string // Decrypted secret value
	RawConfig map[string]any // Original config from destination model
	Config    json.RawMessage
	SecretRef string
	Timeout   time.Duration
}

// RenderedMessage represents the rendered content to be sent to a destination.
type RenderedMessage struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Format string `json:"format"`
}

// Registry manages registered senders by destination type.
type Registry struct {
	senders map[string]Sender
}

// NewRegistry creates a new empty sender registry.
func NewRegistry() *Registry {
	return &Registry{
		senders: make(map[string]Sender),
	}
}

// Register adds a sender implementation for a given destination type.
func (r *Registry) Register(destType string, sender Sender) {
	r.senders[destType] = sender
}

// Get retrieves the sender for a given destination type.
// Returns the sender and an error if not found (compatible with delivery worker expectations).
func (r *Registry) Get(destType string) (Sender, error) {
	s, ok := r.senders[destType]
	if !ok {
		return nil, fmt.Errorf("no sender registered for destination type %q", destType)
	}
	return s, nil
}
