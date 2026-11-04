package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/iusecase"
)

// Poller calls a checker on a timer: the CI results and the PR states.
type Poller struct {
	name     string
	checker  iusecase.Checker
	interval time.Duration
}

func NewPoller(name string, checker iusecase.Checker, interval time.Duration) *Poller {
	return &Poller{name: name, checker: checker, interval: interval}
}

func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.checker.Check(ctx, time.Now()); err != nil && ctx.Err() == nil {
				slog.Warn("poll failed; will retry", "poller", p.name, "err", err)
			}
		}
	}
}
