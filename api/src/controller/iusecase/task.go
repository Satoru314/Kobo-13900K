// Package iusecase defines what the controllers need from the usecases.
package iusecase

import (
	"context"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

type TaskUsecase interface {
	Create(ctx context.Context, in usecase.CreateTaskInput, now time.Time) (domain.Task, error)
	List(ctx context.Context, in usecase.ListTasksInput) ([]usecase.TaskSummary, error)
	Get(ctx context.Context, id int64) (usecase.TaskDetail, error)
	Events(ctx context.Context, id int64) ([]domain.Event, error)
	Update(ctx context.Context, in usecase.UpdateTaskInput, now time.Time) (domain.Task, error)
	Cancel(ctx context.Context, id int64, now time.Time) (domain.Task, error)
	Retry(ctx context.Context, id int64, now time.Time) (domain.Task, error)
}
