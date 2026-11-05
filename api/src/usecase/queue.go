package usecase

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

// queueStatuses are every status a task can wait in the queue in, or be running in.
var queueStatuses = append(slices.Clone(domain.WaitingStatuses), domain.RunningStatuses...)

type QueueUsecase struct {
	tx       irepository.Transactor
	tasks    irepository.TaskRepository
	sessions irepository.SessionRepository
	prs      irepository.PullRequestRepository
	rounds   irepository.ReviewRoundRepository
	events   irepository.EventRepository
	state    irepository.QueueStateRepository
	projects *ProjectUsecase
	runner   irepository.Runner
	git      irepository.Git
	queue    domain.Queue
	settings Settings
	active   *ActiveRun
	signal   *QueueSignal
	changes  *Changes
	track    tracker
	prompts  prompts
}

func NewQueueUsecase(tx irepository.Transactor, tasks irepository.TaskRepository, sessions irepository.SessionRepository,
	prs irepository.PullRequestRepository, rounds irepository.ReviewRoundRepository, events irepository.EventRepository,
	state irepository.QueueStateRepository, projects *ProjectUsecase, runner irepository.Runner, git irepository.Git,
	queue domain.Queue, settings Settings, active *ActiveRun, signal *QueueSignal, changes *Changes) *QueueUsecase {
	return &QueueUsecase{tx: tx, tasks: tasks, sessions: sessions, prs: prs, rounds: rounds, events: events, state: state,
		projects: projects, runner: runner, git: git, queue: queue, settings: settings, active: active, signal: signal,
		changes: changes,
		track:   newTracker(tasks, events), prompts: prompts{maxRounds: settings.MaxReviewRounds}}
}

// Order lists the waiting tasks in the order they will run.
func (u *QueueUsecase) Order(ctx context.Context) ([]domain.Task, error) {
	tasks, err := u.tasks.List(ctx, queueStatuses...)
	if err != nil {
		return nil, err
	}
	out := u.queue.Order(tasks)
	if out == nil {
		out = []domain.Task{}
	}
	return out, nil
}

// Run is one AI run of a task, from Take to Settle.
type Run struct {
	Task    domain.Task
	Project domain.Project
	Role    domain.Role
	Session domain.Session
	Resume  bool
	WorkDir string
	Prompt  string
	Timeout time.Duration
}

// Take starts the next ready task, if nothing is running.
func (u *QueueUsecase) Take(ctx context.Context, now time.Time) (Run, bool, error) {
	var run Run
	found := false
	err := u.tx.InTx(ctx, func(ctx context.Context) error {
		if err := u.tx.LockQueue(ctx); err != nil {
			return err
		}
		paused, err := u.state.Paused(ctx)
		if err != nil || paused {
			return err
		}
		tasks, err := u.tasks.List(ctx, queueStatuses...)
		if err != nil {
			return err
		}
		for _, t := range tasks {
			if t.Status.Running() {
				return nil
			}
		}
		next, ok := u.queue.Next(tasks, now)
		if !ok {
			return nil
		}
		task, err := u.tasks.GetForUpdate(ctx, next.ID)
		if err != nil {
			return err
		}
		from := task.Status
		if err := task.Start(now); err != nil {
			return err
		}
		if err := u.tasks.Update(ctx, task); err != nil {
			return err
		}
		if err := u.track.record(ctx, task.ID, nil, domain.EventStatusChanged,
			map[string]any{"from": string(from), "to": string(task.Status)}, now); err != nil {
			return err
		}
		if run, err = u.prepare(ctx, task, now); err != nil {
			return err
		}
		found = true
		return nil
	})
	if err == nil && found {
		u.changes.Publish(run.Task.ID)
	}
	return run, found, err
}

// prepare decides who runs the task with what prompt, and opens or reuses the session.
func (u *QueueUsecase) prepare(ctx context.Context, task domain.Task, now time.Time) (Run, error) {
	project, err := u.projects.Get(ctx, task.ProjectID)
	if err != nil {
		return Run{}, err
	}
	run := Run{Task: task, Project: project, Role: domain.RoleDev, Timeout: u.settings.DevTimeout}
	if task.Status == domain.StatusReviewing {
		run.Role, run.Timeout = domain.RoleReview, u.settings.ReviewTimeout
	}
	run.WorkDir = filepath.Join(u.settings.WorktreesDir, project.Dir, "task-"+strconv.FormatInt(task.ID, 10)+"-"+string(run.Role))

	session, err := u.sessions.Get(ctx, task.ID, run.Role)
	switch {
	case err == nil:
		run.Session, run.Resume = session, true
	case errors.Is(err, irepository.ErrNotFound):
		run.Session = domain.Session{TaskID: task.ID, Role: run.Role, Provider: u.runner.Provider(),
			ExternalSessionID: u.runner.NewSessionID(), CreatedAt: now}
		if err := u.sessions.Create(ctx, &run.Session); err != nil {
			return Run{}, err
		}
	default:
		return Run{}, err
	}

	if run.Prompt, err = u.prompt(ctx, run); err != nil {
		return Run{}, err
	}
	typ := domain.EventSessionStarted
	if run.Resume {
		typ = domain.EventSessionResumed
	}
	sid := run.Session.ID
	if err := u.track.record(ctx, task.ID, &sid, typ, map[string]any{
		"role": string(run.Role), "provider": run.Session.Provider, "external_session_id": run.Session.ExternalSessionID,
		"work_dir": run.WorkDir}, now); err != nil {
		return Run{}, err
	}
	if err := u.track.record(ctx, task.ID, &sid, domain.EventAIInput, map[string]any{"prompt": run.Prompt}, now); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (u *QueueUsecase) prompt(ctx context.Context, run Run) (string, error) {
	if run.Task.Status == domain.StatusDeveloping {
		return u.prompts.develop(run.Task, run.Project, run.WorkDir), nil
	}
	pr, err := u.prs.GetByTask(ctx, run.Task.ID)
	if err != nil {
		return "", fmt.Errorf("task %d has no pull request: %w", run.Task.ID, err)
	}
	rounds, err := u.rounds.ListByTask(ctx, run.Task.ID)
	if err != nil {
		return "", err
	}
	if run.Task.Status == domain.StatusReviewing {
		return u.prompts.review(run.Task, pr, run.WorkDir, len(rounds)+1), nil
	}
	if len(rounds) == 0 {
		return "", fmt.Errorf("task %d is in fix without a review round", run.Task.ID)
	}
	return u.prompts.fix(pr, run.WorkDir, rounds[len(rounds)-1]), nil
}

// Execute creates the worktree, runs the AI until it reports back, exits or times out,
// and always removes the worktree. Cancelling ctx stops the AI.
func (u *QueueUsecase) Execute(ctx context.Context, run Run) error {
	repoDir := u.projects.Dir(run.Project)
	branch, base := run.Task.Branch(), run.Project.DefaultBranch
	switch run.Task.Status {
	case domain.StatusFixing:
		base = branch
	case domain.StatusReviewing:
		branch, base = "", branch
	}
	if err := u.git.AddWorktree(ctx, repoDir, run.WorkDir, branch, base); err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}
	defer u.git.RemoveWorktree(context.WithoutCancel(ctx), repoDir, run.WorkDir)

	runCtx, cancel := context.WithTimeout(ctx, run.Timeout)
	defer cancel()
	u.active.set(run.Task.ID, cancel)
	defer u.active.clear()
	// A pause that came after Take but before the run was registered stops it here.
	paused, err := u.state.Paused(ctx)
	if err != nil {
		return err
	}
	if paused {
		cancel()
	}

	err = u.runner.Run(runCtx, irepository.RunRequest{
		Role: run.Role, SessionID: run.Session.ExternalSessionID, Resume: run.Resume,
		WorkDir: run.WorkDir, Prompt: run.Prompt,
		LogName: "task-" + strconv.FormatInt(run.Task.ID, 10) + "-" + string(run.Role),
	})
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("AI did not report back within %s", run.Timeout)
	}
	return err
}

// Settle handles a run that ended without the AI reporting back: the run is
// repeated after a delay, and the task goes to error after too many failures.
func (u *QueueUsecase) Settle(ctx context.Context, run Run, runErr error, now time.Time) error {
	err := u.tx.InTx(ctx, func(ctx context.Context) error {
		task, err := u.tasks.GetForUpdate(ctx, run.Task.ID)
		if err != nil {
			return err
		}
		if !task.Status.Running() {
			return nil // the AI reported back
		}
		paused, err := u.state.Paused(ctx)
		if err != nil {
			return err
		}
		if paused {
			return u.putBack(ctx, &task, "paused by user", now)
		}
		msg := "AI exited without reporting back"
		if runErr != nil {
			msg = runErr.Error()
		}
		return u.fail(ctx, &task, msg, now)
	})
	u.signal.Notify()
	if err == nil {
		u.changes.Publish(run.Task.ID)
	}
	return err
}

// fail records a system error on a running task and requeues it or moves it to error.
func (u *QueueUsecase) fail(ctx context.Context, task *domain.Task, msg string, now time.Time) error {
	if err := u.track.record(ctx, task.ID, nil, domain.EventSystemError, map[string]any{"message": msg}, now); err != nil {
		return err
	}
	events, err := u.events.ListByTask(ctx, task.ID)
	if err != nil {
		return err
	}
	if history(events).failuresInARow() > u.settings.MaxAutoRetries {
		return u.track.move(ctx, task, domain.StatusError, msg, now)
	}
	if err := task.Requeue(now.Add(u.settings.RetryDelay), now); err != nil {
		return err
	}
	return u.tasks.Update(ctx, *task)
}

// putBack returns a running task to its waiting status without counting a failure.
func (u *QueueUsecase) putBack(ctx context.Context, task *domain.Task, reason string, now time.Time) error {
	from := task.Status
	if err := task.Requeue(now, now); err != nil {
		return err
	}
	if err := u.tasks.Update(ctx, *task); err != nil {
		return err
	}
	return u.track.record(ctx, task.ID, nil, domain.EventStatusChanged,
		map[string]any{"from": string(from), "to": string(task.Status), "reason": reason}, now)
}

// Paused reports whether the user paused the AI.
func (u *QueueUsecase) Paused(ctx context.Context) (bool, error) {
	return u.state.Paused(ctx)
}

// Pause stops the AI that is running and keeps the queue from starting another
// until Resume. The stopped task goes back to waiting without counting a failure.
func (u *QueueUsecase) Pause(ctx context.Context) error {
	if err := u.state.SetPaused(ctx, true); err != nil {
		return err
	}
	u.active.stopAny()
	return nil
}

// Resume lets the queue start tasks again.
func (u *QueueUsecase) Resume(ctx context.Context) error {
	if err := u.state.SetPaused(ctx, false); err != nil {
		return err
	}
	u.signal.Notify()
	return nil
}

// Recover puts back tasks left running when kobo stopped. It does not count as a failure.
func (u *QueueUsecase) Recover(ctx context.Context, now time.Time) error {
	var recovered []int64
	err := u.tx.InTx(ctx, func(ctx context.Context) error {
		tasks, err := u.tasks.List(ctx, queueStatuses...)
		if err != nil {
			return err
		}
		for _, t := range tasks {
			if !t.Status.Running() {
				continue
			}
			if err := t.Requeue(now, now); err != nil {
				return err
			}
			if err := u.tasks.Update(ctx, t); err != nil {
				return err
			}
			recovered = append(recovered, t.ID)
		}
		return nil
	})
	if err == nil {
		for _, id := range recovered {
			u.changes.Publish(id)
		}
	}
	return err
}
