// Package mcp exposes kobo as MCP servers: one for the user, one for the running AI.
package mcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	tasks      *TaskTools
	projects   *ProjectTools
	queue      *QueueTools
	completion *CompletionTools
}

func NewServer(tasks *TaskTools, projects *ProjectTools, queue *QueueTools, completion *CompletionTools) *Server {
	return &Server{tasks: tasks, projects: projects, queue: queue, completion: completion}
}

// UserHandler serves the tools for issuing and following tasks.
func (s *Server) UserHandler() http.Handler {
	srv := mcp.NewServer(&mcp.Implementation{Name: "kobo", Version: "0.1.0"}, nil)
	mcp.AddTool(srv, &mcp.Tool{Name: "create_task", Description: "Issue a development task. The AI develops it, opens a PR and gets it reviewed. project is a directory name under the projects directory; unknown ones are registered from their git remote."}, s.tasks.Create)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_tasks", Description: "List tasks. Finished and cancelled tasks are left out unless include_hidden is set or statuses are given."}, s.tasks.List)
	mcp.AddTool(srv, &mcp.Tool{Name: "get_task", Description: "Show a task with its project, pull request, review rounds and, when in error, the last error."}, s.tasks.Get)
	mcp.AddTool(srv, &mcp.Tool{Name: "get_task_events", Description: "Show the run history of a task, oldest first."}, s.tasks.Events)
	mcp.AddTool(srv, &mcp.Tool{Name: "update_task", Description: "Change title, description or mode of a task that is runnable or in error."}, s.tasks.Update)
	mcp.AddTool(srv, &mcp.Tool{Name: "cancel_task", Description: "Cancel a task that has not started yet."}, s.tasks.Cancel)
	mcp.AddTool(srv, &mcp.Tool{Name: "retry_task", Description: "Put a task in error back into the queue at the stage it failed in."}, s.tasks.Retry)
	mcp.AddTool(srv, &mcp.Tool{Name: "list_projects", Description: "List registered projects (target repositories)."}, s.projects.List)
	mcp.AddTool(srv, &mcp.Tool{Name: "get_queue", Description: "List waiting tasks in the order they will run."}, s.queue.Get)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
}

// AgentHandler serves the tools the running AI uses to report back.
func (s *Server) AgentHandler() http.Handler {
	srv := mcp.NewServer(&mcp.Implementation{Name: "kobo-agent", Version: "0.1.0"}, nil)
	mcp.AddTool(srv, &mcp.Tool{Name: "complete_development", Description: "Report that development or a fix is done and pushed. Call once, with the PR URL, then stop."}, s.completion.CompleteDevelopment)
	mcp.AddTool(srv, &mcp.Tool{Name: "complete_review", Description: "Report the review result: approved, or changes_requested with the issues in comments. Call once, then stop."}, s.completion.CompleteReview)
	mcp.AddTool(srv, &mcp.Tool{Name: "report_blocked", Description: "Report that the task cannot go on, with the reason and what a human has to do. Call once, then stop."}, s.completion.ReportBlocked)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
}
