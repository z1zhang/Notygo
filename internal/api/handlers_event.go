package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"

	"zPush/internal/event"
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
				"event":        evt,
				"latest_run":   nil,
				"parsed_fields": nil,
			})
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"event":        evt,
			"latest_run":   run,
			"parsed_fields": run.ParsedFields,
		})
	}
}

// ReplayEvent creates a new EventRun with run_type='replay' and sets the event
// status back to 'received' for reprocessing.
func ReplayEvent(db *sql.DB) http.HandlerFunc {
	repo := event.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		evt, err := repo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, ErrConfigValidation, "event not found")
			return
		}

		// Create new EventRun with run_type='replay'
		eventRun := &event.EventRun{
			EventID:          evt.ID,
			RunType:          "replay",
			ParsedFields:     map[string]any{},
			ValidationResult: map[string]any{},
			RuleSetSnapshot:  []event.RuleSnapshot{},
			MatchedRuleIDs:   []string{},
			Status:           event.EventRunStatusProcessing,
		}

		createdRun, err := repo.CreateEventRun(r.Context(), eventRun)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to create replay run")
			return
		}

		// Set event status back to 'received' for reprocessing
		if err := repo.SetStatus(r.Context(), evt.ID, event.EventStatusReceived); err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to reset event status")
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"event":     evt.ID,
			"run":       createdRun,
			"message":   "event queued for replay",
		})
	}
}
