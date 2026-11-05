package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

type TaskUsecase struct {
	tx       irepository.Transactor
	tasks    irepository.TaskRepository
	prs      irepository.PullRequestRepository
	rounds   irepository.ReviewRoundRepository
	events   irepository.EventRepository
	projects *ProjectUsecase
	signal   *QueueSignal
	track    tracker
}

func NewTaskUsecase(tx irepository.Transactor, tasks irepository.TaskRepository, prs irepository.PullRequestRepository,
	rounds irepository.ReviewRoundRepository, events irepository.EventRepository, projects *ProjectUsecase, signal *QueueSignal) *TaskUsecase {
	return &TaskUsecase{tx: tx, tasks: tasks, prs: prs, rounds: rounds, events: events,
		projects: projects, signal: signal, track: newTracker(tasks, events)}
}

type CreateTaskInput struct {
	Project     string      `json:"project" jsonschema:"directory name of the target repository, e.g. emela"`
	Title       string      `json:"title" jsonschema:"short title shown in the task list"`
	Description string      `json:"description" jsonschema:"full instructions for the developing AI"`
	Mode        domain.Mode `json:"mode,omitempty" jsonschema:"priority, immediate (default) or off_peak"`
}

func (u *TaskUsecase) Create(ctx context.Context, in CreateTaskInput, now time.Time) (domain.Task, error) {
	if in.Mode == "" {
		in.Mode = domain.ModeImmediate
	}
	var task domain.Task
	err := u.tx.InTx(ctx, func(ctx context.Context) error {
		project, err := u.projects.Ensure(ctx, in.Project, now)
		if err != nil {
			return err
		}
		task, err = domain.NewTask(project.ID, in.Title, in.Description, in.Mode, now)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		if err := u.tasks.Create(ctx, &task); err != nil {
			return err
		}
		return u.track.record(ctx, task.ID, nil, domain.EventTaskCreated, map[string]any{"mode": task.Mode}, now)
	})
	if err == nil {
		u.signal.Notify()
	}
	return task, err
}

type ListTasksInput struct {
	Statuses      []domain.Status `json:"statuses,omitempty" jsonschema:"only tasks in these statuses"`
	IncludeHidden bool            `json:"include_hidden,omitempty" jsonschema:"also list finished and cancelled tasks"`
}

func (u *TaskUsecase) List(ctx context.Context, in ListTasksInput) ([]domain.Task, error) {
	for _, s := range in.Statuses {
		if !s.Valid() {
			return nil, fmt.Errorf("%w: unknown status %q", ErrInvalidInput, s)
		}
	}
	all, err := u.tasks.List(ctx, in.Statuses...)
	if err != nil {
		return nil, err
	}
	out := []domain.Task{}
	for _, t := range all {
		if in.IncludeHidden || len(in.Statuses) > 0 || !t.Status.Hidden() {
			out = append(out, t)
		}
	}
	return out, nil
}

type TaskDetail struct {
	Task        domain.Task          `json:"task"`
	Project     domain.Project       `json:"project"`
	PullRequest *domain.PullRequest  `json:"pull_request"`
	Rounds      []domain.ReviewRound `json:"review_rounds"`
	// LastError is the latest system error or blocked reason while the task is in error.
	LastError string `json:"last_error,omitempty"`
}

func (u *TaskUsecase) Get(ctx context.Context, id int64) (TaskDetail, error) {
	task, err := u.tasks.Get(ctx, id)
	if err != nil {
		return TaskDetail{}, err
	}
	d := TaskDetail{Task: task, Rounds: []domain.ReviewRound{}}
	if d.Project, err = u.projects.Get(ctx, task.ProjectID); err != nil {
		return TaskDetail{}, err
	}
	pr, err := u.prs.GetByTask(ctx, id)
	switch {
	case err == nil:
		d.PullRequest = &pr
	case !errors.Is(err, irepository.ErrNotFound):
		return TaskDetail{}, err
	}
	rounds, err := u.rounds.ListByTask(ctx, id)
	if err != nil {
		return TaskDetail{}, err
	}
	d.Rounds = append(d.Rounds, rounds...)
	if task.Status == domain.StatusError {
		events, err := u.events.ListByTask(ctx, id)
		if err != nil {
			return TaskDetail{}, err
		}
		d.LastError = history(events).lastError()
	}
	return d, nil
}

func (u *TaskUsecase) Events(ctx context.Context, id int64) ([]domain.Event, error) {
	if _, err := u.tasks.Get(ctx, id); err != nil {
		return nil, err
	}
	events, err := u.events.ListByTask(ctx, id)
	if events == nil {
		events = []domain.Event{}
	}
	return events, err
}

type UpdateTaskInput struct {
	ID          int64        `json:"id"`
	Title       *string      `json:"title,omitempty"`
	Description *string      `json:"description,omitempty"`
	Mode        *domain.Mode `json:"mode,omitempty" jsonschema:"priority, immediate or off_peak"`
}

func (u *TaskUsecase) Update(ctx context.Context, in UpdateTaskInput, now time.Time) (domain.Task, error) {
	var task domain.Task
	err := u.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if task, err = u.tasks.GetForUpdate(ctx, in.ID); err != nil {
			return err
		}
		if err := task.Edit(in.Title, in.Description, in.Mode, now); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		return u.tasks.Update(ctx, task)
	})
	return task, err
}

func (u *TaskUsecase) Cancel(ctx context.Context, id int64, now time.Time) (domain.Task, error) {
	var task domain.Task
	err := u.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if task, err = u.tasks.GetForUpdate(ctx, id); err != nil {
			return err
		}
		if err := u.track.move(ctx, &task, domain.StatusCancelled, "cancelled by user", now); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		return nil
	})
	return task, err
}

// Retry puts a task in error back into the queue at the status it failed in.
func (u *TaskUsecase) Retry(ctx context.Context, id int64, now time.Time) (domain.Task, error) {
	var task domain.Task
	err := u.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if task, err = u.tasks.GetForUpdate(ctx, id); err != nil {
			return err
		}
		if task.Status != domain.StatusError {
			return fmt.Errorf("%w: task %d is %s, not error", ErrInvalidInput, id, task.Status)
		}
		events, err := u.events.ListByTask(ctx, id)
		if err != nil {
			return err
		}
		back, ok := history(events).statusBeforeError()
		if !ok {
			return fmt.Errorf("task %d: no record of the status before the error", id)
		}
		if err := u.track.record(ctx, id, nil, domain.EventRetried, nil, now); err != nil {
			return err
		}
		return u.track.move(ctx, &task, back, "retried by user", now)
	})
	if err == nil {
		u.signal.Notify()
	}
	return task, err
}
