package iusecase

import (
	"context"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

type QueueUsecase interface {
	Order(ctx context.Context) ([]domain.Task, error)
	Take(ctx context.Context, now time.Time) (usecase.Run, bool, error)
	Execute(ctx context.Context, run usecase.Run) error
	Settle(ctx context.Context, run usecase.Run, runErr error, now time.Time) error
	Recover(ctx context.Context, now time.Time) error
}
