package iusecase

import (
	"context"
	"time"
)

// Checker is a usecase polled on a timer: CI results and PR states.
type Checker interface {
	Check(ctx context.Context, now time.Time) error
}
