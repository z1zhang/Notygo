package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(data)
	}
}

// WriteError writes a structured error response.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorResponse{
		Error: NewError(code, message),
	})
}

// PaginatedResponse is the standard paginated list response.
type PaginatedResponse[T any] struct {
	Data  []T   `json:"data"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}

// ParsePagination extracts page and size from query parameters.
// Defaults: page=1, size=20. Max size=100.
func ParsePagination(r *http.Request) (page, size int) {
	page = 1
	size = 20

	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	if s := r.URL.Query().Get("size"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 100 {
			size = n
		}
	}
	return
}

// Offset calculates the SQL offset from page and size.
func Offset(page, size int) int {
	return (page - 1) * size
}

// formatTimeOrNil formats a time.Time for JSON responses.
// Returns empty string for zero time (so omitempty omits it).
func formatTimeOrNil(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05.999Z")
}
