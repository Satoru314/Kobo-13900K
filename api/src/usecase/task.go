package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

func (u *Usecase) CreateTask(ctx context.Context, in CreateTaskInput, now time.Time) (domain.Task, error) {
	if in.Mode == "" {
		in.Mode = domain.ModeImmediate
	}
	var task domain.Task
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		project, err := u.AddProject(ctx, in.Project, now)
		if err != nil {
			return err
		}
		task, err = newTask(project.ID, in.Title, in.Description, in.Mode, now)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		if err := u.db.CreateTask(ctx, &task); err != nil {
			return err
		}
		return u.record(ctx, task.ID, nil, domain.EventTaskCreated, map[string]any{"mode": task.Mode}, now)
	})
	if err == nil {
		u.notify.QueueMayRun()
		u.notify.TaskChanged(task.ID)
	}
	return task, err
}

func (u *Usecase) ListTasks(ctx context.Context, in ListTasksInput) ([]TaskSummary, error) {
	for _, s := range in.Statuses {
		if !s.Valid() {
			return nil, fmt.Errorf("%w: unknown status %q", ErrInvalidInput, s)
		}
	}
	all, err := u.db.ListTasks(ctx, in.Statuses...)
	if err != nil {
		return nil, err
	}
	projects, err := u.db.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	dirs := map[int64]string{}
	for _, p := range projects {
		dirs[p.ID] = p.Dir
	}
	out := []TaskSummary{}
	for _, t := range all {
		if !in.IncludeHidden && len(in.Statuses) == 0 && hidden(t.Status) {
			continue
		}
		s, err := u.summary(ctx, t, dirs[t.ProjectID])
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func (u *Usecase) summary(ctx context.Context, t domain.Task, dir string) (TaskSummary, error) {
	s := TaskSummary{Task: t, Project: dir}
	pr, err := u.db.GetPullRequestByTask(ctx, t.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return s, nil
	}
	if err != nil {
		return TaskSummary{}, err
	}
	s.PRURL, s.PRNumber = pr.URL, pr.Number
	if t.Status == domain.StatusCI {
		s.CI = CIPending
		return s, nil
	}
	events, err := u.db.ListEvents(ctx, t.ID)
	if err != nil {
		return TaskSummary{}, err
	}
	s.CI = CIState(history(events).lastCIState())
	return s, nil
}

func (u *Usecase) GetTask(ctx context.Context, id int64) (TaskDetail, error) {
	task, err := u.db.GetTask(ctx, id)
	if err != nil {
		return TaskDetail{}, err
	}
	d := TaskDetail{Task: task, Rounds: []domain.ReviewRound{}}
	if d.Project, err = u.db.GetProject(ctx, task.ProjectID); err != nil {
		return TaskDetail{}, err
	}
	pr, err := u.db.GetPullRequestByTask(ctx, id)
	switch {
	case err == nil:
		d.PullRequest = &pr
	case !errors.Is(err, domain.ErrNotFound):
		return TaskDetail{}, err
	}
	rounds, err := u.db.ListReviewRounds(ctx, id)
	if err != nil {
		return TaskDetail{}, err
	}
	d.Rounds = append(d.Rounds, rounds...)
	if task.Status == domain.StatusError {
		events, err := u.db.ListEvents(ctx, id)
		if err != nil {
			return TaskDetail{}, err
		}
		d.LastError = history(events).lastError()
	}
	return d, nil
}

func (u *Usecase) ListTaskEvents(ctx context.Context, id int64) ([]domain.Event, error) {
	if _, err := u.db.GetTask(ctx, id); err != nil {
		return nil, err
	}
	events, err := u.db.ListEvents(ctx, id)
	if events == nil {
		events = []domain.Event{}
	}
	return events, err
}

func (u *Usecase) EditTask(ctx context.Context, in EditTaskInput, now time.Time) (domain.Task, error) {
	var task domain.Task
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		var err error
		if task, err = u.db.GetTaskForUpdate(ctx, in.ID); err != nil {
			return err
		}
		if err := edit(&task, in.Title, in.Description, in.Mode, now); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		return u.db.UpdateTask(ctx, task)
	})
	if err == nil {
		u.notify.TaskChanged(task.ID)
	}
	return task, err
}

// CancelTask cancels a task that has not started yet.
func (u *Usecase) CancelTask(ctx context.Context, id int64, now time.Time) (domain.Task, error) {
	var task domain.Task
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		var err error
		if task, err = u.db.GetTaskForUpdate(ctx, id); err != nil {
			return err
		}
		if err := u.move(ctx, &task, domain.StatusCancelled, "cancelled by user", now); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		return nil
	})
	if err == nil {
		u.notify.TaskChanged(task.ID)
	}
	return task, err
}

// RetryTask puts a task in error back into the queue at the status it failed in.
func (u *Usecase) RetryTask(ctx context.Context, id int64, now time.Time) (domain.Task, error) {
	var task domain.Task
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		var err error
		if task, err = u.db.GetTaskForUpdate(ctx, id); err != nil {
			return err
		}
		if task.Status != domain.StatusError {
			return fmt.Errorf("%w: task %d is %s, not error", ErrInvalidInput, id, task.Status)
		}
		events, err := u.db.ListEvents(ctx, id)
		if err != nil {
			return err
		}
		back, ok := history(events).statusBeforeError()
		if !ok {
			return fmt.Errorf("task %d: no record of the status before the error", id)
		}
		if err := u.record(ctx, id, nil, domain.EventRetried, nil, now); err != nil {
			return err
		}
		return u.move(ctx, &task, back, "retried by user", now)
	})
	if err == nil {
		u.notify.QueueMayRun()
		u.notify.TaskChanged(task.ID)
	}
	return task, err
}
