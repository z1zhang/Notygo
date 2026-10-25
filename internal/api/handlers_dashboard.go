package api

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"notygo/internal/delivery"
	"notygo/internal/event"
)

// failedEventSummary is a dashboard row: the fields the error list needs, no payload.
type failedEventSummary struct {
	ID           string    `json:"id"`
	EndpointID   string    `json:"endpoint_id"`
	Status       string    `json:"status"`
	ErrorCode    *string   `json:"error_code"`
	ErrorSummary *string   `json:"error_summary"`
	ReceivedAt   time.Time `json:"received_at"`
}

// failedDeliverySummary is a dashboard row: the fields the error list needs, no rendered content.
type failedDeliverySummary struct {
	ID               string    `json:"id"`
	ChannelID        string    `json:"channel_id"`
	Status           string    `json:"status"`
	LastErrorCode    *string   `json:"last_error_code"`
	LastErrorSummary *string   `json:"last_error_summary"`
	CreatedAt        time.Time `json:"created_at"`
}

// DashboardStats returns all dashboard data in one atomic call:
// status counts for the stat tiles plus the recent failed events/deliveries
// for the error list. Replaces the previous 4-6 separate list requests.
func DashboardStats(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Status counts via GROUP BY — one query per table instead of one per tile
		eventCounts := map[string]int64{}
		rows, err := db.Query(ctx, "SELECT status, COUNT(*) FROM notygo_events GROUP BY status")
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
		rows, err = db.Query(ctx, "SELECT status, COUNT(*) FROM notygo_deliveries GROUP BY status")
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

func queryFailedEvents(ctx context.Context, db *pgxpool.Pool) ([]failedEventSummary, error) {
	rows, err := db.Query(ctx,
		// event status code: 4=failed (see internal/event/codes.go)
		"SELECT id, endpoint_id, error_code, error_summary, received_at FROM notygo_events WHERE status = 4 ORDER BY received_at DESC LIMIT 5")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []failedEventSummary
	for rows.Next() {
		s := failedEventSummary{Status: event.EventStatusFailed}
		var errorCode, errorSummary sql.NullString
		if err := rows.Scan(&s.ID, &s.EndpointID, &errorCode, &errorSummary, &s.ReceivedAt); err != nil {
			return nil, err
		}
		if errorCode.Valid {
			s.ErrorCode = &errorCode.String
		}
		if errorSummary.Valid {
			s.ErrorSummary = &errorSummary.String
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func queryFailedDeliveries(ctx context.Context, db *pgxpool.Pool) ([]failedDeliverySummary, error) {
	rows, err := db.Query(ctx,
		// delivery status code: 4=failed (see internal/delivery/codes.go)
		"SELECT id, channel_id, last_error_code, last_error_summary, created_at FROM notygo_deliveries WHERE status = 4 ORDER BY created_at DESC LIMIT 5")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []failedDeliverySummary
	for rows.Next() {
		s := failedDeliverySummary{Status: delivery.DeliveryStatusFailed}
		var errorCode, errorSummary sql.NullString
		if err := rows.Scan(&s.ID, &s.ChannelID, &errorCode, &errorSummary, &s.CreatedAt); err != nil {
			return nil, err
		}
		if errorCode.Valid {
			s.LastErrorCode = &errorCode.String
		}
		if errorSummary.Valid {
			s.LastErrorSummary = &errorSummary.String
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
