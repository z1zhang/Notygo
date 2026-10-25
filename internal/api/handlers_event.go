package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"notygo/internal/audit"
	"notygo/internal/event"
	"notygo/internal/queue"
)

// ListEvents returns a paginated list of events, with optional status filter.
func ListEvents(db *pgxpool.Pool) http.HandlerFunc {
	repo := event.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		page, size := ParsePagination(r)
		statusFilter := r.URL.Query().Get("status")
		endpointIDFilter := r.URL.Query().Get("endpoint_id")

		events, total, err := repo.List(r.Context(), page, size, statusFilter, endpointIDFilter)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to list events")
			return
		}

		WriteJSON(w, http.StatusOK, PaginatedResponse[event.Event]{
			Data:  events,
			Total: total,
			Page:  page,
			Size:  size,
		})
	}
}

// GetEvent returns a single event by ID, including parsed_fields from the latest EventRun.
func GetEvent(db *pgxpool.Pool) http.HandlerFunc {
	repo := event.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		evt, err := repo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "event not found")
			return
		}

		// Join with latest EventRun to include parsed_fields
		run, err := repo.GetLatestEventRun(r.Context(), id)
		if err != nil {
			// No event run yet, return event without run data
			WriteJSON(w, http.StatusOK, map[string]any{
				"event":      evt,
				"latest_run": nil,
			})
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"event":      evt,
			"latest_run": run,
		})
	}
}

// ReplayEvent resets the event status to 'received' and re-enqueues its
// processing job. The processor will create a new EventRun with
// run_type='replay' when it processes the event.
func ReplayEvent(db *pgxpool.Pool, q *queue.Manager, auditRepo *audit.Repository) http.HandlerFunc {
	repo := event.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := chi.URLParam(r, "id")
		evt, err := repo.GetByID(ctx, id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "event not found")
			return
		}

		// Reset for replay and queue the processing job in one transaction —
		// a replay without a job would hang forever.
		tx, err := db.Begin(ctx)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to reset event status")
			return
		}
		if err := repo.WithTx(tx).SetForReplay(ctx, evt.ID); err == nil {
			err = q.EnqueueEventProcessTx(ctx, tx, evt.ID)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to reset event status")
			return
		}
		if err := tx.Commit(ctx); err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to reset event status")
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"event_id": evt.ID,
			"message":  "event queued for replay",
		})

		// Audit log
		if auditErr := auditRepo.Insert(ctx, &audit.AuditLog{
			Operator:   GetUsernameFromContext(ctx),
			Action:     "replay",
			TargetType: "event",
			TargetID:   evt.ID,
			Summary:    stringPtr(fmt.Sprintf("replayed event %s", evt.ID)),
		}); auditErr != nil {
			slog.Error("audit log insert failed", "action", "replay", "target_type", "event", "error", auditErr)
		}
	}
}

// stringPtr returns a pointer to the given string.
func stringPtr(s string) *string { return &s }

// marshalSnapshot marshals a value to JSON for audit snapshot_before.
func marshalSnapshot(v any) *string {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}
