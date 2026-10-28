package channel

import "time"

// ChannelType enumerates supported channel kinds. The type is an open
// extension point (any registered sender key is valid); the source of truth
// for what is actually available is the sender registry wired up in
// cmd/server/main.go. These constants cover the built-in senders.
type ChannelType string

// Built-in channel type values; keys must match the sender registry wired
// up in cmd/server/main.go.
const (
	ChannelTypeDingtalk ChannelType = "dingtalk"
	ChannelTypeFeishu   ChannelType = "feishu"
	ChannelTypeWecom    ChannelType = "wecom"
	ChannelTypeEmail    ChannelType = "email"
	ChannelTypeHTTP     ChannelType = "http"
)

// Channel represents a delivery target.
type Channel struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Type    ChannelType    `json:"type"`
	Enabled bool           `json:"enabled"`
	Config  map[string]any `json:"config"`
	// EncryptedSecret is the AES-GCM ciphertext of the channel credential.
	EncryptedSecret *string   `json:"encrypted_secret,omitempty"`
	TimeoutMs       int       `json:"timeout_ms"`
	MaxAttempts     int       `json:"max_attempts"`
	Revision        int       `json:"revision"`
	CreatedAt       time.Time `json:"created_at,omitzero"`
	UpdatedAt       time.Time `json:"updated_at,omitzero"`
}

// CreateChannelRequest is the DTO for creating a new channel.
type CreateChannelRequest struct {
	Name        *string        `json:"name,omitempty"`
	Type        *ChannelType   `json:"type,omitempty"`
	Enabled     *bool          `json:"enabled,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
	Secret      *string        `json:"secret,omitempty"`
	TimeoutMs   *int           `json:"timeout_ms,omitempty"`
	MaxAttempts *int           `json:"max_attempts,omitempty"`
}

// UpdateChannelRequest is the DTO for updating an existing channel.
// All fields are optional — only non-nil fields will be updated.
type UpdateChannelRequest struct {
	Name        *string        `json:"name,omitempty"`
	Type        *ChannelType   `json:"type,omitempty"`
	Enabled     *bool          `json:"enabled,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
	Secret      *string        `json:"secret,omitempty"`
	TimeoutMs   *int           `json:"timeout_ms,omitempty"`
	MaxAttempts *int           `json:"max_attempts,omitempty"`
}

// ChannelResponse is the DTO returned by the API, with masked secret.
type ChannelResponse struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Type    ChannelType    `json:"type"`
	Enabled bool           `json:"enabled"`
	Config  map[string]any `json:"config"`
	// Secret is display-only: the decrypted value for the edit form
	// (masked by the frontend), or a status label.
	Secret      *string `json:"secret,omitempty"`
	TimeoutMs   int     `json:"timeout_ms"`
	MaxAttempts int     `json:"max_attempts"`
	Revision    int     `json:"revision"`
	CreatedAt   string  `json:"created_at,omitempty"`
	UpdatedAt   string  `json:"updated_at,omitempty"`
}
