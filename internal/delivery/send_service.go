package delivery

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"notygo/internal/backoff"
	"notygo/internal/channel"
	"notygo/internal/secret"
	"notygo/internal/sender"
)

// SendService executes single delivery send attempts on behalf of the River
// delivery_send worker. The retry dichotomy lives here:
//
//   - permanent failures (non-retryable HTTP status, DNS/TLS/credential
//     problems, missing channel/sender) mark the delivery failed and return
//     permanent=true — the queue cancels the job, no retries
//   - transient failures (timeout, connection reset, 429/5xx) mark the
//     delivery retry_wait and return the error — the queue schedules the
//     next attempt per its retry policy, until the attempt budget
//     (max_attempts) is exhausted
type SendService struct {
	pool           *pgxpool.Pool
	destRepo       *channel.Repository
	deliveryRepo   *Repository
	senderRegistry *sender.Registry
	secretSvc      *secret.Service
	retryCfg       backoff.Config
}

// NewSendService creates a SendService.
func NewSendService(
	pool *pgxpool.Pool,
	destRepo *channel.Repository,
	deliveryRepo *Repository,
	senderRegistry *sender.Registry,
	secretSvc *secret.Service,
	retryCfg backoff.Config,
) *SendService {
	return &SendService{
		pool:           pool,
		destRepo:       destRepo,
		deliveryRepo:   deliveryRepo,
		senderRegistry: senderRegistry,
		secretSvc:      secretSvc,
		retryCfg:       retryCfg,
	}
}

// Execute runs one send attempt for the delivery as attempt number attemptNo
// (the River job's attempt count). The attempt record and status transition
// are written in one transaction before the outcome is reported.
//
// Returns (permanent, err):
//   - (false, nil): delivered, or nothing to do (terminal/duplicate job)
//   - (true, err):  non-retryable failure — the job should be cancelled
//   - (false, err): retryable failure — the job should retry; the attempt
//     budget is enforced by the job's MaxAttempts
func (s *SendService) Execute(ctx context.Context, deliveryID string, attemptNo int) (bool, error) {
	d, err := s.deliveryRepo.GetByID(ctx, deliveryID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil // delivery deleted; nothing to send
		}
		return false, fmt.Errorf("load delivery %s: %w", deliveryID, err)
	}

	// Claim guard: pending/retry_wait → processing. Terminal statuses and
	// deliveries owned by another job no-op.
	claimed, err := s.deliveryRepo.MarkAttemptInProgress(ctx, d.ID, attemptNo)
	if err != nil {
		return false, fmt.Errorf("claim delivery %s: %w", d.ID, err)
	}
	if !claimed {
		return false, nil
	}

	slog.Info("sending delivery", "delivery_id", d.ID, "event_id", d.EventID, "channel_id", d.ChannelID, "attempt", attemptNo)

	// 1. Fetch channel
	dest, err := s.destRepo.GetByID(ctx, d.ChannelID)
	if err != nil {
		slog.Error("failed to fetch channel for delivery", "delivery_id", d.ID, "event_id", d.EventID, "channel_id", d.ChannelID, "error", err)
		return true, s.finishFailed(ctx, d.ID, "DELIVER_001", "channel not found")
	}

	// 2. Get channel config
	destConfig := channel.BuildSenderConfig(dest, s.secretSvc)

	// 3. Build rendered message
	msg := sender.RenderedMessage{
		Title:  stringValue(d.RenderedTitle),
		Body:   stringValue(d.RenderedBody),
		Format: stringValue(d.RenderedFormat),
	}

	// 4. Get sender
	snd, err := s.senderRegistry.Get(string(dest.Type))
	if err != nil {
		slog.Error("no sender for channel type", "type", dest.Type, "error", err)
		return true, s.finishFailed(ctx, d.ID, "DELIVER_001", err.Error())
	}

	// 5. Build request summary for audit/debug
	requestSummary := buildRequestSummary(destConfig, msg)

	// 6. Send
	startTime := time.Now().UTC()
	result, sendErr := snd.Send(ctx, destConfig, msg)
	duration := time.Since(startTime)

	// 6b. Record the exact request body the sender put on the wire
	// (SendResult.RequestPayload), merged into the pre-send summary.
	if result != nil && result.RequestPayload != "" {
		requestSummary = attachRequestPayload(requestSummary, result.RequestPayload)
	}

	// 7. Record delivery attempt
	attempt := &DeliveryAttempt{
		DeliveryID:     d.ID,
		AttemptNo:      attemptNo,
		StartedAt:      startTime,
		DurationMs:     intPtr(int(duration.Milliseconds())),
		RequestSummary: &requestSummary,
	}

	if result != nil {
		attempt.ResponseStatus = &result.StatusCode
		respBody := result.Body
		if respBody == "" {
			respBody = result.ResponseBody
		}
		attempt.ResponseSummary = &respBody
		finished := startTime.Add(duration)
		attempt.FinishedAt = &finished
	}

	// 8. Handle result
	if sendErr != nil {
		// Non-HTTP error (connection, timeout, SSRF guard, …)
		slog.Error("delivery send error", "delivery_id", d.ID, "event_id", d.EventID, "error", sendErr)
		attempt.ErrorCode = stringPtr("DELIVER_001")
		attempt.ErrorSummary = stringPtr(sendErr.Error())
		if _, err := s.deliveryRepo.RecordDeliveryAttempt(ctx, attempt); err != nil {
			return false, fmt.Errorf("record attempt: %w", err)
		}
		return s.handleFailure(ctx, d, attemptNo, isPermanentNetworkError(sendErr), "DELIVER_001", sendErr.Error())
	}

	if result.StatusCode >= 200 && result.StatusCode < 300 {
		// Success
		slog.Info("delivery succeeded", "delivery_id", d.ID, "event_id", d.EventID, "status_code", result.StatusCode)
		if _, err := s.deliveryRepo.RecordDeliveryAttempt(ctx, attempt); err != nil {
			return false, fmt.Errorf("record attempt: %w", err)
		}
		if err := s.deliveryRepo.MarkDelivered(ctx, d.ID); err != nil {
			return false, fmt.Errorf("mark delivered: %w", err)
		}
		return false, nil
	}

	// Non-2xx response
	statusCode := result.StatusCode
	errCode := classifyHTTPError(statusCode)
	respBody := result.Body
	if respBody == "" {
		respBody = result.ResponseBody
	}
	errMsg := fmt.Sprintf("HTTP %d: %s", statusCode, respBody)
	attempt.ErrorCode = stringPtr(errCode)
	attempt.ErrorSummary = stringPtr(errMsg)
	if _, err := s.deliveryRepo.RecordDeliveryAttempt(ctx, attempt); err != nil {
		return false, fmt.Errorf("record attempt: %w", err)
	}

	retryable := result.IsRetryable || isRetryableHTTPStatus(statusCode)
	return s.handleFailure(ctx, d, attemptNo, !retryable, errCode, errMsg)
}

// handleFailure records a failed attempt and decides the retry outcome.
// permanent=true marks the delivery failed (no more attempts). A transient
// failure within the budget marks retry_wait and returns the error for the
// queue to retry; at the budget's end the delivery is marked failed.
func (s *SendService) handleFailure(ctx context.Context, d *Delivery, attemptNo int, permanent bool, errCode, errMsg string) (bool, error) {
	if permanent {
		slog.Warn("delivery failed permanently", "delivery_id", d.ID, "event_id", d.EventID, "error_code", errCode)
		return true, s.finishFailed(ctx, d.ID, errCode, errMsg)
	}

	if attemptNo >= d.MaxAttempts {
		slog.Warn("delivery max attempts reached, moving to dead letter", "delivery_id", d.ID, "event_id", d.EventID, "attempts", attemptNo)
		return true, s.finishFailed(ctx, d.ID, errCode, errMsg)
	}

	// Display-only prediction of the queue's next attempt, computed with the
	// same backoff curve the queue retry policy uses.
	nextAttempt := time.Now().UTC().Add(backoff.Delay(attemptNo+1, s.retryCfg))
	slog.Info("delivery retry scheduled", "delivery_id", d.ID, "event_id", d.EventID, "attempt", attemptNo, "next_attempt", nextAttempt)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	deliveryTx := s.deliveryRepo.WithTx(tx)
	if err := deliveryTx.MarkRetryWait(ctx, d.ID, nextAttempt, errCode, errMsg); err != nil {
		return false, fmt.Errorf("mark retry_wait: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}

	return false, fmt.Errorf("%s: %s", errCode, errMsg) // transient — let the queue retry
}

// finishFailed marks the delivery failed with its final error.
func (s *SendService) finishFailed(ctx context.Context, deliveryID, errCode, errMsg string) error {
	if err := s.deliveryRepo.MarkFailed(ctx, deliveryID, errCode, errMsg); err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	return fmt.Errorf("%s: %s", errCode, errMsg)
}

// isPermanentNetworkError classifies non-HTTP send errors. DNS resolution
// failures (unknown host) and TLS certificate problems are permanent —
// retrying will not fix them. Everything else (timeouts, resets, refused)
// is transient; unclassified errors default to transient (宁多试不漏发).
func isPermanentNetworkError(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		return true
	}
	var unknownAuthorityErr x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthorityErr) {
		return true
	}
	var certInvalidErr x509.CertificateInvalidError
	if errors.As(err, &certInvalidErr) {
		return true
	}
	return false
}

// isRetryableHTTPStatus returns true for status codes that warrant a retry.
func isRetryableHTTPStatus(statusCode int) bool {
	switch statusCode {
	case 429, 502, 503, 504:
		return true
	default:
		return false
	}
}

// classifyHTTPError returns an error code for a given HTTP status.
func classifyHTTPError(statusCode int) string {
	switch {
	case statusCode == 429:
		return "DELIVER_005"
	case statusCode == 401, statusCode == 403:
		return "DELIVER_002"
	case statusCode >= 500:
		return "DELIVER_001"
	default:
		return "DELIVER_004"
	}
}

// buildRequestSummary creates a JSON summary of the request that will be sent,
// suitable for storing in delivery_attempts.request_summary.
// Sensitive headers (auth, token, key, secret) are redacted.
// Empty method/url are omitted (chat senders like dingtalk have neither).
func buildRequestSummary(cfg sender.ChannelConfig, msg sender.RenderedMessage) string {
	summary := map[string]any{
		"type": cfg.Type,
	}
	if cfg.Method != "" {
		summary["method"] = cfg.Method
	}
	if cfg.URL != "" {
		summary["url"] = cfg.URL
	}
	if len(cfg.Headers) > 0 {
		redacted := make(map[string]string)
		for k, v := range cfg.Headers {
			lower := strings.ToLower(k)
			if strings.Contains(lower, "auth") || strings.Contains(lower, "key") ||
				strings.Contains(lower, "token") || strings.Contains(lower, "secret") {
				redacted[k] = "***"
			} else {
				redacted[k] = v
			}
		}
		summary["headers"] = redacted
	}
	titleSnippet := msg.Title
	if len(titleSnippet) > 128 {
		titleSnippet = titleSnippet[:128] + "...(truncated)"
	}
	if titleSnippet != "" {
		summary["title_snippet"] = titleSnippet
	}
	// Always record the body snippet (even when empty) so empty-content
	// deliveries are visible in the logs.
	bodySnippet := msg.Body
	if len(bodySnippet) > 512 {
		bodySnippet = bodySnippet[:512] + "...(truncated)"
	}
	summary["body_snippet"] = bodySnippet
	b, _ := json.Marshal(summary)
	return string(b)
}

// attachRequestPayload merges the exact request body the sender put on the
// wire into the pre-built request summary JSON (truncated).
func attachRequestPayload(summary, payload string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(summary), &m); err != nil {
		return summary
	}
	if len(payload) > 2048 {
		payload = payload[:2048] + "...(truncated)"
	}
	m["payload"] = payload
	b, err := json.Marshal(m)
	if err != nil {
		return summary
	}
	return string(b)
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}
