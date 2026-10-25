package endpoint

import "time"

// Endpoint represents a webhook endpoint in the system.
type Endpoint struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Enabled  bool   `json:"enabled"`
	AuthType string `json:"auth_type"`
	// EncryptedSecret is the AES-GCM ciphertext of the endpoint's bearer
	// token; the plaintext only ever exists in memory at ingest time.
	EncryptedSecret    *string   `json:"encrypted_secret,omitempty"`
	PayloadContentType string    `json:"payload_content_type"`
	MaxBodyBytes       int       `json:"max_body_bytes"`
	RateLimit          *string   `json:"rate_limit,omitempty"`
	MappingID          *string   `json:"mapping_id,omitempty"`
	CreatedAt          time.Time `json:"created_at,omitempty"`
	UpdatedAt          time.Time `json:"updated_at,omitempty"`
}

// CreateEndpointRequest is the DTO for creating a new endpoint.
type CreateEndpointRequest struct {
	Name               string  `json:"name"`
	Slug               string  `json:"slug"`
	Enabled            *bool   `json:"enabled,omitempty"`
	AuthType           *string `json:"auth_type,omitempty"`
	Secret             *string `json:"secret,omitempty"`
	PayloadContentType *string `json:"payload_content_type,omitempty"`
	MaxBodyBytes       *int    `json:"max_body_bytes,omitempty"`
	RateLimit          *string `json:"rate_limit,omitempty"`
	MappingID          *string `json:"mapping_id,omitempty"`
}

// UpdateEndpointRequest is the DTO for updating an existing endpoint.
type UpdateEndpointRequest struct {
	Name               *string `json:"name,omitempty"`
	Slug               *string `json:"slug,omitempty"`
	Enabled            *bool   `json:"enabled,omitempty"`
	AuthType           *string `json:"auth_type,omitempty"`
	Secret             *string `json:"secret,omitempty"`
	PayloadContentType *string `json:"payload_content_type,omitempty"`
	MaxBodyBytes       *int    `json:"max_body_bytes,omitempty"`
	RateLimit          *string `json:"rate_limit,omitempty"`
	MappingID          *string `json:"mapping_id,omitempty"`
}

// EndpointResponse is the DTO returned by the API.
type EndpointResponse struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Slug               string  `json:"slug"`
	Enabled            bool    `json:"enabled"`
	AuthType           string  `json:"auth_type"`
	Secret             *string `json:"secret,omitempty"`
	PayloadContentType string  `json:"payload_content_type"`
	MaxBodyBytes       int     `json:"max_body_bytes"`
	RateLimit          *string `json:"rate_limit,omitempty"`
	MappingID          *string `json:"mapping_id,omitempty"`
	CreatedAt          string  `json:"created_at,omitempty"`
	UpdatedAt          string  `json:"updated_at,omitempty"`
}
