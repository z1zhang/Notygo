package queue

import (
	"context"
	"errors"
	"fmt"

	"github.com/riverqueue/river"

	"notygo/internal/event"
	"notygo/internal/processor"
)

// EventProcessArgs identifies the event to process.
type EventProcessArgs struct {
	EventID string `json:"event_id"`
}

// Kind implements river.JobArgs.
func (EventProcessArgs) Kind() string { return "event_process" }

// EventProcessWorker drives the event pipeline: mapping → rule evaluation →
// template render → delivery creation, all inside one transaction committed
// by processor.Service.Process.
type EventProcessWorker struct {
	river.WorkerDefaults[EventProcessArgs]

	eventRepo *event.Repository
	processor *processor.Service
}

// Work implements river.Worker. Returning an error discards the job
// (MaxAttempts=1) without touching the event row; business failures are
// written by the processor as status=failed, and lost jobs are re-driven by
// the stuck sweep.
func (w *EventProcessWorker) Work(ctx context.Context, job *river.Job[EventProcessArgs]) error {
	evt, err := w.eventRepo.GetByID(ctx, job.Args.EventID)
	if err != nil {
		if errors.Is(err, event.ErrNotFound) {
			return nil // event deleted; nothing to process
		}
		return fmt.Errorf("load event %s: %w", job.Args.EventID, err)
	}

	// Idempotency guard: only received events are processable. Duplicate jobs
	// (stuck sweep racing the original, replay while in-flight) no-op here.
	if evt.Status != event.EventStatusReceived {
		return nil
	}

	return w.processor.Process(ctx, evt)
}
