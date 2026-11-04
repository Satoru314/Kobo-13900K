package irepository

import (
	"context"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type ProjectRepository interface {
	Create(ctx context.Context, p *domain.Project) error
	Get(ctx context.Context, id int64) (domain.Project, error)
	GetByDir(ctx context.Context, dir string) (domain.Project, error)
	List(ctx context.Context) ([]domain.Project, error)
}
