package iusecase

import (
	"context"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type ProjectUsecase interface {
	List(ctx context.Context) ([]domain.Project, error)
}
