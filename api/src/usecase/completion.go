package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

var ErrNothingRunning = errors.New("no task is running")

// CompletionUsecase applies what the running AI reports. Runs are serial, so a
// report always belongs to the one running task.
type CompletionUsecase struct {
	tx       irepository.Transactor
	tasks    irepository.TaskRepository
	sessions irepository.SessionRepository
	prs      irepository.PullRequestRepository
	rounds   irepository.ReviewRoundRepository
	events   irepository.EventRepository
	projects *ProjectUsecase
	git      irepository.Git
	settings Settings
	active   *ActiveRun
	signal   *QueueSignal
	changes  *Changes
	track    tracker
	book     roundBook
}

func NewCompletionUsecase(tx irepository.Transactor, tasks irepository.TaskRepository, sessions irepository.SessionRepository,
	prs irepository.PullRequestRepository, rounds irepository.ReviewRoundRepository, events irepository.EventRepository,
	projects *ProjectUsecase, git irepository.Git, settings Settings, active *ActiveRun, signal *QueueSignal, changes *Changes) *CompletionUsecase {
	return &CompletionUsecase{tx: tx, tasks: tasks, sessions: sessions, prs: prs, rounds: rounds, events: events,
		projects: projects, git: git, settings: settings, active: active, signal: signal, changes: changes,
		track: newTracker(tasks, events),
		book: newRoundBook(rounds, newTracker(tasks, events))}
}

// running returns the running task, locked.
func (u *CompletionUsecase) running(ctx context.Context) (domain.Task, error) {
	tasks, err := u.tasks.List(ctx, domain.RunningStatuses...)
	if err != nil {
		return domain.Task{}, err
	}
	for _, t := range tasks {
		if t.Status.Running() {
			return u.tasks.GetForUpdate(ctx, t.ID)
		}
	}
	return domain.Task{}, ErrNothingRunning
}

func (u *CompletionUsecase) sessionID(ctx context.Context, task domain.Task, role domain.Role) *int64 {
	s, err := u.sessions.Get(ctx, task.ID, role)
	if err != nil {
		return nil
	}
	return &s.ID
}

// finish stops the AI process after a report has been saved.
func (u *CompletionUsecase) finish(taskID int64, err error) error {
	if err == nil {
		u.active.stop(taskID)
		u.signal.Notify()
		u.changes.Publish(taskID)
	}
	return err
}

type CompleteDevelopmentInput struct {
	PRURL   string `json:"pr_url" jsonschema:"URL of the pull request you created or pushed to"`
	Summary string `json:"summary" jsonschema:"what you did, in a few lines"`
}

func (u *CompletionUsecase) CompleteDevelopment(ctx context.Context, in CompleteDevelopmentInput, now time.Time) (domain.Task, error) {
	var task domain.Task
	err := u.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if task, err = u.running(ctx); err != nil {
			return err
		}
		if task.Status != domain.StatusDeveloping && task.Status != domain.StatusFixing {
			return fmt.Errorf("%w: task %d is %s; complete_development is for development and fixes", ErrInvalidInput, task.ID, task.Status)
		}
		sid := u.sessionID(ctx, task, domain.RoleDev)
		if err := u.track.record(ctx, task.ID, sid, domain.EventAIResult,
			map[string]any{"pr_url": in.PRURL, "summary": in.Summary}, now); err != nil {
			return err
		}
		if task.Status == domain.StatusDeveloping {
			if err := u.createPR(ctx, task, in.PRURL, now); err != nil {
				return err
			}
		}
		project, err := u.projects.Get(ctx, task.ProjectID)
		if err != nil {
			return err
		}
		hasCI, err := u.git.HasWorkflows(ctx, u.projects.Dir(project), task.Branch())
		if err != nil {
			return fmt.Errorf("check for CI workflows: %w", err)
		}
		next := domain.StatusReview
		if hasCI {
			next = domain.StatusCI
		}
		return u.track.move(ctx, &task, next, "", now)
	})
	return task, u.finish(task.ID, err)
}

func (u *CompletionUsecase) createPR(ctx context.Context, task domain.Task, prURL string, now time.Time) error {
	number, err := u.prNumber(prURL)
	if err != nil {
		return err
	}
	pr := domain.PullRequest{TaskID: task.ID, Number: number, URL: prURL, Branch: task.Branch(),
		State: domain.PRStateOpen, CreatedAt: now}
	if err := u.prs.Create(ctx, &pr); err != nil {
		return err
	}
	return u.track.record(ctx, task.ID, nil, domain.EventPRCreated, map[string]any{"url": prURL, "number": number}, now)
}

// prNumber reads the number from https://github.com/<owner>/<name>/pull/<number>.
func (u *CompletionUsecase) prNumber(prURL string) (int, error) {
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

type CompleteReviewInput struct {
	Result   domain.ReviewResult `json:"result" jsonschema:"approved or changes_requested"`
	Comments string              `json:"comments,omitempty" jsonschema:"the issues that must be fixed, when changes_requested"`
}

func (u *CompletionUsecase) CompleteReview(ctx context.Context, in CompleteReviewInput, now time.Time) (domain.Task, error) {
	if in.Result != domain.ReviewApproved && in.Result != domain.ReviewChangesRequested {
		return domain.Task{}, fmt.Errorf("%w: result must be approved or changes_requested", ErrInvalidInput)
	}
	var task domain.Task
	err := u.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if task, err = u.running(ctx); err != nil {
			return err
		}
		if task.Status != domain.StatusReviewing {
			return fmt.Errorf("%w: task %d is %s; complete_review is for reviews", ErrInvalidInput, task.ID, task.Status)
		}
		sid := u.sessionID(ctx, task, domain.RoleReview)
		if err := u.track.record(ctx, task.ID, sid, domain.EventAIResult,
			map[string]any{"result": string(in.Result), "comments": in.Comments}, now); err != nil {
			return err
		}
		round, err := u.book.add(ctx, task.ID, in.Result, in.Comments, now)
		if err != nil {
			return err
		}
		return u.track.move(ctx, &task, round.NextStatus(u.settings.MaxReviewRounds), "", now)
	})
	return task, u.finish(task.ID, err)
}

type ReportBlockedInput struct {
	Reason string `json:"reason" jsonschema:"why the task cannot go on and what a human has to do"`
}

func (u *CompletionUsecase) ReportBlocked(ctx context.Context, in ReportBlockedInput, now time.Time) (domain.Task, error) {
	var task domain.Task
	err := u.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if task, err = u.running(ctx); err != nil {
			return err
		}
		role := domain.RoleDev
		if task.Status == domain.StatusReviewing {
			role = domain.RoleReview
		}
		if err := u.track.record(ctx, task.ID, u.sessionID(ctx, task, role), domain.EventBlocked,
			map[string]any{"reason": in.Reason}, now); err != nil {
			return err
		}
		return u.track.move(ctx, &task, domain.StatusError, "blocked: "+in.Reason, now)
	})
	return task, u.finish(task.ID, err)
}
