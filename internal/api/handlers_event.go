package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"

	"zpush/internal/event"
	"zpush/internal/notify"
)

// ListEvents returns a paginated list of events, with optional status filter.
func ListEvents(db *sql.DB) http.HandlerFunc {
	repo := event.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		page, size := ParsePagination(r)
		statusFilter := r.URL.Query().Get("status")

		events, total, err := repo.List(r.Context(), page, size, statusFilter)
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
func GetEvent(db *sql.DB) http.HandlerFunc {
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

// ReplayEvent sets the event status back to 'received' for reprocessing.
// The processor will create a new EventRun with run_type='replay' when it picks up the event.
func ReplayEvent(db *sql.DB, notifier *notify.Notifier) http.HandlerFunc {
	repo := event.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		evt, err := repo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "event not found")
			return
		}

		// Set event for replay: status back to 'received', run_type = 'replay'
		if err := repo.SetForReplay(r.Context(), evt.ID); err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to reset event status")
			return
		}

		// Notify the event processor to pick up the replayed event immediately
		if notifier != nil {
			notifier.NotifyEvent()
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"event_id": evt.ID,
			"message":  "event queued for replay",
		})
	}
}
