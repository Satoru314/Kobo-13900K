package usecase

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

// AI reports. Runs are serial, so a report always belongs to the one running task.

// runningTask returns the running task, locked.
func (u *Usecase) runningTask(ctx context.Context) (domain.Task, error) {
	tasks, err := u.db.ListTasks(ctx, runningStatuses...)
	if err != nil {
		return domain.Task{}, err
	}
	if len(tasks) == 0 {
		return domain.Task{}, ErrNothingRunning
	}
	return u.db.GetTaskForUpdate(ctx, tasks[0].ID)
}

func (u *Usecase) sessionID(ctx context.Context, task domain.Task, role domain.Role) *int64 {
	s, err := u.db.GetSession(ctx, task.ID, role)
	if err != nil {
		return nil
	}
	return &s.ID
}

// reported stops the AI process after its report has been saved.
func (u *Usecase) reported(taskID int64, err error) error {
	if err == nil {
		u.active.stop(taskID)
		u.notify.QueueMayRun()
		u.notify.TaskChanged(taskID)
	}
	return err
}

func (u *Usecase) CompleteDevelopment(ctx context.Context, in CompleteDevelopmentInput, now time.Time) (domain.Task, error) {
	var task domain.Task
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		var err error
		if task, err = u.runningTask(ctx); err != nil {
			return err
		}
		if task.Status != domain.StatusDeveloping && task.Status != domain.StatusFixing {
			return fmt.Errorf("%w: task %d is %s; complete_development is for development and fixes", ErrInvalidInput, task.ID, task.Status)
		}
		sid := u.sessionID(ctx, task, domain.RoleDev)
		if err := u.record(ctx, task.ID, sid, domain.EventAIResult,
			map[string]any{"pr_url": in.PRURL, "summary": in.Summary}, now); err != nil {
			return err
		}
		if task.Status == domain.StatusDeveloping {
			if err := u.createPR(ctx, task, in.PRURL, now); err != nil {
				return err
			}
		}
		project, err := u.db.GetProject(ctx, task.ProjectID)
		if err != nil {
			return err
		}
		hasCI, err := u.repo.HasWorkflows(ctx, u.projectDir(project), branch(task))
		if err != nil {
			return fmt.Errorf("check for CI workflows: %w", err)
		}
		next := domain.StatusReview
		if hasCI {
			next = domain.StatusCI
		}
		return u.move(ctx, &task, next, "", now)
	})
	return task, u.reported(task.ID, err)
}

func (u *Usecase) createPR(ctx context.Context, task domain.Task, prURL string, now time.Time) error {
	number, err := prNumber(prURL)
	if err != nil {
		return err
	}
	pr := domain.PullRequest{TaskID: task.ID, Number: number, URL: prURL, Branch: branch(task),
		State: domain.PRStateOpen, CreatedAt: now}
	if err := u.db.CreatePullRequest(ctx, &pr); err != nil {
		return err
	}
	return u.record(ctx, task.ID, nil, domain.EventPRCreated, map[string]any{"url": prURL, "number": number}, now)
}

// prNumber reads the number from https://github.com/<owner>/<name>/pull/<number>.
func prNumber(prURL string) (int, error) {
	parsed, err := url.Parse(prURL)
	if err != nil {
		return 0, fmt.Errorf("%w: %q is not a pull request URL", ErrInvalidInput, prURL)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "pull" {
		return 0, fmt.Errorf("%w: %q is not a pull request URL", ErrInvalidInput, prURL)
	}
	n, err := strconv.Atoi(parts[3])
	if err != nil {
		return 0, fmt.Errorf("%w: %q is not a pull request URL", ErrInvalidInput, prURL)
	}
	return n, nil
}

func (u *Usecase) CompleteReview(ctx context.Context, in CompleteReviewInput, now time.Time) (domain.Task, error) {
	if in.Result != domain.ReviewApproved && in.Result != domain.ReviewChangesRequested {
		return domain.Task{}, fmt.Errorf("%w: result must be approved or changes_requested", ErrInvalidInput)
	}
	var task domain.Task
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		var err error
		if task, err = u.runningTask(ctx); err != nil {
			return err
		}
		if task.Status != domain.StatusReviewing {
			return fmt.Errorf("%w: task %d is %s; complete_review is for reviews", ErrInvalidInput, task.ID, task.Status)
		}
		sid := u.sessionID(ctx, task, domain.RoleReview)
		if err := u.record(ctx, task.ID, sid, domain.EventAIResult,
			map[string]any{"result": string(in.Result), "comments": in.Comments}, now); err != nil {
			return err
		}
		round, err := u.addRound(ctx, task.ID, in.Result, in.Comments, now)
		if err != nil {
			return err
		}
		return u.move(ctx, &task, nextStatus(round, u.settings.MaxReviewRounds), "", now)
	})
	return task, u.reported(task.ID, err)
}

// ReportBlocked moves the running task to error at once with the AI's reason.
func (u *Usecase) ReportBlocked(ctx context.Context, reason string, now time.Time) (domain.Task, error) {
	var task domain.Task
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		var err error
		if task, err = u.runningTask(ctx); err != nil {
			return err
		}
		role := domain.RoleDev
		if task.Status == domain.StatusReviewing {
			role = domain.RoleReview
		}
		if err := u.record(ctx, task.ID, u.sessionID(ctx, task, role), domain.EventBlocked,
			map[string]any{"reason": reason}, now); err != nil {
			return err
		}
		return u.move(ctx, &task, domain.StatusError, "blocked: "+reason, now)
	})
	return task, u.reported(task.ID, err)
}
