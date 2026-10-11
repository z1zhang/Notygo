package processor

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"zPush/internal/storage"
)

// LeaseRecoveryLoop recovers events whose lease has expired by resetting them
// back to 'received' status so they can be re-claimed.
func LeaseRecoveryLoop(ctx context.Context, db *sql.DB, interval time.Duration) {
	slog.Info("event lease recovery loop starting", "interval", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("event lease recovery loop stopped")
			return

		case <-ticker.C:
			nowExpr := storage.NowExpr()
			result, err := db.ExecContext(ctx,
				"UPDATE events SET status='received', lease_until=NULL, lease_owner=NULL"+
					" WHERE status='processing' AND lease_until IS NOT NULL AND lease_until < "+nowExpr)
			if err != nil {
				slog.Error("event lease recovery error", "error", err)
				continue
			}
			n, _ := result.RowsAffected()
			if n > 0 {
				slog.Info("event lease recovery reclaimed events", "count", n)
			}
		}
	}
}
