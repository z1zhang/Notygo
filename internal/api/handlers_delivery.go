package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"notygo/internal/audit"
	"notygo/internal/delivery"
	"notygo/internal/queue"
)

// ListDeliveries returns a paginated list of deliveries with optional filters.
func ListDeliveries(db *pgxpool.Pool) http.HandlerFunc {
	repo := delivery.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		page, size := ParsePagination(r)
		statusFilter := r.URL.Query().Get("status")
		destIDFilter := r.URL.Query().Get("channel_id")
		eventIDFilter := r.URL.Query().Get("event_id")

		deliveries, total, err := repo.List(r.Context(), page, size, statusFilter, destIDFilter, eventIDFilter)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to list deliveries")
			return
		}

		WriteJSON(w, http.StatusOK, PaginatedResponse[delivery.Delivery]{
			Data:  deliveries,
			Total: total,
			Page:  page,
			Size:  size,
		})
	}
}

// GetDelivery returns a single delivery by ID, including delivery_attempts.
func GetDelivery(db *pgxpool.Pool) http.HandlerFunc {
	repo := delivery.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		d, err := repo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "delivery not found")
			return
		}

		attempts, err := repo.ListDeliveryAttempts(r.Context(), id)
		if err != nil {
			// Return delivery without attempts if fetch fails
			attempts = []delivery.DeliveryAttempt{}
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"delivery":          d,
			"delivery_attempts": attempts,
		})
	}
}

// RetryDelivery resets a failed/retry_wait delivery and re-enqueues its
// queue job, atomically.
func RetryDelivery(db *pgxpool.Pool, q *queue.Manager, auditRepo *audit.Repository) http.HandlerFunc {
	repo := delivery.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := chi.URLParam(r, "id")

		d, err := repo.GetByID(ctx, id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "delivery not found")
			return
		}

		// Only allow retry for failed or retry_wait deliveries
		if d.Status != delivery.DeliveryStatusFailed && d.Status != delivery.DeliveryStatusRetryWait {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "delivery is not in a retryable state")
			return
		}

		// Reset to pending and queue a fresh job in one transaction —
		// a retry without a job would hang forever.
		tx, err := db.Begin(ctx)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to reset delivery")
			return
		}
		reset, err := repo.WithTx(tx).ResetForRetry(ctx, id)
		if err == nil && reset {
			maxAttempts := d.MaxAttempts
			if maxAttempts < 1 {
				maxAttempts = 1
			}
			err = q.EnqueueDeliverySendTx(ctx, tx, id, maxAttempts)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to reset delivery")
			return
		}
		if err := tx.Commit(ctx); err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to reset delivery")
			return
		}

		updated, _ := repo.GetByID(ctx, id)
		WriteJSON(w, http.StatusOK, map[string]any{
			"delivery": updated,
			"message":  "delivery queued for retry",
		})

		// Audit log
		if auditErr := auditRepo.Insert(ctx, &audit.AuditLog{
			Operator:   GetUsernameFromContext(ctx),
			Action:     "retry",
			TargetType: "delivery",
			TargetID:   id,
			Summary:    stringPtr(fmt.Sprintf("retried delivery %s", id)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "retry", "target_type", "delivery", "error", auditErr)
		}
	}
}

// BulkRetry resets multiple failed/retry_wait deliveries and re-enqueues a
// queue job for each, atomically.
func BulkRetry(db *pgxpool.Pool, q *queue.Manager, auditRepo *audit.Repository) http.HandlerFunc {
	repo := delivery.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var req struct {
			Status    *string `json:"status,omitempty"`
			ChannelID *string `json:"channel_id,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "invalid request body")
			return
		}

		statusFilter := ""
		if req.Status != nil {
			statusFilter = *req.Status
		}
		destIDFilter := ""
		if req.ChannelID != nil {
			destIDFilter = *req.ChannelID
		}

		// Only failed or retry_wait deliveries may be retried (dead_letter never)
		if statusFilter != "" &&
			statusFilter != string(delivery.DeliveryStatusFailed) &&
			statusFilter != string(delivery.DeliveryStatusRetryWait) {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "only failed or retry_wait deliveries can be retried")
			return
		}

		// Reset all matching rows and enqueue a job per delivery in one
		// transaction, so the visible count matches the queued jobs exactly.
		tx, err := db.Begin(ctx)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to bulk retry deliveries")
			return
		}
		items, err := repo.WithTx(tx).BulkResetForRetry(ctx, statusFilter, destIDFilter)
		if err == nil {
			for _, it := range items {
				if err = q.EnqueueDeliverySendTx(ctx, tx, it.ID, it.MaxAttempts); err != nil {
					break
				}
			}
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to bulk retry deliveries")
			return
		}
		if err := tx.Commit(ctx); err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to bulk retry deliveries")
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"reset_count": int64(len(items)),
			"message":     "deliveries queued for retry",
		})

		// Audit log
		if auditErr := auditRepo.Insert(ctx, &audit.AuditLog{
			Operator:   GetUsernameFromContext(ctx),
			Action:     "bulk_retry",
			TargetType: "delivery",
			TargetID:   "",
			Summary:    stringPtr(fmt.Sprintf("bulk retried %d deliveries", len(items))),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "bulk_retry", "target_type", "delivery", "error", auditErr)
		}
	}
}
