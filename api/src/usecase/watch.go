package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

// CheckCI looks at the CI of every task in ci and moves the finished ones on.
// A failing lookup is left for the next check.
func (u *Usecase) CheckCI(ctx context.Context, now time.Time) error {
	tasks, err := u.db.ListTasks(ctx, domain.StatusCI)
	if err != nil {
		return err
	}
	var errs []error
	for _, t := range tasks {
		if err := u.checkTaskCI(ctx, t.ID, now); err != nil {
			errs = append(errs, fmt.Errorf("task %d: %w", t.ID, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("ci check: %v", errs)
	}
	return nil
}

func (u *Usecase) checkTaskCI(ctx context.Context, taskID int64, now time.Time) error {
	pr, err := u.db.GetPullRequestByTask(ctx, taskID)
	if err != nil {
		return err
	}
	task, err := u.db.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	project, err := u.db.GetProject(ctx, task.ProjectID)
	if err != nil {
		return err
	}
	ci, err := u.repo.CI(ctx, fullName(project), pr.Number)
	if err != nil || ci.State == CIPending {
		return err
	}
	err = u.db.InTx(ctx, func(ctx context.Context) error {
		task, err := u.db.GetTaskForUpdate(ctx, taskID)
		if err != nil || task.Status != domain.StatusCI {
			return err
		}
		if err := u.record(ctx, taskID, nil, domain.EventCIResult,
			map[string]any{"state": string(ci.State), "summary": ci.Summary}, now); err != nil {
			return err
		}
		if ci.State == CISuccess {
			return u.move(ctx, &task, domain.StatusReview, "", now)
		}
		round, err := u.addRound(ctx, taskID, domain.ReviewCIFailed, ci.Summary, now)
		if err != nil {
			return err
		}
		return u.move(ctx, &task, nextStatus(round, u.settings.MaxReviewRounds), "", now)
	})
	if err == nil {
		u.notify.QueueMayRun()
		u.notify.TaskChanged(taskID)
	}
	return err
}

// openStatuses are the statuses a task with an open PR can be in while the AI is not working on it.
var openStatuses = []domain.Status{domain.StatusCI, domain.StatusReview, domain.StatusFix,
	domain.StatusDone, domain.StatusUnresolved, domain.StatusError}

// CheckPullRequests finishes tasks whose PR was merged or closed. Running tasks
// are left until their run ends.
func (u *Usecase) CheckPullRequests(ctx context.Context, now time.Time) error {
	tasks, err := u.db.ListTasks(ctx, openStatuses...)
	if err != nil {
		return err
	}
	var errs []error
	for _, t := range tasks {
		if err := u.checkTaskPR(ctx, t, now); err != nil {
			errs = append(errs, fmt.Errorf("task %d: %w", t.ID, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("pr check: %v", errs)
	}
	return nil
}

func (u *Usecase) checkTaskPR(ctx context.Context, task domain.Task, now time.Time) error {
	pr, err := u.db.GetPullRequestByTask(ctx, task.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	project, err := u.db.GetProject(ctx, task.ProjectID)
	if err != nil {
		return err
	}
	info, err := u.repo.PullRequest(ctx, fullName(project), pr.Number)
	if err != nil || info.State == domain.PRStateOpen {
		return err
	}
	err = u.db.InTx(ctx, func(ctx context.Context) error {
		task, err := u.db.GetTaskForUpdate(ctx, task.ID)
		if err != nil || !canMoveTo(task.Status, domain.StatusFinished) {
			return err
		}
		pr.State, pr.MergedAt = info.State, info.MergedAt
		if err := u.db.UpdatePullRequest(ctx, pr); err != nil {
			return err
		}
		return u.move(ctx, &task, domain.StatusFinished, "pull request "+string(info.State), now)
	})
	if err == nil {
		u.notify.TaskChanged(task.ID)
	}
	return err
}
