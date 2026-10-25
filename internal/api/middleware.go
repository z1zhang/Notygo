package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"notygo/internal/storage"
)

type authContextKey string

const (
	userIDKey   authContextKey = "user_id"
	usernameKey authContextKey = "username"
)

// GetUsernameFromContext extracts the username from the request context.
// Returns "system" if not found (e.g. internal/system operations).
func GetUsernameFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(usernameKey).(string); ok {
		return v
	}
	return "system"
}

// RecoveryMiddleware catches panics and returns a 500 error.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "error", err, "path", r.URL.Path)
				WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs each request with duration and status.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", r.Header.Get("X-Request-ID"),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// AdminAuthMiddleware validates the admin session token from the Authorization header.
// V1 uses a simple session token stored in the sessions table.
func AdminAuthMiddleware(db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				WriteError(w, http.StatusUnauthorized, ErrIngestAuthFailed, "missing authorization token")
				return
			}

			nowExpr := storage.NowExpr()
			var userID string
			err := db.QueryRow(r.Context(),
				"SELECT user_id FROM notygo_sessions WHERE token_hash = $1 AND expires_at > "+nowExpr,
				hashToken(token),
			).Scan(&userID)

			if err != nil {
				WriteError(w, http.StatusUnauthorized, ErrIngestAuthFailed, "invalid or expired session")
				return
			}

			// Lookup username for audit logging
			var username string
			_ = db.QueryRow(r.Context(),
				"SELECT username FROM notygo_users WHERE id = $1", userID,
			).Scan(&username)

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			ctx = context.WithValue(ctx, usernameKey, username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
