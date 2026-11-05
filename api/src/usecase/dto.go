package usecase

import (
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

// Inputs of more than one value, and outputs that no domain type has the shape of.

type CreateTaskInput struct {
	Project     string // directory name under the projects directory
	Title       string
	Description string
	Mode        domain.Mode // empty means immediate
}

type ListTasksInput struct {
	Statuses      []domain.Status // only these; empty means all
	IncludeHidden bool            // also finished and cancelled
}

type EditTaskInput struct {
	ID          int64
	Title       *string
	Description *string
	Mode        *domain.Mode
}

type CompleteDevelopmentInput struct {
	PRURL   string
	Summary string
}

type CompleteReviewInput struct {
	Result   domain.ReviewResult
	Comments string
}

type SetAISettingInput struct {
	DevAI          string
	ReviewAI       string
	SeparateReview bool
}

// AIView is what the settings screen shows: the catalog and the current choice.
type AIView struct {
	Models  []AIModel
	Setting domain.AISetting
}

// SessionInfo is one role's AI conversation on a task, with the catalog label
// resolved for display.
type SessionInfo struct {
	Role      domain.Role
	AI        string
	AILabel   string
	Provider  string
	Model     string
	CreatedAt time.Time
}

// TaskSummary is a task with what its card in the list shows.
type TaskSummary struct {
	Task     domain.Task
	Project  string // directory name
	PRURL    string
	PRNumber int
	// CI is the state of the latest CI run, empty when CI never ran.
	CI CIState
}

type TaskDetail struct {
	Task        domain.Task
	Project     domain.Project
	PullRequest *domain.PullRequest
	Rounds      []domain.ReviewRound
	Sessions    []SessionInfo
	// LastError is the latest system error or blocked reason while the task is in error.
	LastError string
}

type CandidateProblem string

const (
	ProblemNotGit    CandidateProblem = "not_git"
	ProblemNotGitHub CandidateProblem = "not_github"
)

// ProjectCandidate is a directory under the projects directory.
type ProjectCandidate struct {
	Dir        string
	Registered bool
	// Problem says why it cannot be registered.
	Problem CandidateProblem
}
