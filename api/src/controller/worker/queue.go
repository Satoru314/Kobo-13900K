// Package worker holds the entry points kobo runs by itself: on a timer or on a signal.
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/iusecase"
)

// QueueWorker takes tasks out of the queue one at a time. It wakes on signal,
// and every interval so off-peak windows and retry delays are noticed.
type QueueWorker struct {
	queue    iusecase.QueueUsecase
	signal   <-chan struct{}
	interval time.Duration
}

func NewQueueWorker(queue iusecase.QueueUsecase, signal <-chan struct{}, interval time.Duration) *QueueWorker {
	return &QueueWorker{queue: queue, signal: signal, interval: interval}
}

func (w *QueueWorker) Run(ctx context.Context) {
	if err := w.queue.Recover(ctx, time.Now()); err != nil {
		slog.Error("recover running tasks", "err", err)
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		if w.runOne(ctx) {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-w.signal:
		case <-ticker.C:
		}
	}
}

// runOne runs the next task to the end and reports whether there was one.
func (w *QueueWorker) runOne(ctx context.Context) bool {
	run, ok, err := w.queue.Take(ctx, time.Now())
	if err != nil {
		slog.Error("take next task", "err", err)
		return false
	}
	if !ok {
		return false
	}
	slog.Info("run started", "task", run.Task.ID, "role", run.Role, "resume", run.Resume)
	runErr := w.queue.Execute(ctx, run)
	if ctx.Err() != nil {
		// kobo is stopping; Recover puts the task back on the next start.
		return false
	}
	if err := w.queue.Settle(context.WithoutCancel(ctx), run, runErr, time.Now()); err != nil {
		slog.Error("settle run", "task", run.Task.ID, "err", err)
	}
	slog.Info("run ended", "task", run.Task.ID, "role", run.Role, "err", runErr)
	return true
}
