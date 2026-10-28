package api

import (
	"net/http"
	"notygo/internal/errcode"

	"notygo/internal/queue"
)

// GetQueueStats returns river_job counts grouped by queue and state, for the
// dashboard's queue health card.
func GetQueueStats(q *queue.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := q.Stats(r.Context())
		if err != nil {
			WriteError(w, http.StatusInternalServerError, errcode.SystemInternal, "failed to load queue stats")
			return
		}
		if rows == nil {
			rows = []queue.QueueStatsRow{}
		}
		WriteJSON(w, http.StatusOK, map[string]any{
			"queues":            rows,
			"retention_seconds": int64(q.Retention().Seconds()),
		})
	}
}
