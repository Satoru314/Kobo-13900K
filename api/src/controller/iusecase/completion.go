package iusecase

import (
	"context"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

type CompletionUsecase interface {
	CompleteDevelopment(ctx context.Context, in usecase.CompleteDevelopmentInput, now time.Time) (domain.Task, error)
	CompleteReview(ctx context.Context, in usecase.CompleteReviewInput, now time.Time) (domain.Task, error)
	ReportBlocked(ctx context.Context, in usecase.ReportBlockedInput, now time.Time) (domain.Task, error)
}
