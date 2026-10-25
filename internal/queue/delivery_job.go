package queue

import (
	"context"

	"github.com/riverqueue/river"

	"notygo/internal/delivery"
)

// DeliverySendArgs identifies the delivery to send.
type DeliverySendArgs struct {
	DeliveryID string `json:"delivery_id"`
}

// Kind implements river.JobArgs.
func (DeliverySendArgs) Kind() string { return "delivery_send" }

// DeliverySendWorker drives one send attempt. The retry dichotomy:
//   - permanent failure  → river.JobCancel: the job is cancelled, no retries
//   - transient failure  → returned error: River schedules the next attempt
//     per the shared RetryPolicy; the attempt budget is the channel's
//     max_attempts (snapshot at insert time as the job's MaxAttempts)
type DeliverySendWorker struct {
	river.WorkerDefaults[DeliverySendArgs]

	sender *delivery.SendService
}

// Work implements river.Worker.
func (w *DeliverySendWorker) Work(ctx context.Context, job *river.Job[DeliverySendArgs]) error {
	permanent, err := w.sender.Execute(ctx, job.Args.DeliveryID, job.Attempt)
	if err == nil {
		return nil
	}
	if permanent {
		return river.JobCancel(err)
	}
	return err
}
