package api

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"time"
)

// Healthz returns 200 if the process is alive.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz returns 200 if the database is reachable, 503 otherwise.
func Readyz(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			WriteError(w, http.StatusServiceUnavailable, ErrIngestDBUnavailable, "database unavailable")
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}
