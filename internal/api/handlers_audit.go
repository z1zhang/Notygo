package api

import (
	"database/sql"
	"net/http"

	"zPush/internal/audit"
)

// ListAuditLogs returns a paginated list of audit logs with optional filtering.
func ListAuditLogs(db *sql.DB) http.HandlerFunc {
	repo := audit.NewRepository(db)
	return func(w http.ResponseWriter, r *http.Request) {
		page, size := ParsePagination(r)
		action := r.URL.Query().Get("action")
		targetType := r.URL.Query().Get("target_type")
		startTime := r.URL.Query().Get("start_time")
		endTime := r.URL.Query().Get("end_time")

		logs, total, err := repo.List(r.Context(), page, size, action, targetType, startTime, endTime)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "SYSTEM_999", "failed to list audit logs")
			return
		}

		WriteJSON(w, http.StatusOK, PaginatedResponse[audit.AuditLog]{
			Data:  logs,
			Total: total,
			Page:  page,
			Size:  size,
		})
	}
}
