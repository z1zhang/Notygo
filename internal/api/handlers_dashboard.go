package api

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"zpush/internal/storage"
)

// failedEventSummary is a dashboard row: the fields the error list needs, no payload.
type failedEventSummary struct {
	ID           string    `json:"id"`
	SourceID     string    `json:"source_id"`
	Status       string    `json:"status"`
	ErrorCode    *string   `json:"error_code"`
	ErrorSummary *string   `json:"error_summary"`
	ReceivedAt   time.Time `json:"received_at"`
}

// failedDeliverySummary is a dashboard row: the fields the error list needs, no rendered content.
type failedDeliverySummary struct {
	ID               string    `json:"id"`
	DestinationID    string    `json:"destination_id"`
	Status           string    `json:"status"`
	LastErrorCode    *string   `json:"last_error_code"`
	LastErrorSummary *string   `json:"last_error_summary"`
	CreatedAt        time.Time `json:"created_at"`
}

// DashboardStats returns all dashboard data in one atomic call:
// status counts for the stat tiles plus the recent failed events/deliveries
// for the error list. Replaces the previous 4-6 separate list requests.
func DashboardStats(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Status counts via GROUP BY — one query per table instead of one per tile
		eventCounts := map[string]int64{}
		rows, err := db.QueryContext(ctx, "SELECT status, COUNT(*) FROM events GROUP BY status")
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to count events")
			return
		}
		for rows.Next() {
			var status string
			var n int64
			if err := rows.Scan(&status, &n); err != nil {
				rows.Close()
				WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to count events")
				return
			}
			eventCounts[status] = n
		}
		rows.Close()

		deliveryCounts := map[string]int64{}
		rows, err = db.QueryContext(ctx, "SELECT status, COUNT(*) FROM deliveries GROUP BY status")
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to count deliveries")
			return
		}
		for rows.Next() {
			var status string
			var n int64
			if err := rows.Scan(&status, &n); err != nil {
				rows.Close()
				WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to count deliveries")
				return
			}
			deliveryCounts[status] = n
		}
		rows.Close()

		var totalEvents int64
		for _, n := range eventCounts {
			totalEvents += n
		}

		// Recent failures (same filters the old per-page requests used)
		failedEvents, err := queryFailedEvents(ctx, db)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to list failed events")
			return
		}
		failedDeliveries, err := queryFailedDeliveries(ctx, db)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to list failed deliveries")
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"total_events":             totalEvents,
			"processing_events":        eventCounts["processing"],
			"pending_deliveries":       deliveryCounts["pending"],
			"failed_deliveries":        deliveryCounts["failed"],
			"recent_failed_events":     failedEvents,
			"recent_failed_deliveries": failedDeliveries,
		})
	}
}

func queryFailedEvents(ctx context.Context, db *sql.DB) ([]failedEventSummary, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, source_id, status, error_code, error_summary, received_at FROM events WHERE status = 'failed' ORDER BY received_at DESC LIMIT 5")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []failedEventSummary
	for rows.Next() {
		var s failedEventSummary
		var errorCode, errorSummary sql.NullString
		var receivedAt string
		if err := rows.Scan(&s.ID, &s.SourceID, &s.Status, &errorCode, &errorSummary, &receivedAt); err != nil {
			return nil, err
		}
		if errorCode.Valid {
			s.ErrorCode = &errorCode.String
		}
		if errorSummary.Valid {
			s.ErrorSummary = &errorSummary.String
		}
		s.ReceivedAt, _ = storage.ParseTime(receivedAt)
		out = append(out, s)
	}
	return out, rows.Err()
}

func queryFailedDeliveries(ctx context.Context, db *sql.DB) ([]failedDeliverySummary, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, destination_id, status, last_error_code, last_error_summary, created_at FROM deliveries WHERE status = 'failed' ORDER BY created_at DESC LIMIT 5")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []failedDeliverySummary
	for rows.Next() {
		var s failedDeliverySummary
		var errorCode, errorSummary sql.NullString
		var createdAt string
		if err := rows.Scan(&s.ID, &s.DestinationID, &s.Status, &errorCode, &errorSummary, &createdAt); err != nil {
			return nil, err
		}
		if errorCode.Valid {
			s.LastErrorCode = &errorCode.String
		}
		if errorSummary.Valid {
			s.LastErrorSummary = &errorSummary.String
		}
		s.CreatedAt, _ = storage.ParseTime(createdAt)
		out = append(out, s)
	}
	return out, rows.Err()
}
