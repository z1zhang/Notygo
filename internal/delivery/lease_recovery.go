package delivery

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// LeaseRecoveryLoop recovers deliveries whose lease has expired by resetting them
// back to 'pending' status so they can be re-claimed.
func LeaseRecoveryLoop(ctx context.Context, db *sql.DB, interval time.Duration) {
	slog.Info("delivery lease recovery loop starting", "interval", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("delivery lease recovery loop stopped")
			return

		case <-ticker.C:
			result, err := db.ExecContext(ctx,
				`UPDATE deliveries SET status='pending', attempt_count=attempt_count-1, lease_until=NULL, lease_owner=NULL, updated_at=datetime('now')
				 WHERE status='sending' AND lease_until IS NOT NULL AND lease_until < datetime('now')`)
			if err != nil {
				slog.Error("delivery lease recovery error", "error", err)
				continue
			}
			n, _ := result.RowsAffected()
			if n > 0 {
				slog.Info("delivery lease recovery reclaimed deliveries", "count", n)
			}
		}
	}
}
