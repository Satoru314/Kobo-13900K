package irepository

import (
	"context"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type ReviewRoundRepository interface {
	Create(ctx context.Context, r *domain.ReviewRound) error
	// ListByTask returns the rounds in round order.
	ListByTask(ctx context.Context, taskID int64) ([]domain.ReviewRound, error)
}
