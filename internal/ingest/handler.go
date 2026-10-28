package ingest

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"notygo/internal/errcode"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oklog/ulid/v2"

	"github.com/jackc/pgx/v5/pgxpool"
	"notygo/internal/endpoint"
	"notygo/internal/event"
	"notygo/internal/observability"
	"notygo/internal/queue"
	"notygo/internal/secret"
)

// Ingester handles incoming webhook ingestion requests.
type Ingester struct {
	db           *pgxpool.Pool
	endpointRepo *endpoint.Repository
	secretSvc    *secret.Service
	rateLimiter  *RateLimiter
	maxBodyBytes int
	queue        *queue.Manager
}

// NewIngester creates a new Ingester.
func NewIngester(db *pgxpool.Pool, endpointRepo *endpoint.Repository, secretSvc *secret.Service, rl *RateLimiter, maxBodyBytes int, q *queue.Manager) *Ingester {
	return &Ingester{
		db:           db,
		endpointRepo: endpointRepo,
		secretSvc:    secretSvc,
		rateLimiter:  rl,
		maxBodyBytes: maxBodyBytes,
		queue:        q,
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

	// 1. Extract endpoint slug from URL
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		writeIngestError(w, r, http.StatusBadRequest, errcode.IngestEndpointMissing, "missing slug in URL path")
		return
	}

	// 2. Lookup endpoint by slug
	src, err := h.endpointRepo.GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, endpoint.ErrNotFound) {
			writeIngestError(w, r, http.StatusNotFound, errcode.IngestEndpointMissing, fmt.Sprintf("endpoint %q not found", slug))
			return
		}
		writeIngestError(w, r, http.StatusInternalServerError, errcode.IngestDBUnavailable, "database unavailable")
		return
	}

	// 3. Check endpoint is enabled
	if !src.Enabled {
		writeIngestError(w, r, http.StatusForbidden, errcode.IngestEndpointMissing, fmt.Sprintf("endpoint %q is disabled", slug))
		return
	}

	// 4. Check rate limit
	endpointRateLimit := ""
	if src.RateLimit != nil {
		endpointRateLimit = *src.RateLimit
	}
	if !h.rateLimiter.Allow(slug, endpointRateLimit) {
		writeIngestError(w, r, http.StatusTooManyRequests, errcode.IngestRateLimited, "rate limit exceeded")
		return
	}

	// 5. Read body with size limit (must precede auth: HMAC signs the raw body)
	maxBytes := h.maxBodyBytes
	if src.MaxBodyBytes > 0 && src.MaxBodyBytes < maxBytes {
		maxBytes = src.MaxBodyBytes
	}
	lr := io.LimitReader(r.Body, int64(maxBytes)+1)
	body, err := io.ReadAll(lr)
	if err != nil {
		writeIngestError(w, r, http.StatusBadRequest, errcode.IngestInvalidJSON, "failed to read request body")
		return
	}
	if len(body) > maxBytes {
		writeIngestError(w, r, http.StatusRequestEntityTooLarge, errcode.IngestBodyTooLarge,
			fmt.Sprintf("payload exceeds maximum size of %d bytes", maxBytes))
		return
	}

	// 6. Validate auth
	switch src.AuthType {
	case "bearer":
		if err := h.validateBearerAuth(ctx, src, r); err != nil {
			writeIngestError(w, r, http.StatusUnauthorized, errcode.IngestAuthFailed, "authentication failed")
			return
		}
	case "hmac":
		if err := h.validateHMACAuth(ctx, src, r, body); err != nil {
			writeIngestError(w, r, http.StatusUnauthorized, errcode.IngestAuthFailed, "authentication failed")
			return
		}
	}

	// 7. Validate JSON
	if !json.Valid(body) {
		writeIngestError(w, r, http.StatusBadRequest, errcode.IngestInvalidJSON, "invalid JSON payload")
		return
	}

	// 8. Compute SHA-256 hash
	hash := sha256.Sum256(body)
	payloadHash := fmt.Sprintf("%x", hash[:])

	// 9. Check for duplicate upstream_event_id (from X-Delivery-Id header)
	deliveryID := r.Header.Get("X-Delivery-Id")
	if deliveryID != "" {
		isDup, err := h.endpointRepo.CheckDuplicateUpstreamEventID(ctx, src.ID, deliveryID)
		if err != nil {
			// A failed duplicate check must not block ingestion (the event
			// is still valuable), but dedup protection silently degrades —
			// log it so the degradation is visible to operators.
			slog.Warn("duplicate check failed, continuing without dedup",
				"endpoint_id", src.ID, "upstream_event_id", deliveryID, "error", err)
		} else if isDup {
			writeIngestError(w, r, http.StatusConflict, errcode.IngestDuplicate, "duplicate event")
			return
		}
	}

	// 10. Generate ULID for event ID
	eventID := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()

	// 11. Get request ID from context
	requestID := observability.GetRequestID(ctx)

	// 12. INSERT INTO notygo_event + queue the processing job in one transaction:
	// the event row and its event_process job are atomic — a crash in between
	// can never leave an unprocessed event behind.
	receivedAt := time.Now().UTC()
	statusCode, err := event.StatusToCode(event.EventStatusReceived)
	if err != nil {
		writeIngestError(w, r, http.StatusInternalServerError, errcode.IngestDBUnavailable, "failed to store event")
		return
	}
	runTypeCode, err := event.RunTypeToCode(event.EventRunTypeInitial)
	if err != nil {
		writeIngestError(w, r, http.StatusInternalServerError, errcode.IngestDBUnavailable, "failed to store event")
		return
	}

	var upstreamEventID *string
	if deliveryID != "" {
		upstreamEventID = &deliveryID
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		writeIngestError(w, r, http.StatusInternalServerError, errcode.IngestDBUnavailable, "failed to store event")
		return
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO notygo_event (id, endpoint_id, upstream_event_id, request_id, received_at, content_type, payload_raw, payload_hash, status, run_type)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		eventID, src.ID, upstreamEventID, requestID, receivedAt, src.PayloadContentType, string(body), payloadHash, statusCode, runTypeCode,
	)
	if err == nil {
		err = h.queue.EnqueueEventProcessTx(ctx, tx, eventID)
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		writeIngestError(w, r, http.StatusInternalServerError, errcode.IngestDBUnavailable, "failed to store event")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeIngestError(w, r, http.StatusInternalServerError, errcode.IngestDBUnavailable, "failed to store event")
		return
	}

	// 13. Return 202 Accepted
	resp := ingestResponse{
		EventID: eventID,
		Status:  "accepted",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	// Status is committed; encode failure has nowhere to go.
	_ = json.NewEncoder(w).Encode(resp)
}

// validateBearerAuth checks the Authorization: Bearer header against the endpoint's decrypted secret.
func (h *Ingester) validateBearerAuth(ctx context.Context, src *endpoint.Endpoint, r *http.Request) error {
	if src.EncryptedSecret == nil {
		return fmt.Errorf("bearer auth required but no secret configured")
	}

	// Decrypt the stored secret
	decrypted, err := h.secretSvc.Decrypt(*src.EncryptedSecret)
	if err != nil {
		return fmt.Errorf("failed to decrypt endpoint secret: %w", err)
	}

	// Extract Bearer token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return fmt.Errorf("missing Bearer token")
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Constant-time comparison prevents leaking the secret via timing
	if subtle.ConstantTimeCompare([]byte(token), []byte(decrypted)) != 1 {
		return fmt.Errorf("invalid bearer token")
	}

	return nil
}

// validateHMACAuth checks the X-Signature header against an HMAC-SHA256 of the
// raw request body, keyed by the endpoint's decrypted secret. The signature is
// hex-encoded and may carry a "sha256=" prefix (GitHub webhook style).
func (h *Ingester) validateHMACAuth(ctx context.Context, src *endpoint.Endpoint, r *http.Request, body []byte) error {
	if src.EncryptedSecret == nil {
		return fmt.Errorf("hmac auth required but no secret configured")
	}

	// Decrypt the stored secret
	decrypted, err := h.secretSvc.Decrypt(*src.EncryptedSecret)
	if err != nil {
		return fmt.Errorf("failed to decrypt endpoint secret: %w", err)
	}

	sig := strings.TrimPrefix(r.Header.Get("X-Signature"), "sha256=")
	if sig == "" {
		return fmt.Errorf("missing X-Signature header")
	}
	given, err := hex.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("invalid signature encoding")
	}

	// hmac.Equal is constant-time, preventing the secret from leaking via timing
	mac := hmac.New(sha256.New, []byte(decrypted))
	mac.Write(body)
	if !hmac.Equal(mac.Sum(nil), given) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// writeIngestError writes a structured error response for ingest endpoints.
func writeIngestError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	// Server-side record by severity: 5xx are ours to fix (error), 401/429
	// signal probes or abuse worth alerting on (warn). Plain 400s are client
	// mistakes in untrusted webhook traffic — too noisy to log per-request,
	// and the request-access log already carries their status.
	switch {
	case status >= 500:
		slog.Error("ingest request failed",
			"code", code, "message", message, "request_id", observability.GetRequestID(r.Context()))
	case status == http.StatusUnauthorized, status == http.StatusForbidden, status == http.StatusTooManyRequests:
		slog.Warn("ingest rejected",
			"code", code, "message", message,
			"request_id", observability.GetRequestID(r.Context()), "remote_ip", r.RemoteAddr)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Status is committed; encode failure has nowhere to go.
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
