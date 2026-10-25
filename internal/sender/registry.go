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

// Sender is the interface that channel senders must implement.
type Sender interface {
	Send(ctx context.Context, destConfig ChannelConfig, msg RenderedMessage) (*SendResult, error)
}

// ChannelConfig holds the resolved configuration for a channel.
type ChannelConfig struct {
	ID        string
	Type      string
	URL       string
	Method    string
	Headers   map[string]string
	Secret    string         // Decrypted secret value
	RawConfig map[string]any // Original config from channel model
	Config    json.RawMessage
	Timeout   time.Duration
}

// RenderedMessage represents the rendered content to be sent to a channel.
type RenderedMessage struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Format string `json:"format"`
}

// Registry manages registered senders by channel type.
type Registry struct {
	senders map[string]Sender
}

// NewRegistry creates a new empty sender registry.
func NewRegistry() *Registry {
	return &Registry{
		senders: make(map[string]Sender),
	}
}

// Register adds a sender implementation for a given channel type.
func (r *Registry) Register(channelType string, sender Sender) {
	r.senders[channelType] = sender
}

// Get retrieves the sender for a given channel type.
// Returns the sender and an error if not found (compatible with delivery worker expectations).
func (r *Registry) Get(channelType string) (Sender, error) {
	s, ok := r.senders[channelType]
	if !ok {
		return nil, fmt.Errorf("no sender registered for channel type %q", channelType)
	}
	return s, nil
}
