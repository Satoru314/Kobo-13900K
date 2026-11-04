package irepository

import (
	"context"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type TaskRepository interface {
	Create(ctx context.Context, t *domain.Task) error
	Get(ctx context.Context, id int64) (domain.Task, error)
	// GetForUpdate locks the row until the transaction ends.
	GetForUpdate(ctx context.Context, id int64) (domain.Task, error)
	// List returns tasks in the given statuses, or all tasks when none are given, oldest first.
	List(ctx context.Context, statuses ...domain.Status) ([]domain.Task, error)
	Update(ctx context.Context, t domain.Task) error
}
