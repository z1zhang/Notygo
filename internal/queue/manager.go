// Package queue wires the River job queue into Notygo: job kinds, workers,
// enqueue helpers, and the shared client. The river_job table is River's own
// schema, created by River's migrator (Migrate) — business tables (events,
// deliveries) remain the source of truth; River is a scheduling shadow only.
package queue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"notygo/internal/backoff"
	"notygo/internal/config"
	"notygo/internal/delivery"
	"notygo/internal/event"
	"notygo/internal/processor"
)

// Queue names. River dispatches by queue so each pipeline stage gets its own
// worker pool size.
const (
	QueueEventProcess = "event_process"
	QueueDeliverySend = "delivery_send"
	QueueMaintenance  = "maintenance"
)

// Deps bundles the services the queue workers drive.
type Deps struct {
	EventRepo     *event.Repository
	DeliveryRepo  *delivery.Repository
	Processor     *processor.Service
	SenderService *delivery.SendService
}

// Manager owns the River client and exposes enqueue helpers for the rest of
// the application (ingest, handlers, workers).
type Manager struct {
	pool   *pgxpool.Pool
	client *river.Client[pgx.Tx]
	cfg    config.QueueConfig
}

// NewManager builds the River client with all Notygo workers registered.
func NewManager(pool *pgxpool.Pool, cfg config.QueueConfig, deps Deps) (*Manager, error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &EventProcessWorker{
		eventRepo: deps.EventRepo,
		processor: deps.Processor,
	})
	river.AddWorker(workers, &DeliverySendWorker{sender: deps.SenderService})
	river.AddWorker(workers, &DedupeCleanupWorker{deliveryRepo: deps.DeliveryRepo})
	sweepWorker := &StuckSweepWorker{
		pool:         pool,
		deliveryRepo: deps.DeliveryRepo,
	}
	river.AddWorker(workers, sweepWorker)

	periodicJobs := []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(dedupeCleanupInterval),
			func() (river.JobArgs, *river.InsertOpts) {
				return DedupeCleanupArgs{}, &river.InsertOpts{
					Queue:       QueueMaintenance,
					MaxAttempts: 1,
				}
			},
			&river.PeriodicJobOpts{ID: "dedupe-cleanup"},
		),
		river.NewPeriodicJob(
			river.PeriodicInterval(stuckSweepInterval),
			func() (river.JobArgs, *river.InsertOpts) {
				return StuckSweepArgs{}, &river.InsertOpts{
					Queue:       QueueMaintenance,
					MaxAttempts: 1,
				}
			},
			&river.PeriodicJobOpts{ID: "stuck-sweep"},
		),
	}

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			QueueEventProcess: {MaxWorkers: cfg.EventConcurrency},
			QueueDeliverySend: {MaxWorkers: cfg.DeliveryConcurrency},
			QueueMaintenance:  {MaxWorkers: 1},
		},
		Workers:      workers,
		PeriodicJobs: periodicJobs,
		RetryPolicy: &RetryPolicy{Config: backoff.Config{
			Base:          cfg.RetryBaseDelay,
			Max:           cfg.RetryMaxDelay,
			JitterPercent: cfg.RetryJitterPercent,
		}},
		CompletedJobRetentionPeriod: cfg.JobRetention,
		CancelledJobRetentionPeriod: cfg.JobRetention,
		DiscardedJobRetentionPeriod: cfg.JobRetention,
	})
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}

	m := &Manager{pool: pool, client: client, cfg: cfg}
	sweepWorker.bindEnqueues(m)
	return m, nil
}

// Migrate applies River's own schema (river_job and friends). It runs at boot
// after the goose migrations and tracks its versions in the river_migration
// table, so upgrading the River dependency upgrades the schema automatically.
func (m *Manager) Migrate(ctx context.Context) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(m.pool), nil)
	if err != nil {
		return fmt.Errorf("create river migrator: %w", err)
	}
	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("migrate river schema: %w", err)
	}
	if len(res.Versions) > 0 {
		for _, v := range res.Versions {
			slog.Info("river migration applied", "version", v.Version)
		}
	}
	return nil
}

// Start starts the River client (workers + periodic scheduler).
func (m *Manager) Start(ctx context.Context) error {
	return m.client.Start(ctx)
}

// Stop drains in-flight jobs. Cancel-worthy? Use Stop (graceful) from main's
// shutdown path.
func (m *Manager) Stop(ctx context.Context) error {
	return m.client.Stop(ctx)
}

// EnqueueEventProcessTx queues an event_process job in the caller's
// transaction, so the event INSERT and its job become atomic.
// MaxAttempts=1: event processing failures are surfaced as status=failed and
// are re-driven by manual replay, not by queue retries.
func (m *Manager) EnqueueEventProcessTx(ctx context.Context, tx pgx.Tx, eventID string) error {
	_, err := m.client.InsertTx(ctx, tx, EventProcessArgs{EventID: eventID}, &river.InsertOpts{
		Queue:       QueueEventProcess,
		MaxAttempts: 1,
	})
	if err != nil {
		return fmt.Errorf("enqueue event_process job: %w", err)
	}
	return nil
}

// EnqueueEventProcess queues an event_process job outside a transaction
// (manual replay, stuck sweep).
func (m *Manager) EnqueueEventProcess(ctx context.Context, eventID string) error {
	_, err := m.client.Insert(ctx, EventProcessArgs{EventID: eventID}, &river.InsertOpts{
		Queue:       QueueEventProcess,
		MaxAttempts: 1,
	})
	if err != nil {
		return fmt.Errorf("enqueue event_process job: %w", err)
	}
	return nil
}

// EnqueueDeliverySendTx queues a delivery_send job in the caller's
// transaction. maxAttempts snapshots the channel's retry budget at enqueue
// time, mirroring deliveries.max_attempts.
func (m *Manager) EnqueueDeliverySendTx(ctx context.Context, tx pgx.Tx, deliveryID string, maxAttempts int) error {
	_, err := m.client.InsertTx(ctx, tx, DeliverySendArgs{DeliveryID: deliveryID}, &river.InsertOpts{
		Queue:       QueueDeliverySend,
		MaxAttempts: maxAttempts,
	})
	if err != nil {
		return fmt.Errorf("enqueue delivery_send job: %w", err)
	}
	return nil
}

// EnqueueDeliverySend queues a delivery_send job outside a transaction
// (manual/bulk retry, stuck sweep).
func (m *Manager) EnqueueDeliverySend(ctx context.Context, deliveryID string, maxAttempts int) error {
	_, err := m.client.Insert(ctx, DeliverySendArgs{DeliveryID: deliveryID}, &river.InsertOpts{
		Queue:       QueueDeliverySend,
		MaxAttempts: maxAttempts,
	})
	if err != nil {
		return fmt.Errorf("enqueue delivery_send job: %w", err)
	}
	return nil
}

// QueueStatsRow is one (queue, state) count of river_job rows. Terminal-state
// counts (completed/cancelled/discarded) are bounded by the retention window —
// see Manager.Retention, which the UI uses to label the stats' scope.
type QueueStatsRow struct {
	Queue string `json:"queue"`
	State string `json:"state"`
	Count int64  `json:"count"`
}

// Retention is how long River keeps completed/cancelled/discarded jobs before
// deleting them — i.e. the lookback window of the stats above.
func (m *Manager) Retention() time.Duration {
	return m.cfg.JobRetention
}

// Stats returns river_job counts grouped by queue and state.
func (m *Manager) Stats(ctx context.Context) ([]QueueStatsRow, error) {
	rows, err := m.pool.Query(ctx,
		`SELECT queue, state::text, count(*) FROM river_job GROUP BY queue, state ORDER BY queue, state`)
	if err != nil {
		return nil, fmt.Errorf("query queue stats: %w", err)
	}
	defer rows.Close()

	var out []QueueStatsRow
	for rows.Next() {
		var r QueueStatsRow
		if err := rows.Scan(&r.Queue, &r.State, &r.Count); err != nil {
			return nil, fmt.Errorf("scan queue stats: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queue stats: %w", err)
	}
	return out, nil
}
