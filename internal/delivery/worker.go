package delivery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	mathrand "math/rand"
	"sync"
	"time"

	"zpush/internal/config"
	"zpush/internal/destination"
	"zpush/internal/notify"
	"zpush/internal/secret"
	"zpush/internal/sender"
)

// DeliveryWorker processes pending deliveries by claiming them and sending
// notifications to destinations.
type DeliveryWorker struct {
	cfg            config.WorkerConfig
	destRepo       *destination.Repository
	deliveryRepo   *Repository
	senderRegistry *sender.Registry
	circuitBreaker *CircuitBreakerManager
	secretSvc      *secret.Service
	notifier       *notify.Notifier
	workerID       string
}

// NewDeliveryWorker creates a new DeliveryWorker.
func NewDeliveryWorker(
	cfg config.WorkerConfig,
	destRepo *destination.Repository,
	deliveryRepo *Repository,
	senderRegistry *sender.Registry,
	circuitBreaker *CircuitBreakerManager,
	secretSvc *secret.Service,
	notifier *notify.Notifier,
) *DeliveryWorker {
	workerID := generateDeliveryWorkerID()
	return &DeliveryWorker{
		cfg:            cfg,
		destRepo:       destRepo,
		deliveryRepo:   deliveryRepo,
		senderRegistry: senderRegistry,
		circuitBreaker: circuitBreaker,
		secretSvc:      secretSvc,
		notifier:       notifier,
		workerID:       workerID,
	}
}

// Run starts the delivery worker loop. Blocks until context is cancelled.
func (w *DeliveryWorker) Run(ctx context.Context) {
	slog.Info("delivery worker starting", "worker_id", w.workerID, "concurrency", w.cfg.DeliveryWorkerConcurrency)

	// Work channel: buffered to concurrency capacity. When full, the dispatcher
	// blocks on send, providing natural backpressure.
	workCh := make(chan *Delivery, w.cfg.DeliveryWorkerConcurrency)

	// doneCh signals when a worker finishes processing — allows the dispatcher
	// to wake up immediately and claim more deliveries without waiting for the ticker.
	doneCh := make(chan struct{}, w.cfg.DeliveryWorkerConcurrency)

	// Start worker pool — N goroutines reading from workCh
	var wg sync.WaitGroup
	for i := 0; i < w.cfg.DeliveryWorkerConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for d := range workCh {
				func() {
					defer func() {
						if r := recover(); r != nil {
							slog.Error("delivery worker panic", "delivery_id", d.ID, "event_id", d.EventID, "panic", r)
						}
					}()
					w.sendDelivery(ctx, d)
				}()
				// Signal that a slot is free (non-blocking, doneCh is buffered)
				select {
				case doneCh <- struct{}{}:
				default:
				}
			}
		}()
	}

	// Dispatcher: wait for notification, worker-done, or ticker, then batch-claim loop
	ticker := time.NewTicker(w.cfg.ClaimPollInterval)
	defer ticker.Stop()

	notifyCh := w.notifier.DeliveryCh() // nil if notifier is nil

dispatch:
	for {
		// Wait for a signal: notification, worker-done, or ticker fallback
		if notifyCh != nil {
			select {
			case <-ctx.Done():
				break dispatch
			case <-notifyCh:
				// New deliveries arrived, drain coalesced signals
				drainDeliveryChannel(notifyCh)
			case <-doneCh:
				// A worker finished, try to claim more immediately
			case <-ticker.C:
				// Fallback poll
			}
		} else {
			// No notifier: listen on doneCh + ticker
			select {
			case <-ctx.Done():
				break dispatch
			case <-doneCh:
			case <-ticker.C:
			}
		}

		// Batch claim loop: claim deliveries until workCh is full or no more claimable
		for {
			select {
			case <-ctx.Done():
				break dispatch
			default:
			}

			d, err := w.deliveryRepo.ClaimDelivery(ctx, w.workerID, w.cfg.LeaseTimeout)
			if err != nil {
				if err != ErrAlreadyClaimed {
					// Transient DB error — log and retry after brief pause
					slog.Error("claim delivery error, retrying", "error", err)
					select {
					case <-ctx.Done():
						break dispatch
					case <-time.After(200 * time.Millisecond):
						continue // retry the claim immediately
					}
				}
				break // no more deliveries to claim right now
			}

			// Push to workCh. This blocks when the channel is full (backpressure).
			select {
			case workCh <- d:
				// Delivery dispatched to worker pool
			case <-ctx.Done():
				// Context cancelled while waiting for a slot.
				// The delivery is claimed but not dispatched. Lease recovery will handle it.
				break dispatch
			}
		}
	}

	// Graceful shutdown: close workCh so workers drain, then wait
	close(workCh)
	slog.Info("delivery worker stopping, waiting for in-flight tasks", "worker_id", w.workerID)
	wg.Wait()
	slog.Info("delivery worker stopped", "worker_id", w.workerID)
}

// drainDeliveryChannel drains any coalesced signals from a notification channel.
func drainDeliveryChannel(ch <-chan struct{}) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// sendDelivery handles a single delivery attempt.
func (w *DeliveryWorker) sendDelivery(ctx context.Context, d *Delivery) {
	slog.Info("sending delivery", "delivery_id", d.ID, "event_id", d.EventID, "destination_id", d.DestinationID, "attempt", d.AttemptCount)

	// 1. Check circuit breaker
	dest, err := w.destRepo.GetByID(ctx, d.DestinationID)
	if err != nil {
		slog.Error("failed to fetch destination for delivery", "delivery_id", d.ID, "event_id", d.EventID, "destination_id", d.DestinationID, "error", err)
		_ = w.deliveryRepo.UpdateError(ctx, d.ID, "DELIVER_001", "destination not found")
		_ = w.deliveryRepo.UpdateStatus(ctx, d.ID, DeliveryStatusFailed)
		return
	}

	var cbConfig CircuitBreakerConfig
	if dest.CircuitBreaker != nil {
		cbConfig = CircuitBreakerConfig{
			FailureThreshold: dest.CircuitBreaker.FailureThreshold,
			RecoveryInterval: fmt.Sprintf("%ds", dest.CircuitBreaker.TimeoutSeconds),
			HalfOpenMaxProbe: dest.CircuitBreaker.HalfOpenMaxCalls,
		}
	} else {
		// Default circuit breaker config
		cbConfig = CircuitBreakerConfig{
			FailureThreshold: 5,
			RecoveryInterval: "60s",
			HalfOpenMaxProbe: 1,
		}
	}

	if w.circuitBreaker.IsOpen(d.DestinationID, cbConfig) {
		slog.Warn("circuit breaker open for destination", "destination_id", d.DestinationID)
		_ = w.deliveryRepo.ResetToPending(ctx, d.ID)
		w.circuitBreaker.RecordFailure(d.DestinationID, cbConfig)
		return
	}

	// Record half-open probe if applicable
	w.circuitBreaker.RecordHalfOpenProbe(d.DestinationID)

	// 2. Get destination config
	destConfig := buildDestinationConfig(dest, w.secretSvc)

	// 3. Build rendered message
	msg := sender.RenderedMessage{
		Title:  stringValue(d.RenderedTitle),
		Body:   stringValue(d.RenderedBody),
		Format: stringValue(d.RenderedFormat),
	}

	// 4. Get sender
	s, err := w.senderRegistry.Get(string(dest.Type))
	if err != nil {
		slog.Error("no sender for destination type", "type", dest.Type, "error", err)
		_ = w.deliveryRepo.UpdateError(ctx, d.ID, "DELIVER_001", err.Error())
		_ = w.deliveryRepo.UpdateStatus(ctx, d.ID, DeliveryStatusFailed)
		return
	}

	// 5. Send
	startTime := time.Now()
	result, err := s.Send(ctx, destConfig, msg)
	duration := time.Since(startTime)

	// 6. Record delivery attempt
	attempt := &DeliveryAttempt{
		DeliveryID: d.ID,
		AttemptNo:  d.AttemptCount,
		StartedAt:  startTime,
		DurationMs: intPtr(int(duration.Milliseconds())),
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

	// 7. Handle result
	if err != nil {
		// Non-HTTP error (connection, timeout, etc.)
		slog.Error("delivery send error", "delivery_id", d.ID, "event_id", d.EventID, "error", err)
		attempt.ErrorCode = stringPtr("DELIVER_001")
		attempt.ErrorSummary = stringPtr(err.Error())
		_, _ = w.deliveryRepo.RecordDeliveryAttempt(ctx, attempt)

		w.handleRetryableFailure(ctx, d, "DELIVER_001", err.Error())
		w.circuitBreaker.RecordFailure(d.DestinationID, cbConfig)
		return
	}

	if result.StatusCode >= 200 && result.StatusCode < 300 {
		// Success
		slog.Info("delivery succeeded", "delivery_id", d.ID, "event_id", d.EventID, "status_code", result.StatusCode)
		_, _ = w.deliveryRepo.RecordDeliveryAttempt(ctx, attempt)
		_ = w.deliveryRepo.UpdateStatus(ctx, d.ID, DeliveryStatusDelivered)
		w.circuitBreaker.RecordSuccess(d.DestinationID)
		return
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
	_, _ = w.deliveryRepo.RecordDeliveryAttempt(ctx, attempt)

	_ = w.deliveryRepo.UpdateError(ctx, d.ID, errCode, errMsg)

	if result.IsRetryable || isRetryableHTTPStatus(statusCode) {
		w.handleRetryableFailure(ctx, d, errCode, errMsg)
		w.circuitBreaker.RecordFailure(d.DestinationID, cbConfig)
	} else if d.AttemptCount >= d.MaxAttempts {
		// Max attempts reached, non-retryable
		_ = w.deliveryRepo.UpdateStatus(ctx, d.ID, DeliveryStatusFailed)
		w.circuitBreaker.RecordFailure(d.DestinationID, cbConfig)
	} else {
		// Non-retryable error, go to dead letter
		_ = w.deliveryRepo.UpdateStatus(ctx, d.ID, DeliveryStatusFailed)
		w.circuitBreaker.RecordFailure(d.DestinationID, cbConfig)
	}
}

// handleRetryableFailure handles a retryable failure by calculating backoff
// and scheduling the next attempt.
func (w *DeliveryWorker) handleRetryableFailure(ctx context.Context, d *Delivery, errCode, errMsg string) {
	if d.AttemptCount >= d.MaxAttempts {
		slog.Warn("delivery max attempts reached, moving to dead letter", "delivery_id", d.ID, "event_id", d.EventID, "attempts", d.AttemptCount)
		_ = w.deliveryRepo.UpdateStatus(ctx, d.ID, DeliveryStatusFailed)
		return
	}

	backoff := calculateBackoff(d.AttemptCount, w.cfg.RetryBaseDelay, w.cfg.RetryMaxDelay, w.cfg.RetryJitterPercent)
	nextAttempt := time.Now().UTC().Add(backoff)

	slog.Info("delivery retry scheduled", "delivery_id", d.ID, "event_id", d.EventID, "attempt", d.AttemptCount, "backoff", backoff, "next_attempt", nextAttempt)
	_ = w.deliveryRepo.UpdateError(ctx, d.ID, errCode, errMsg)
	_ = w.deliveryRepo.UpdateStatus(ctx, d.ID, DeliveryStatusRetryWait)
	_ = w.deliveryRepo.UpdateNextAttempt(ctx, d.ID, nextAttempt)
}

// calculateBackoff calculates exponential backoff with jitter.
func calculateBackoff(attemptCount int, base, max time.Duration, jitterPercent int) time.Duration {
	delay := base * time.Duration(1<<uint(attemptCount-1)) // 30s, 60s, 120s, ...
	if delay > max {
		delay = max
	}
	jitterRange := float64(delay) * float64(jitterPercent) / 100.0
	jitter := time.Duration((mathrand.Float64()*2 - 1) * jitterRange) // +-20%
	return delay + jitter
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

// buildDestinationConfig builds a sender.DestinationConfig from a destination model.
func buildDestinationConfig(dest *destination.Destination, secretSvc *secret.Service) sender.DestinationConfig {
	cfg := sender.DestinationConfig{
		Type:      string(dest.Type),
		Timeout:   time.Duration(dest.TimeoutMs) * time.Millisecond,
		RawConfig: dest.Config,
	}

	// Marshal the config map to json.RawMessage for sender consumption
	if configBytes, err := json.Marshal(dest.Config); err == nil {
		cfg.Config = configBytes
	}

	// Extract URL and headers from config
	if url, ok := dest.Config["url"].(string); ok {
		cfg.URL = url
	}
	if method, ok := dest.Config["method"].(string); ok {
		cfg.Method = method
	}
	if headers, ok := dest.Config["headers"].(map[string]any); ok {
		cfg.Headers = make(map[string]string)
		for k, v := range headers {
			if s, ok := v.(string); ok {
				cfg.Headers[k] = s
			}
		}
	}

	// Decrypt secret if available
	if dest.SecretRef != nil && *dest.SecretRef != "" && secretSvc != nil {
		decrypted, err := secretSvc.Decrypt(*dest.SecretRef)
		if err == nil {
			cfg.Secret = decrypted
			cfg.SecretRef = decrypted
		}
	}

	return cfg
}

func generateDeliveryWorkerID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
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
