package irepository

import (
	"context"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type PullRequestRepository interface {
	Create(ctx context.Context, pr *domain.PullRequest) error
	GetByTask(ctx context.Context, taskID int64) (domain.PullRequest, error)
	Update(ctx context.Context, pr domain.PullRequest) error
}
