package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"zpush/internal/delivery"
	"zpush/internal/notify"
	"zpush/internal/storage"
)

// ListDeliveries returns a paginated list of deliveries with optional filters.
func ListDeliveries(db *sql.DB) http.HandlerFunc {
	repo := delivery.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		page, size := ParsePagination(r)
		statusFilter := r.URL.Query().Get("status")
		destIDFilter := r.URL.Query().Get("destination_id")
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
func GetDelivery(db *sql.DB) http.HandlerFunc {
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

// RetryDelivery resets a delivery back to pending status for retry.
func RetryDelivery(db *sql.DB, notifier *notify.Notifier) http.HandlerFunc {
	repo := delivery.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		d, err := repo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "delivery not found")
			return
		}

		// Only allow retry for failed or retry_wait deliveries
		if d.Status != delivery.DeliveryStatusFailed && d.Status != delivery.DeliveryStatusRetryWait {
			WriteError(w, http.StatusBadRequest, ErrConfigValidation, "delivery is not in a retryable state")
			return
		}

		// Reset to pending, clear error fields, set next_attempt_at to now
		nowStr := storage.FormatNow()
		nowExpr := storage.NowExpr()
		_, err = db.ExecContext(r.Context(),
			"UPDATE deliveries SET status='pending', last_error_code=NULL, last_error_summary=NULL, next_attempt_at=?, lease_until=NULL, lease_owner=NULL, updated_at="+nowExpr+" WHERE id=?",
			nowStr, id,
		)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to reset delivery")
			return
		}

		// Notify the delivery worker to pick up the retried delivery immediately
		if notifier != nil {
			notifier.NotifyDelivery()
		}

		updated, _ := repo.GetByID(r.Context(), id)
		WriteJSON(w, http.StatusOK, map[string]any{
			"delivery": updated,
			"message":  "delivery queued for retry",
		})
	}
}

// BulkRetry resets multiple deliveries back to pending based on filters.
func BulkRetry(db *sql.DB, notifier *notify.Notifier) http.HandlerFunc {
	repo := delivery.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Status        *string `json:"status,omitempty"`
			DestinationID *string `json:"destination_id,omitempty"`
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
		if req.DestinationID != nil {
			destIDFilter = *req.DestinationID
		}

		count, err := repo.BulkResetPending(r.Context(), statusFilter, destIDFilter)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to bulk retry deliveries")
			return
		}

		// Notify the delivery worker to pick up the reset deliveries
		if notifier != nil {
			notifier.NotifyDelivery()
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"reset_count": count,
			"message":     "deliveries queued for retry",
		})
	}
}
