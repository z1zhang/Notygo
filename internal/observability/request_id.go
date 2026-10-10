package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// RequestIDMiddleware generates a unique request ID (ULID) for each request.
// If the incoming request has an X-Request-ID header, it is used instead.
// The ID is stored in the request context and set in the response header.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateULID()
		}

		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func generateULID() string {
	entropy := rand.Reader
	ts := ulid.Timestamp(time.Now())
	id, err := ulid.New(ts, entropy)
	if err != nil {
		// Fallback to random hex if ULID generation fails
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		return hex.EncodeToString(b)
	}
	return id.String()
}
