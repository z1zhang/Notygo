package api

// Error codes (DOMAIN_NNN) live in internal/errcode — the single registry
// shared by every layer, so ingest/processor/delivery can emit codes without
// importing this package (which would be an import cycle). See that package
// for the code list and the registry policy.

// Error represents a structured API error.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error Error `json:"error"`
}

// NewError creates a new Error with the given code and message.
func NewError(code, message string) Error {
	return Error{Code: code, Message: message}
}
