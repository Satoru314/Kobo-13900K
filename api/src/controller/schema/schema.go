// Package schema is the shape of what the REST API and the MCP servers send and
// receive, and its conversion from the usecase's results.
package schema

import (
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

type Task struct {
	ID          int64      `json:"id"`
	ProjectID   int64      `json:"project_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Mode        string     `json:"mode"`
	EnqueuedAt  *time.Time `json:"enqueued_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type TaskSummary struct {
	Task
	Project  string `json:"project"` // directory name
	PRURL    string `json:"pr_url,omitempty"`
	PRNumber int    `json:"pr_number,omitempty"`
	CI       string `json:"ci,omitempty"` // pending / success / failure
}

type TaskDetail struct {
	Task        Task          `json:"task"`
	Project     Project       `json:"project"`
	PullRequest *PullRequest  `json:"pull_request"`
	Rounds      []ReviewRound `json:"review_rounds"`
	Sessions    []Session     `json:"sessions"`
	LastError   string        `json:"last_error,omitempty"`
}

type Session struct {
	Role      string    `json:"role"`
	AI        string    `json:"ai"`
	AILabel   string    `json:"ai_label"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
}

type Project struct {
	ID            int64     `json:"id"`
	Dir           string    `json:"dir"`
	Owner         string    `json:"owner"`
	Name          string    `json:"name"`
	DefaultBranch string    `json:"default_branch"`
	CreatedAt     time.Time `json:"created_at"`
}

type ProjectCandidate struct {
	Dir        string `json:"dir"`
	Registered bool   `json:"registered"`
	Problem    string `json:"problem,omitempty"` // not_git / not_github
}

type PullRequest struct {
	ID        int64      `json:"id"`
	TaskID    int64      `json:"task_id"`
	Number    int        `json:"number"`
	URL       string     `json:"url"`
	Branch    string     `json:"branch"`
	State     string     `json:"state"`
	CreatedAt time.Time  `json:"created_at"`
	MergedAt  *time.Time `json:"merged_at"`
}

type ReviewRound struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	RoundNo   int       `json:"round_no"`
	Result    string    `json:"result"`
	Comments  string    `json:"comments"`
	CreatedAt time.Time `json:"created_at"`
}

type Event struct {
	ID        int64          `json:"id"`
	TaskID    int64          `json:"task_id"`
	SessionID *int64         `json:"session_id"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

type QueueState struct {
	Paused bool `json:"paused"`
}

type AIModel struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type AISetting struct {
	DevAI          string    `json:"dev_ai"`
	ReviewAI       string    `json:"review_ai"`
	SeparateReview bool      `json:"separate_review"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type AIView struct {
	Models  []AIModel `json:"models"`
	Setting AISetting `json:"setting"`
}

// ChangeEvent is one message on GET /events.
type ChangeEvent struct {
	TaskID int64 `json:"task_id"`
}

type Health struct {
	Status string `json:"status"`
}

type Error struct {
	Error string `json:"error"`
}

// Requests. The jsonschema tags describe the MCP tools' arguments.

type CreateTaskRequest struct {
	Project     string `json:"project" jsonschema:"directory name of the target repository, e.g. emela"`
	Title       string `json:"title" jsonschema:"short title shown in the task list"`
	Description string `json:"description" jsonschema:"full instructions for the developing AI"`
	Mode        string `json:"mode,omitempty" jsonschema:"priority, immediate (default) or off_peak"`
}

type EditTaskRequest struct {
	ID          int64   `json:"id,omitempty" jsonschema:"task id (MCP only; REST takes it from the path)"`
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Mode        *string `json:"mode,omitempty" jsonschema:"priority, immediate or off_peak"`
}

type ListTasksRequest struct {
	Statuses      []string `json:"statuses,omitempty" jsonschema:"only tasks in these statuses"`
	IncludeHidden bool     `json:"include_hidden,omitempty" jsonschema:"also list finished and cancelled tasks"`
}

type TaskIDRequest struct {
	ID int64 `json:"id" jsonschema:"task id"`
}

type AddProjectRequest struct {
	Dir string `json:"dir"`
}

type CompleteDevelopmentRequest struct {
	PRURL   string `json:"pr_url" jsonschema:"URL of the pull request you created or pushed to"`
	Summary string `json:"summary" jsonschema:"what you did, in a few lines"`
}

type CompleteReviewRequest struct {
	Result   string `json:"result" jsonschema:"approved or changes_requested"`
	Comments string `json:"comments,omitempty" jsonschema:"the issues that must be fixed, when changes_requested"`
}

type ReportBlockedRequest struct {
	Reason string `json:"reason" jsonschema:"why the task cannot go on and what a human has to do"`
}

type AISettingRequest struct {
	DevAI          string `json:"dev_ai" jsonschema:"catalog id of the model used for development"`
	ReviewAI       string `json:"review_ai,omitempty" jsonschema:"catalog id used for review when separate_review is true"`
	SeparateReview bool   `json:"separate_review" jsonschema:"whether review uses a different model from development"`
}

// MCP results must be objects, so lists are wrapped.

type TaskList struct {
	Tasks []TaskSummary `json:"tasks"`
}

type EventList struct {
	Events []Event `json:"events"`
}

type ProjectList struct {
	Projects []Project `json:"projects"`
}

type QueueList struct {
	Tasks []Task `json:"tasks"`
}

// Conversions from the usecase's results.

func FromTask(t domain.Task) Task {
	return Task{ID: t.ID, ProjectID: t.ProjectID, Title: t.Title, Description: t.Description, Status: string(t.Status),
		Mode: string(t.Mode), EnqueuedAt: t.EnqueuedAt, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

func FromTaskSummary(s usecase.TaskSummary) TaskSummary {
	return TaskSummary{Task: FromTask(s.Task), Project: s.Project, PRURL: s.PRURL, PRNumber: s.PRNumber, CI: string(s.CI)}
}

func FromTaskDetail(d usecase.TaskDetail) TaskDetail {
	out := TaskDetail{Task: FromTask(d.Task), Project: FromProject(d.Project), Rounds: Map(d.Rounds, FromReviewRound),
		Sessions: Map(d.Sessions, FromSessionInfo), LastError: d.LastError}
	if d.PullRequest != nil {
		pr := FromPullRequest(*d.PullRequest)
		out.PullRequest = &pr
	}
	return out
}

func FromSessionInfo(s usecase.SessionInfo) Session {
	return Session{Role: string(s.Role), AI: s.AI, AILabel: s.AILabel, Provider: s.Provider,
		Model: s.Model, CreatedAt: s.CreatedAt}
}

func FromAIModel(m usecase.AIModel) AIModel {
	return AIModel{ID: m.ID, Label: m.Label, Provider: m.Provider, Model: m.Model}
}

func FromAISetting(s domain.AISetting) AISetting {
	return AISetting{DevAI: s.DevAI, ReviewAI: s.ReviewAI, SeparateReview: s.SeparateReview, UpdatedAt: s.UpdatedAt}
}

func FromAIView(v usecase.AIView) AIView {
	return AIView{Models: Map(v.Models, FromAIModel), Setting: FromAISetting(v.Setting)}
}

func FromProject(p domain.Project) Project {
	return Project{ID: p.ID, Dir: p.Dir, Owner: p.Owner, Name: p.Name, DefaultBranch: p.DefaultBranch, CreatedAt: p.CreatedAt}
}

func FromProjectCandidate(c usecase.ProjectCandidate) ProjectCandidate {
	return ProjectCandidate{Dir: c.Dir, Registered: c.Registered, Problem: string(c.Problem)}
}

func FromPullRequest(p domain.PullRequest) PullRequest {
	return PullRequest{ID: p.ID, TaskID: p.TaskID, Number: p.Number, URL: p.URL, Branch: p.Branch,
		State: string(p.State), CreatedAt: p.CreatedAt, MergedAt: p.MergedAt}
}

func FromReviewRound(r domain.ReviewRound) ReviewRound {
	return ReviewRound{ID: r.ID, TaskID: r.TaskID, RoundNo: r.RoundNo, Result: string(r.Result), Comments: r.Comments, CreatedAt: r.CreatedAt}
}

func FromEvent(e domain.Event) Event {
	return Event{ID: e.ID, TaskID: e.TaskID, SessionID: e.SessionID, Type: string(e.Type), Payload: e.Payload, CreatedAt: e.CreatedAt}
}

func FromQueueState(s domain.QueueState) QueueState {
	return QueueState{Paused: s.Paused}
}

// Conversions of requests into usecase inputs.

func (r CreateTaskRequest) Input() usecase.CreateTaskInput {
	return usecase.CreateTaskInput{Project: r.Project, Title: r.Title, Description: r.Description, Mode: domain.Mode(r.Mode)}
}

func (r EditTaskRequest) Input(id int64) usecase.EditTaskInput {
	in := usecase.EditTaskInput{ID: id, Title: r.Title, Description: r.Description}
	if r.Mode != nil {
		m := domain.Mode(*r.Mode)
		in.Mode = &m
	}
	return in
}

func (r ListTasksRequest) Input() usecase.ListTasksInput {
	return usecase.ListTasksInput{Statuses: Map(r.Statuses, func(s string) domain.Status { return domain.Status(s) }), IncludeHidden: r.IncludeHidden}
}

func (r CompleteDevelopmentRequest) Input() usecase.CompleteDevelopmentInput {
	return usecase.CompleteDevelopmentInput{PRURL: r.PRURL, Summary: r.Summary}
}

func (r CompleteReviewRequest) Input() usecase.CompleteReviewInput {
	return usecase.CompleteReviewInput{Result: domain.ReviewResult(r.Result), Comments: r.Comments}
}

func (r AISettingRequest) Input() usecase.SetAISettingInput {
	return usecase.SetAISettingInput{DevAI: r.DevAI, ReviewAI: r.ReviewAI, SeparateReview: r.SeparateReview}
}

// Map converts a list, keeping an empty list empty rather than null in JSON.
func Map[A, B any](in []A, f func(A) B) []B {
	out := make([]B, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}
