// Package worker holds what kobo runs by itself: the queue, and polling GitHub.
package worker

import (
	"context"
	"log/slog"
	"time"
)

// Queue is what the queue worker needs from the usecase.
type Queue interface {
	RecoverRuns(ctx context.Context, now time.Time) error
	RunNext(ctx context.Context, now time.Time) (bool, error)
}

// RunQueue runs tasks one at a time until ctx ends. It wakes on wake, and every
// interval so off-peak windows and retry delays are noticed.
func RunQueue(ctx context.Context, q Queue, wake <-chan struct{}, interval time.Duration) {
	if err := q.RecoverRuns(ctx, time.Now()); err != nil {
		slog.Error("recover running tasks", "err", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		ran, err := q.RunNext(ctx, time.Now())
		if err != nil && ctx.Err() == nil {
			slog.Error("run next task", "err", err)
		}
		if ran && ctx.Err() == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-wake:
		case <-ticker.C:
		}
	}
}

// Poll calls check every interval until ctx ends: the CI results and the PR states.
// A failed check is logged and tried again on the next tick.
func Poll(ctx context.Context, name string, check func(ctx context.Context, now time.Time) error, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := check(ctx, time.Now()); err != nil && ctx.Err() == nil {
				slog.Warn("poll failed; will retry", "poller", name, "err", err)
			}
		}
	}
}
