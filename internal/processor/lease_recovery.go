package processor

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"zpush/internal/delivery"
	"zpush/internal/notify"
	"zpush/internal/storage"
)

// LeaseRecoveryLoop recovers events whose lease has expired by resetting them
// back to 'received' status so they can be re-claimed.
// Also cleans up expired dedupe keys to prevent unbounded table growth.
func LeaseRecoveryLoop(ctx context.Context, db *sql.DB, interval time.Duration, notifier *notify.Notifier) {
	slog.Info("event lease recovery loop starting", "interval", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	deliveryRepo := delivery.NewRepository(db)

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
			} else {
				n, _ := result.RowsAffected()
				if n > 0 {
					slog.Info("event lease recovery reclaimed events", "count", n)
					// Wake up the event processor dispatcher immediately
					if notifier != nil {
						notifier.NotifyEvent()
					}
				}
			}

			// Clean up expired dedupe keys
			cleaned, err := deliveryRepo.CleanExpiredDedupeKeys(ctx)
			if err != nil {
				slog.Error("dedupe key cleanup error", "error", err)
			} else if cleaned > 0 {
				slog.Info("cleaned expired dedupe keys", "count", cleaned)
			}
		}
	}
}
