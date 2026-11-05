// Package mcp exposes kobo as MCP servers: one for the user, one for the running AI.
package mcp

import (
	"context"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/schema"
	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

// Usecase is what the MCP servers need from the usecase.
type Usecase interface {
	CreateTask(ctx context.Context, in usecase.CreateTaskInput, now time.Time) (domain.Task, error)
	ListTasks(ctx context.Context, in usecase.ListTasksInput) ([]usecase.TaskSummary, error)
	GetTask(ctx context.Context, id int64) (usecase.TaskDetail, error)
	ListTaskEvents(ctx context.Context, id int64) ([]domain.Event, error)
	EditTask(ctx context.Context, in usecase.EditTaskInput, now time.Time) (domain.Task, error)
	CancelTask(ctx context.Context, id int64, now time.Time) (domain.Task, error)
	RetryTask(ctx context.Context, id int64, now time.Time) (domain.Task, error)
	ListProjects(ctx context.Context) ([]domain.Project, error)
	ListQueue(ctx context.Context) ([]domain.Task, error)
	CompleteDevelopment(ctx context.Context, in usecase.CompleteDevelopmentInput, now time.Time) (domain.Task, error)
	CompleteReview(ctx context.Context, in usecase.CompleteReviewInput, now time.Time) (domain.Task, error)
	ReportBlocked(ctx context.Context, reason string, now time.Time) (domain.Task, error)
}

type tools struct {
	uc Usecase
}

type noInput struct{}

// UserHandler serves the tools for issuing and following tasks.
func UserHandler(uc Usecase) http.Handler {
	t := &tools{uc: uc}
	srv := mcp.NewServer(&mcp.Implementation{Name: "kobo", Version: "0.1.0"}, nil)
	mcp.AddTool(srv, &mcp.Tool{Name: "create_task", Description: "Issue a development task. The AI develops it, opens a PR and gets it reviewed. project is a directory name under the projects directory; unknown ones are registered from their git remote."}, t.createTask)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_tasks", Description: "List tasks. Finished and cancelled tasks are left out unless include_hidden is set or statuses are given."}, t.listTasks)
	mcp.AddTool(srv, &mcp.Tool{Name: "get_task", Description: "Show a task with its project, pull request, review rounds and, when in error, the last error."}, t.getTask)
	mcp.AddTool(srv, &mcp.Tool{Name: "get_task_events", Description: "Show the run history of a task, oldest first."}, t.getTaskEvents)
	mcp.AddTool(srv, &mcp.Tool{Name: "update_task", Description: "Change title, description or mode of a task that is waiting in develop or in error."}, t.updateTask)
	mcp.AddTool(srv, &mcp.Tool{Name: "cancel_task", Description: "Cancel a task that has not started yet."}, t.cancelTask)
	mcp.AddTool(srv, &mcp.Tool{Name: "retry_task", Description: "Put a task in error back into the queue at the stage it failed in."}, t.retryTask)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_projects", Description: "List registered projects (target repositories)."}, t.listProjects)
	mcp.AddTool(srv, &mcp.Tool{Name: "get_queue", Description: "List waiting tasks in the order they will run."}, t.getQueue)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
}

// AgentHandler serves the tools the running AI uses to report back.
func AgentHandler(uc Usecase) http.Handler {
	t := &tools{uc: uc}
	srv := mcp.NewServer(&mcp.Implementation{Name: "kobo-agent", Version: "0.1.0"}, nil)
	mcp.AddTool(srv, &mcp.Tool{Name: "complete_development", Description: "Report that development or a fix is done and pushed. Call once, with the PR URL, then stop."}, t.completeDevelopment)
	mcp.AddTool(srv, &mcp.Tool{Name: "complete_review", Description: "Report the review result: approved, or changes_requested with the issues in comments. Call once, then stop."}, t.completeReview)
	mcp.AddTool(srv, &mcp.Tool{Name: "report_blocked", Description: "Report that the task cannot go on, with the reason and what a human has to do. Call once, then stop."}, t.reportBlocked)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
}

func (t *tools) createTask(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateTaskRequest) (*mcp.CallToolResult, schema.Task, error) {
	task, err := t.uc.CreateTask(ctx, in.Input(), time.Now())
	return nil, schema.FromTask(task), err
}

func (t *tools) listTasks(ctx context.Context, _ *mcp.CallToolRequest, in schema.ListTasksRequest) (*mcp.CallToolResult, schema.TaskList, error) {
	tasks, err := t.uc.ListTasks(ctx, in.Input())
	return nil, schema.TaskList{Tasks: schema.Map(tasks, schema.FromTaskSummary)}, err
}

func (t *tools) getTask(ctx context.Context, _ *mcp.CallToolRequest, in schema.TaskIDRequest) (*mcp.CallToolResult, schema.TaskDetail, error) {
	detail, err := t.uc.GetTask(ctx, in.ID)
	return nil, schema.FromTaskDetail(detail), err
}

func (t *tools) getTaskEvents(ctx context.Context, _ *mcp.CallToolRequest, in schema.TaskIDRequest) (*mcp.CallToolResult, schema.EventList, error) {
	events, err := t.uc.ListTaskEvents(ctx, in.ID)
	return nil, schema.EventList{Events: schema.Map(events, schema.FromEvent)}, err
}

func (t *tools) updateTask(ctx context.Context, _ *mcp.CallToolRequest, in schema.EditTaskRequest) (*mcp.CallToolResult, schema.Task, error) {
	task, err := t.uc.EditTask(ctx, in.Input(in.ID), time.Now())
	return nil, schema.FromTask(task), err
}

func (t *tools) cancelTask(ctx context.Context, _ *mcp.CallToolRequest, in schema.TaskIDRequest) (*mcp.CallToolResult, schema.Task, error) {
	task, err := t.uc.CancelTask(ctx, in.ID, time.Now())
	return nil, schema.FromTask(task), err
}

func (t *tools) retryTask(ctx context.Context, _ *mcp.CallToolRequest, in schema.TaskIDRequest) (*mcp.CallToolResult, schema.Task, error) {
	task, err := t.uc.RetryTask(ctx, in.ID, time.Now())
	return nil, schema.FromTask(task), err
}

func (t *tools) listProjects(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, schema.ProjectList, error) {
	projects, err := t.uc.ListProjects(ctx)
	return nil, schema.ProjectList{Projects: schema.Map(projects, schema.FromProject)}, err
}

func (t *tools) getQueue(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, schema.QueueList, error) {
	tasks, err := t.uc.ListQueue(ctx)
	return nil, schema.QueueList{Tasks: schema.Map(tasks, schema.FromTask)}, err
}

func (t *tools) completeDevelopment(ctx context.Context, _ *mcp.CallToolRequest, in schema.CompleteDevelopmentRequest) (*mcp.CallToolResult, schema.Task, error) {
	task, err := t.uc.CompleteDevelopment(ctx, in.Input(), time.Now())
	return nil, schema.FromTask(task), err
}

func (t *tools) completeReview(ctx context.Context, _ *mcp.CallToolRequest, in schema.CompleteReviewRequest) (*mcp.CallToolResult, schema.Task, error) {
	task, err := t.uc.CompleteReview(ctx, in.Input(), time.Now())
	return nil, schema.FromTask(task), err
}

func (t *tools) reportBlocked(ctx context.Context, _ *mcp.CallToolRequest, in schema.ReportBlockedRequest) (*mcp.CallToolResult, schema.Task, error) {
	task, err := t.uc.ReportBlocked(ctx, in.Reason, time.Now())
	return nil, schema.FromTask(task), err
}
