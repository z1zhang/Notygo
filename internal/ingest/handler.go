package ingest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oklog/ulid/v2"

	"zPush/internal/observability"
	"zPush/internal/secret"
	"zPush/internal/source"
	"zPush/internal/storage"
)

// Ingester handles incoming webhook ingestion requests.
type Ingester struct {
	db           *sql.DB
	sourceRepo   *source.Repository
	secretSvc    *secret.Service
	rateLimiter  *RateLimiter
	maxBodyBytes int
}

// NewIngester creates a new Ingester.
func NewIngester(db *sql.DB, sourceRepo *source.Repository, secretSvc *secret.Service, rl *RateLimiter, maxBodyBytes int) *Ingester {
	return &Ingester{
		db:           db,
		sourceRepo:   sourceRepo,
		secretSvc:    secretSvc,
		rateLimiter:  rl,
		maxBodyBytes: maxBodyBytes,
	}
}

// ingestResponse is the JSON response returned on successful ingestion.
type ingestResponse struct {
	EventID string `json:"event_id"`
	Status  string `json:"status"`
}

// HandleIngest processes an incoming webhook request.
func (h *Ingester) HandleIngest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Extract source slug from URL
	slug := chi.URLParam(r, "source_slug")
	if slug == "" {
		writeIngestError(w, http.StatusBadRequest, "INGEST_005", "missing source_slug in URL path")
		return
	}

	// 2. Lookup source by slug
	src, err := h.sourceRepo.GetBySlug(ctx, slug)
	if err != nil {
		if err == source.ErrNotFound {
			writeIngestError(w, http.StatusNotFound, "INGEST_005", fmt.Sprintf("source %q not found", slug))
			return
		}
		writeIngestError(w, http.StatusInternalServerError, "INGEST_006", "database unavailable")
		return
	}

	// 3. Check source is enabled
	if !src.Enabled {
		writeIngestError(w, http.StatusForbidden, "INGEST_005", fmt.Sprintf("source %q is disabled", slug))
		return
	}

	// 4. Validate auth
	if src.AuthType == "bearer" {
		if err := h.validateBearerAuth(ctx, src, r); err != nil {
			writeIngestError(w, http.StatusUnauthorized, "INGEST_001", "authentication failed")
			return
		}
	}

	// 5. Check rate limit
	sourceRateLimit := ""
	if src.RateLimit != nil {
		sourceRateLimit = *src.RateLimit
	}
	if !h.rateLimiter.Allow(slug, sourceRateLimit) {
		writeIngestError(w, http.StatusTooManyRequests, "INGEST_004", "rate limit exceeded")
		return
	}

	// 6. Read body with size limit
	maxBytes := h.maxBodyBytes
	if src.MaxBodyBytes > 0 && src.MaxBodyBytes < maxBytes {
		maxBytes = src.MaxBodyBytes
	}
	lr := io.LimitReader(r.Body, int64(maxBytes)+1)
	body, err := io.ReadAll(lr)
	if err != nil {
		writeIngestError(w, http.StatusBadRequest, "INGEST_003", "failed to read request body")
		return
	}
	if len(body) > maxBytes {
		writeIngestError(w, http.StatusRequestEntityTooLarge, "INGEST_002",
			fmt.Sprintf("payload exceeds maximum size of %d bytes", maxBytes))
		return
	}

	// 7. Validate JSON
	if !json.Valid(body) {
		writeIngestError(w, http.StatusBadRequest, "INGEST_003", "invalid JSON payload")
		return
	}

	// 8. Compute SHA-256 hash
	hash := sha256.Sum256(body)
	payloadHash := fmt.Sprintf("%x", hash[:])

	// 9. Check for duplicate source_event_id (from X-Delivery-Id header)
	deliveryID := r.Header.Get("X-Delivery-Id")
	if deliveryID != "" {
		isDup, err := h.sourceRepo.CheckDuplicateSourceEventID(ctx, src.ID, deliveryID)
		if err != nil {
			// Log but don't block on duplicate check failure
			// Continue processing
			_ = err
		} else if isDup {
			writeIngestError(w, http.StatusConflict, "INGEST_007", "duplicate event")
			return
		}
	}

	// 10. Generate ULID for event ID
	eventID := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()

	// 11. Get request ID from context
	requestID := observability.GetRequestID(ctx)

	// 12. INSERT INTO events
	receivedAt := storage.FormatNow()

	var sourceEventID *string
	if deliveryID != "" {
		sourceEventID = &deliveryID
	}

	_, err = h.db.ExecContext(ctx,
		`INSERT INTO events (id, source_id, source_event_id, request_id, received_at, content_type, payload_raw, payload_hash, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		eventID, src.ID, sourceEventID, requestID, receivedAt, src.PayloadContentType, string(body), payloadHash, "received",
	)
	if err != nil {
		writeIngestError(w, http.StatusInternalServerError, "INGEST_006", "failed to store event")
		return
	}

	// 13. Return 202 Accepted
	resp := ingestResponse{
		EventID: eventID,
		Status:  "accepted",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}

// validateBearerAuth checks the Authorization: Bearer header against the source's decrypted secret.
func (h *Ingester) validateBearerAuth(ctx context.Context, src *source.Source, r *http.Request) error {
	if src.SecretRef == nil {
		return fmt.Errorf("bearer auth required but no secret configured")
	}

	// Decrypt the stored secret
	decrypted, err := h.secretSvc.Decrypt(*src.SecretRef)
	if err != nil {
		return fmt.Errorf("failed to decrypt source secret: %w", err)
	}

	// Extract Bearer token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return fmt.Errorf("missing Bearer token")
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	if token != decrypted {
		return fmt.Errorf("invalid bearer token")
	}

	return nil
}

// writeIngestError writes a structured error response for ingest endpoints.
func writeIngestError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
