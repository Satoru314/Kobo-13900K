package iusecase

import (
	"context"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

type ProjectUsecase interface {
	List(ctx context.Context) ([]domain.Project, error)
	Candidates(ctx context.Context) ([]usecase.ProjectCandidate, error)
	Ensure(ctx context.Context, dir string, now time.Time) (domain.Project, error)
}
