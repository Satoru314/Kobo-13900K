package irepository

import (
	"context"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type EventRepository interface {
	Create(ctx context.Context, e *domain.Event) error
	// ListByTask returns the task's events in the order they happened.
	ListByTask(ctx context.Context, taskID int64) ([]domain.Event, error)
}
