package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

type CIUsecase struct {
	tx       irepository.Transactor
	tasks    irepository.TaskRepository
	prs      irepository.PullRequestRepository
	projects *ProjectUsecase
	github   irepository.GitHub
	settings Settings
	signal   *QueueSignal
	track    tracker
	book     roundBook
}

func NewCIUsecase(tx irepository.Transactor, tasks irepository.TaskRepository, prs irepository.PullRequestRepository,
	rounds irepository.ReviewRoundRepository, events irepository.EventRepository, projects *ProjectUsecase,
	github irepository.GitHub, settings Settings, signal *QueueSignal) *CIUsecase {
	track := newTracker(tasks, events)
	return &CIUsecase{tx: tx, tasks: tasks, prs: prs, projects: projects, github: github, settings: settings,
		signal: signal, track: track, book: newRoundBook(rounds, track)}
}

// Check looks at the CI of every task in ci and moves the finished ones on.
// A failing lookup is left for the next check.
func (u *CIUsecase) Check(ctx context.Context, now time.Time) error {
	tasks, err := u.tasks.List(ctx, domain.StatusCI)
	if err != nil {
		return err
	}
	var errs []error
	for _, t := range tasks {
		if err := u.checkTask(ctx, t.ID, now); err != nil {
			errs = append(errs, fmt.Errorf("task %d: %w", t.ID, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("ci check: %v", errs)
	}
	return nil
}

func (u *CIUsecase) checkTask(ctx context.Context, taskID int64, now time.Time) error {
	pr, err := u.prs.GetByTask(ctx, taskID)
	if err != nil {
		return err
	}
	task, err := u.tasks.Get(ctx, taskID)
	if err != nil {
		return err
	}
	project, err := u.projects.Get(ctx, task.ProjectID)
	if err != nil {
		return err
	}
	ci, err := u.github.CI(ctx, project.FullName(), pr.Number)
	if err != nil || ci.State == irepository.CIPending {
		return err
	}
	err = u.tx.InTx(ctx, func(ctx context.Context) error {
		task, err := u.tasks.GetForUpdate(ctx, taskID)
		if err != nil || task.Status != domain.StatusCI {
			return err
		}
		if err := u.track.record(ctx, taskID, nil, domain.EventCIResult,
			map[string]any{"state": string(ci.State), "summary": ci.Summary}, now); err != nil {
			return err
		}
		if ci.State == irepository.CISuccess {
			return u.track.move(ctx, &task, domain.StatusReviewing, "", now)
		}
		round, err := u.book.add(ctx, taskID, domain.ReviewCIFailed, ci.Summary, now)
		if err != nil {
			return err
		}
		return u.track.move(ctx, &task, round.NextStatus(u.settings.MaxReviewRounds), "", now)
	})
	if err == nil {
		u.signal.Notify()
	}
	return err
}
