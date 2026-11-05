package usecase

import (
	"context"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

// DB is the database. Calls made with the ctx given to InTx's fn take part in
// that transaction. Getters return domain.ErrNotFound for a missing row.
type DB interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error

	CreateProject(ctx context.Context, p *domain.Project) error // returns the existing row when dir is taken
	GetProject(ctx context.Context, id int64) (domain.Project, error)
	GetProjectByDir(ctx context.Context, dir string) (domain.Project, error)
	ListProjects(ctx context.Context) ([]domain.Project, error)

	CreateTask(ctx context.Context, t *domain.Task) error
	GetTask(ctx context.Context, id int64) (domain.Task, error)
	GetTaskForUpdate(ctx context.Context, id int64) (domain.Task, error)
	ListTasks(ctx context.Context, statuses ...domain.Status) ([]domain.Task, error)
	UpdateTask(ctx context.Context, t domain.Task) error
	// NextRunnableTask locks and returns the waiting task the queue takes next at
	// now. inOffPeak says whether off_peak tasks may start.
	NextRunnableTask(ctx context.Context, now time.Time, inOffPeak bool) (domain.Task, bool, error)
	// ListQueue returns the waiting tasks in the order the queue takes them.
	ListQueue(ctx context.Context) ([]domain.Task, error)

	CreatePullRequest(ctx context.Context, pr *domain.PullRequest) error
	GetPullRequestByTask(ctx context.Context, taskID int64) (domain.PullRequest, error)
	UpdatePullRequest(ctx context.Context, pr domain.PullRequest) error

	CreateSession(ctx context.Context, s *domain.Session) error
	GetSession(ctx context.Context, taskID int64, role domain.Role) (domain.Session, error)
	ListSessions(ctx context.Context, taskID int64) ([]domain.Session, error)

	GetAISetting(ctx context.Context) (domain.AISetting, error)
	SetAISetting(ctx context.Context, s domain.AISetting) error

	CreateReviewRound(ctx context.Context, r *domain.ReviewRound) error
	ListReviewRounds(ctx context.Context, taskID int64) ([]domain.ReviewRound, error) // in round order

	CreateEvent(ctx context.Context, e *domain.Event) error
	ListEvents(ctx context.Context, taskID int64) ([]domain.Event, error) // oldest first

	GetQueueState(ctx context.Context) (domain.QueueState, error)
	SetQueuePaused(ctx context.Context, paused bool) error
}

type RemoteInfo struct {
	Owner         string
	Name          string
	DefaultBranch string
}

type CIState string

const (
	CIPending CIState = "pending"
	CISuccess CIState = "success"
	CIFailure CIState = "failure"
)

type CIResult struct {
	State CIState
	// Summary lists the failed checks with links when State is CIFailure.
	Summary string
}

type PRInfo struct {
	State    domain.PRState
	MergedAt *time.Time
}

// RepoHost is the target repositories: their checkouts through git, their pull
// requests and CI through GitHub.
type RepoHost interface {
	// Remote reads the GitHub owner, name and default branch from origin.
	Remote(ctx context.Context, repoDir string) (RemoteInfo, error)
	// GitHubOrigin reads owner and name from origin without going to the network.
	// ok is false when there is no origin or it is not on GitHub.
	GitHubOrigin(ctx context.Context, repoDir string) (owner, name string, ok bool, err error)
	// AddWorktree fetches origin and creates a worktree at path from origin/base.
	// With branch set, the worktree checks out branch reset to origin/base; otherwise it is detached.
	AddWorktree(ctx context.Context, repoDir, path, branch, base string) error
	// RemoveWorktree deletes the worktree at path, even if it has changes.
	RemoveWorktree(ctx context.Context, repoDir, path string) error
	// HasWorkflows reports whether origin/branch contains GitHub Actions workflows.
	HasWorkflows(ctx context.Context, repoDir, branch string) (bool, error)
	// CI reports the combined state of the checks on the PR's latest commit.
	CI(ctx context.Context, fullName string, number int) (CIResult, error)
	PullRequest(ctx context.Context, fullName string, number int) (PRInfo, error)
}

type RunRequest struct {
	Role domain.Role
	// AI is the catalog id, for logs and events. Model is what the provider is
	// asked to use. Both are fixed on the session when it is created.
	AI        string
	Model     string
	SessionID string
	// Resume continues SessionID instead of starting a new conversation with it.
	Resume  bool
	WorkDir string
	Prompt  string
	// LogName names the file the AI's output is written to.
	LogName string
}

// AI starts an AI on a prompt. Each provider has its own implementation.
type AI interface {
	Provider() string
	NewSessionID() string
	// Run blocks until the AI process exits. Cancelling ctx kills the process.
	Run(ctx context.Context, req RunRequest) error
}

// Notifier passes on what changed, after the transaction that changed it commits.
type Notifier interface {
	// TaskChanged tells the GUI to fetch the task again.
	TaskChanged(taskID int64)
	// QueueMayRun wakes the queue worker: a task may be ready.
	QueueMayRun()
}
