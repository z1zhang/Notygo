package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"notygo/internal/delivery"
)

const (
	dedupeCleanupInterval = 15 * time.Minute
	stuckSweepInterval    = 1 * time.Minute
	// stuckThreshold: business rows older than this without progress are
	// considered orphaned (their River job was lost, discarded, or the process
	// died mid-work). Generous enough that normal processing never trips it.
	stuckThreshold = 2 * time.Minute
)

// DedupeCleanupArgs is the periodic dedupe_keys vacuum job.
type DedupeCleanupArgs struct{}

// Kind implements river.JobArgs.
func (DedupeCleanupArgs) Kind() string { return "dedupe_cleanup" }

// DedupeCleanupWorker removes expired dedupe keys, previously the job of the
// (deleted) lease recovery loop.
type DedupeCleanupWorker struct {
	river.WorkerDefaults[DedupeCleanupArgs]

	deliveryRepo *delivery.Repository
}

// Work implements river.Worker.
func (w *DedupeCleanupWorker) Work(ctx context.Context, _ *river.Job[DedupeCleanupArgs]) error {
	cleaned, err := w.deliveryRepo.CleanExpiredDedupeKeys(ctx)
	if err != nil {
		return err
	}
	if cleaned > 0 {
		slog.Info("dedupe keys cleaned", "count", cleaned)
	}
	return nil
}

// StuckSweepArgs is the periodic stuck-row recovery job.
type StuckSweepArgs struct{}

// Kind implements river.JobArgs.
func (StuckSweepArgs) Kind() string { return "stuck_sweep" }

// StuckSweepWorker replaces the old lease recovery loops: it finds business
// rows that stopped making progress and re-enqueues their queue jobs.
//   - events stuck in `received` (job lost before processing)
//   - deliveries stuck in `processing` (process died mid-send)
//
// The workers' own status guards make re-enqueueing safe: a job that finds
// its row already handled (or actively handled elsewhere) simply no-ops.
type StuckSweepWorker struct {
	river.WorkerDefaults[StuckSweepArgs]

	pool         *pgxpool.Pool
	deliveryRepo *delivery.Repository

	// enqueue functions are bound after the Manager exists (see
	// bindEnqueues) — the workers are registered with the River client
	// before the client is fully constructed.
	enqueueEvent    func(ctx context.Context, eventID string) error
	enqueueDelivery func(ctx context.Context, deliveryID string, maxAttempts int) error
}

// bindEnqueues wires the manager's enqueue helpers into the sweep worker.
func (w *StuckSweepWorker) bindEnqueues(m *Manager) {
	w.enqueueEvent = m.EnqueueEventProcess
	w.enqueueDelivery = m.EnqueueDeliverySend
}

// Work implements river.Worker.
func (w *StuckSweepWorker) Work(ctx context.Context, _ *river.Job[StuckSweepArgs]) error {
	w.sweepStuckEvents(ctx)
	w.sweepStuckDeliveries(ctx)
	return nil
}

func (w *StuckSweepWorker) sweepStuckEvents(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`SELECT id FROM notygo_event WHERE status = 0 AND received_at < $1 LIMIT 100`,
		timeNowUTC().Add(-stuckThreshold))
	if err != nil {
		slog.Error("stuck sweep: query events", "error", err)
		return
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			slog.Error("stuck sweep: scan event id", "error", err)
			return
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		slog.Error("stuck sweep: iterate events", "error", err)
		return
	}

	for _, id := range ids {
		if err := w.enqueueEvent(ctx, id); err != nil {
			slog.Error("stuck sweep: re-enqueue event", "event_id", id, "error", err)
		} else {
			slog.Warn("stuck sweep: re-enqueued stuck event", "event_id", id)
		}
	}
}

func (w *StuckSweepWorker) sweepStuckDeliveries(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`UPDATE notygo_delivery SET status = 1, updated_at = $1
		 WHERE id IN (
		   SELECT id FROM notygo_delivery WHERE status = 2 AND updated_at < $2 LIMIT 100
		 )
		 RETURNING id, max_attempts`,
		timeNowUTC(), timeNowUTC().Add(-stuckThreshold))
	if err != nil {
		slog.Error("stuck sweep: reset stuck deliveries", "error", err)
		return
	}
	defer rows.Close()

	type stuck struct {
		ID          string
		MaxAttempts int
	}
	var items []stuck
	for rows.Next() {
		var s stuck
		if err := rows.Scan(&s.ID, &s.MaxAttempts); err != nil {
			slog.Error("stuck sweep: scan delivery", "error", err)
			return
		}
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		slog.Error("stuck sweep: iterate deliveries", "error", err)
		return
	}

	for _, s := range items {
		maxAttempts := max(s.MaxAttempts, 1)
		if err := w.enqueueDelivery(ctx, s.ID, maxAttempts); err != nil {
			slog.Error("stuck sweep: re-enqueue delivery", "delivery_id", s.ID, "error", err)
		} else {
			slog.Warn("stuck sweep: re-enqueued stuck delivery", "delivery_id", s.ID)
		}
	}
}
