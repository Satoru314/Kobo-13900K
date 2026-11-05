package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

// ListQueue lists the waiting tasks in the order they will run.
func (u *Usecase) ListQueue(ctx context.Context) ([]domain.Task, error) {
	tasks, err := u.db.ListQueue(ctx)
	if tasks == nil {
		tasks = []domain.Task{}
	}
	return tasks, err
}

func (u *Usecase) GetQueueState(ctx context.Context) (domain.QueueState, error) {
	return u.db.GetQueueState(ctx)
}

// PauseQueue stops the AI that is running and keeps the queue from starting
// another until ResumeQueue. The stopped task goes back to waiting without
// counting as a failure.
func (u *Usecase) PauseQueue(ctx context.Context) error {
	if err := u.db.SetQueuePaused(ctx, true); err != nil {
		return err
	}
	u.active.stopAny()
	return nil
}

func (u *Usecase) ResumeQueue(ctx context.Context) error {
	if err := u.db.SetQueuePaused(ctx, false); err != nil {
		return err
	}
	u.notify.QueueMayRun()
	return nil
}

// run is one AI run of a task.
type run struct {
	task    domain.Task
	project domain.Project
	role    domain.Role
	session domain.Session
	resume  bool
	workDir string
	prompt  string
	timeout time.Duration
}

// RunNext takes the next ready task, runs the AI on it until it reports back,
// exits or times out, and settles the outcome. It reports whether there was a
// task to run. When ctx is cancelled (kobo is stopping) the task is left
// running; RecoverRuns puts it back on the next start.
func (u *Usecase) RunNext(ctx context.Context, now time.Time) (bool, error) {
	r, ok, err := u.take(ctx, now)
	if err != nil || !ok {
		return false, err
	}
	slog.Info("run started", "task", r.task.ID, "role", r.role, "resume", r.resume)
	runErr := u.execute(ctx, r)
	if ctx.Err() != nil {
		return true, nil
	}
	slog.Info("run ended", "task", r.task.ID, "role", r.role, "err", runErr)
	return true, u.settle(ctx, r, runErr, time.Now())
}

// take starts the next ready task, if the queue is not paused and nothing is running.
func (u *Usecase) take(ctx context.Context, now time.Time) (run, bool, error) {
	var r run
	found := false
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		state, err := u.db.GetQueueState(ctx)
		if err != nil || state.Paused {
			return err
		}
		active, err := u.db.ListTasks(ctx, runningStatuses...)
		if err != nil || len(active) > 0 {
			return err
		}
		task, ok, err := u.db.NextRunnableTask(ctx, now, inOffPeak(u.settings.OffPeak, now))
		if err != nil || !ok {
			return err
		}
		from := task.Status
		if err := start(&task, now); err != nil {
			return err
		}
		if err := u.db.UpdateTask(ctx, task); err != nil {
			return err
		}
		if err := u.record(ctx, task.ID, nil, domain.EventStatusChanged,
			map[string]any{"from": string(from), "to": string(task.Status)}, now); err != nil {
			return err
		}
		if r, err = u.prepare(ctx, task, now); err != nil {
			return err
		}
		found = true
		return nil
	})
	if err == nil && found {
		u.notify.TaskChanged(r.task.ID)
	}
	return r, found, err
}

// prepare decides who runs the task with what prompt, and opens or reuses the session.
func (u *Usecase) prepare(ctx context.Context, task domain.Task, now time.Time) (run, error) {
	project, err := u.db.GetProject(ctx, task.ProjectID)
	if err != nil {
		return run{}, err
	}
	r := run{task: task, project: project, role: domain.RoleDev, timeout: u.settings.DevTimeout}
	if task.Status == domain.StatusReviewing {
		r.role, r.timeout = domain.RoleReview, u.settings.ReviewTimeout
	}
	r.workDir = filepath.Join(u.settings.WorktreesDir, project.Dir, "task-"+strconv.FormatInt(task.ID, 10)+"-"+string(r.role))

	session, err := u.db.GetSession(ctx, task.ID, r.role)
	switch {
	case err == nil:
		r.session, r.resume = session, true
	case errors.Is(err, domain.ErrNotFound):
		r.session = domain.Session{TaskID: task.ID, Role: r.role, Provider: u.ai.Provider(),
			ExternalSessionID: u.ai.NewSessionID(), CreatedAt: now}
		if err := u.db.CreateSession(ctx, &r.session); err != nil {
			return run{}, err
		}
	default:
		return run{}, err
	}

	if r.prompt, err = u.prompt(ctx, r); err != nil {
		return run{}, err
	}
	typ := domain.EventSessionStarted
	if r.resume {
		typ = domain.EventSessionResumed
	}
	sid := r.session.ID
	if err := u.record(ctx, task.ID, &sid, typ, map[string]any{
		"role": string(r.role), "provider": r.session.Provider, "external_session_id": r.session.ExternalSessionID,
		"work_dir": r.workDir}, now); err != nil {
		return run{}, err
	}
	if err := u.record(ctx, task.ID, &sid, domain.EventAIInput, map[string]any{"prompt": r.prompt}, now); err != nil {
		return run{}, err
	}
	return r, nil
}

func (u *Usecase) prompt(ctx context.Context, r run) (string, error) {
	if r.task.Status == domain.StatusDeveloping {
		return u.prompts.develop(r.task, r.project, r.workDir), nil
	}
	pr, err := u.db.GetPullRequestByTask(ctx, r.task.ID)
	if err != nil {
		return "", fmt.Errorf("task %d has no pull request: %w", r.task.ID, err)
	}
	rounds, err := u.db.ListReviewRounds(ctx, r.task.ID)
	if err != nil {
		return "", err
	}
	if r.task.Status == domain.StatusReviewing {
		return u.prompts.review(r.task, pr, r.workDir, len(rounds)+1), nil
	}
	if len(rounds) == 0 {
		return "", fmt.Errorf("task %d is in fix without a review round", r.task.ID)
	}
	return u.prompts.fix(pr, r.workDir, rounds[len(rounds)-1]), nil
}

// execute creates the worktree, runs the AI until it reports back, exits or
// times out, and always removes the worktree. Cancelling ctx stops the AI.
func (u *Usecase) execute(ctx context.Context, r run) error {
	repoDir := u.projectDir(r.project)
	br, base := branch(r.task), r.project.DefaultBranch
	switch r.task.Status {
	case domain.StatusFixing:
		base = br
	case domain.StatusReviewing:
		br, base = "", br
	}
	if err := u.repo.AddWorktree(ctx, repoDir, r.workDir, br, base); err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}
	defer u.repo.RemoveWorktree(context.WithoutCancel(ctx), repoDir, r.workDir)

	runCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	u.active.set(r.task.ID, cancel)
	defer u.active.clear()
	// A pause that came after take but before the run was registered stops it here.
	state, err := u.db.GetQueueState(ctx)
	if err != nil {
		return err
	}
	if state.Paused {
		cancel()
	}

	err = u.ai.Run(runCtx, RunRequest{
		Role: r.role, SessionID: r.session.ExternalSessionID, Resume: r.resume,
		WorkDir: r.workDir, Prompt: r.prompt,
		LogName: "task-" + strconv.FormatInt(r.task.ID, 10) + "-" + string(r.role),
	})
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("AI did not report back within %s", r.timeout)
	}
	return err
}

// settle handles a run that ended without the AI reporting back: the run is
// repeated after a delay, and the task goes to error after too many failures.
// A run stopped by a pause goes back to waiting without counting.
func (u *Usecase) settle(ctx context.Context, r run, runErr error, now time.Time) error {
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		task, err := u.db.GetTaskForUpdate(ctx, r.task.ID)
		if err != nil {
			return err
		}
		if !running(task.Status) {
			return nil // the AI reported back
		}
		state, err := u.db.GetQueueState(ctx)
		if err != nil {
			return err
		}
		if state.Paused {
			return u.putBack(ctx, &task, "paused by user", now)
		}
		msg := "AI exited without reporting back"
		if runErr != nil {
			msg = runErr.Error()
		}
		return u.fail(ctx, &task, msg, now)
	})
	u.notify.QueueMayRun()
	if err == nil {
		u.notify.TaskChanged(r.task.ID)
	}
	return err
}

// fail records a system error on a running task and requeues it or moves it to error.
func (u *Usecase) fail(ctx context.Context, task *domain.Task, msg string, now time.Time) error {
	if err := u.record(ctx, task.ID, nil, domain.EventSystemError, map[string]any{"message": msg}, now); err != nil {
		return err
	}
	events, err := u.db.ListEvents(ctx, task.ID)
	if err != nil {
		return err
	}
	if history(events).failuresInARow() > u.settings.MaxAutoRetries {
		return u.move(ctx, task, domain.StatusError, msg, now)
	}
	if err := requeue(task, now.Add(u.settings.RetryDelay), now); err != nil {
		return err
	}
	return u.db.UpdateTask(ctx, *task)
}

// putBack returns a running task to its waiting status without counting a failure.
func (u *Usecase) putBack(ctx context.Context, task *domain.Task, reason string, now time.Time) error {
	from := task.Status
	if err := requeue(task, now, now); err != nil {
		return err
	}
	if err := u.db.UpdateTask(ctx, *task); err != nil {
		return err
	}
	return u.record(ctx, task.ID, nil, domain.EventStatusChanged,
		map[string]any{"from": string(from), "to": string(task.Status), "reason": reason}, now)
}

// RecoverRuns puts back tasks left running when kobo stopped. It does not count as a failure.
func (u *Usecase) RecoverRuns(ctx context.Context, now time.Time) error {
	var recovered []int64
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		tasks, err := u.db.ListTasks(ctx, runningStatuses...)
		if err != nil {
			return err
		}
		for _, t := range tasks {
			if err := requeue(&t, now, now); err != nil {
				return err
			}
			if err := u.db.UpdateTask(ctx, t); err != nil {
				return err
			}
			recovered = append(recovered, t.ID)
		}
		return nil
	})
	if err == nil {
		for _, id := range recovered {
			u.notify.TaskChanged(id)
		}
	}
	return err
}
