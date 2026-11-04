package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

// openStatuses are the statuses a task with an open PR can be in.
var openStatuses = []domain.Status{domain.StatusCI, domain.StatusReviewing, domain.StatusFix,
	domain.StatusDone, domain.StatusUnresolved, domain.StatusError}

type PRUsecase struct {
	tx       irepository.Transactor
	tasks    irepository.TaskRepository
	prs      irepository.PullRequestRepository
	projects *ProjectUsecase
	github   irepository.GitHub
	track    tracker
}

func NewPRUsecase(tx irepository.Transactor, tasks irepository.TaskRepository, prs irepository.PullRequestRepository,
	events irepository.EventRepository, projects *ProjectUsecase, github irepository.GitHub) *PRUsecase {
	return &PRUsecase{tx: tx, tasks: tasks, prs: prs, projects: projects, github: github, track: newTracker(tasks, events)}
}

// Check finishes tasks whose PR was merged or closed. Running tasks are left
// until their run ends.
func (u *PRUsecase) Check(ctx context.Context, now time.Time) error {
	tasks, err := u.tasks.List(ctx, openStatuses...)
	if err != nil {
		return err
	}
	var errs []error
	for _, t := range tasks {
		if t.Running {
			continue
		}
		if err := u.checkTask(ctx, t, now); err != nil {
			errs = append(errs, fmt.Errorf("task %d: %w", t.ID, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("pr check: %v", errs)
	}
	return nil
}

func (u *PRUsecase) checkTask(ctx context.Context, task domain.Task, now time.Time) error {
	pr, err := u.prs.GetByTask(ctx, task.ID)
	if errors.Is(err, irepository.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	project, err := u.projects.Get(ctx, task.ProjectID)
	if err != nil {
		return err
	}
	info, err := u.github.PR(ctx, project.FullName(), pr.Number)
	if err != nil || info.State == domain.PRStateOpen {
		return err
	}
	return u.tx.InTx(ctx, func(ctx context.Context) error {
		task, err := u.tasks.GetForUpdate(ctx, task.ID)
		if err != nil || task.Running || !task.Status.CanMoveTo(domain.StatusFinished) {
			return err
		}
		pr.State, pr.MergedAt = info.State, info.MergedAt
		if err := u.prs.Update(ctx, pr); err != nil {
			return err
		}
		return u.track.move(ctx, &task, domain.StatusFinished, "pull request "+string(info.State), now)
	})
}
