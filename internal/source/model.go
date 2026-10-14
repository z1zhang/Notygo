package source

import "time"

// Source represents a webhook source in the system.
type Source struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Slug               string    `json:"slug"`
	Enabled            bool      `json:"enabled"`
	AuthType           string    `json:"auth_type"`
	SecretRef          *string   `json:"secret_ref,omitempty"`
	PayloadContentType string    `json:"payload_content_type"`
	MaxBodyBytes       int       `json:"max_body_bytes"`
	RateLimit          *string   `json:"rate_limit,omitempty"`
	MappingID          *string   `json:"mapping_id,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// CreateSourceRequest is the DTO for creating a new source.
type CreateSourceRequest struct {
	Name               string  `json:"name"`
	Slug               string  `json:"slug"`
	Enabled            *bool   `json:"enabled,omitempty"`
	AuthType           *string `json:"auth_type,omitempty"`
	SecretRef          *string `json:"secret_ref,omitempty"`
	PayloadContentType *string `json:"payload_content_type,omitempty"`
	MaxBodyBytes       *int    `json:"max_body_bytes,omitempty"`
	RateLimit          *string `json:"rate_limit,omitempty"`
	MappingID          *string `json:"mapping_id,omitempty"`
}

// UpdateSourceRequest is the DTO for updating an existing source.
type UpdateSourceRequest struct {
	Name               *string `json:"name,omitempty"`
	Slug               *string `json:"slug,omitempty"`
	Enabled            *bool   `json:"enabled,omitempty"`
	AuthType           *string `json:"auth_type,omitempty"`
	SecretRef          *string `json:"secret_ref,omitempty"`
	PayloadContentType *string `json:"payload_content_type,omitempty"`
	MaxBodyBytes       *int    `json:"max_body_bytes,omitempty"`
	RateLimit          *string `json:"rate_limit,omitempty"`
	MappingID          *string `json:"mapping_id,omitempty"`
}

// SourceResponse is the DTO returned by the API.
type SourceResponse struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Slug               string  `json:"slug"`
	Enabled            bool    `json:"enabled"`
	AuthType           string  `json:"auth_type"`
	SecretRef          *string `json:"secret_ref,omitempty"`
	PayloadContentType string  `json:"payload_content_type"`
	MaxBodyBytes       int     `json:"max_body_bytes"`
	RateLimit          *string `json:"rate_limit,omitempty"`
	MappingID          *string `json:"mapping_id,omitempty"`
	CreatedAt          string  `json:"created_at,omitempty"`
	UpdatedAt          string  `json:"updated_at,omitempty"`
}
