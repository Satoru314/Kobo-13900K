package mcp

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/iusecase"
	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

// TaskTools are the user's tools for issuing and following tasks.
type TaskTools struct {
	tasks iusecase.TaskUsecase
}

func NewTaskTools(tasks iusecase.TaskUsecase) *TaskTools {
	return &TaskTools{tasks: tasks}
}

type TaskID struct {
	ID int64 `json:"id" jsonschema:"task id"`
}

type TaskList struct {
	Tasks []domain.Task `json:"tasks"`
}

type EventList struct {
	Events []domain.Event `json:"events"`
}

func (t *TaskTools) Create(ctx context.Context, _ *mcp.CallToolRequest, in usecase.CreateTaskInput) (*mcp.CallToolResult, domain.Task, error) {
	task, err := t.tasks.Create(ctx, in, time.Now())
	return nil, task, err
}

func (t *TaskTools) List(ctx context.Context, _ *mcp.CallToolRequest, in usecase.ListTasksInput) (*mcp.CallToolResult, TaskList, error) {
	tasks, err := t.tasks.List(ctx, in)
	return nil, TaskList{Tasks: tasks}, err
}

func (t *TaskTools) Get(ctx context.Context, _ *mcp.CallToolRequest, in TaskID) (*mcp.CallToolResult, usecase.TaskDetail, error) {
	detail, err := t.tasks.Get(ctx, in.ID)
	return nil, detail, err
}

func (t *TaskTools) Events(ctx context.Context, _ *mcp.CallToolRequest, in TaskID) (*mcp.CallToolResult, EventList, error) {
	events, err := t.tasks.Events(ctx, in.ID)
	return nil, EventList{Events: events}, err
}

func (t *TaskTools) Update(ctx context.Context, _ *mcp.CallToolRequest, in usecase.UpdateTaskInput) (*mcp.CallToolResult, domain.Task, error) {
	task, err := t.tasks.Update(ctx, in, time.Now())
	return nil, task, err
}

func (t *TaskTools) Cancel(ctx context.Context, _ *mcp.CallToolRequest, in TaskID) (*mcp.CallToolResult, domain.Task, error) {
	task, err := t.tasks.Cancel(ctx, in.ID, time.Now())
	return nil, task, err
}

func (t *TaskTools) Retry(ctx context.Context, _ *mcp.CallToolRequest, in TaskID) (*mcp.CallToolResult, domain.Task, error) {
	task, err := t.tasks.Retry(ctx, in.ID, time.Now())
	return nil, task, err
}
