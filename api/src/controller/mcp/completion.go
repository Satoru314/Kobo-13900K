package mcp

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/iusecase"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

// CompletionTools are for the running AI only.
type CompletionTools struct {
	completion iusecase.CompletionUsecase
}

func NewCompletionTools(completion iusecase.CompletionUsecase) *CompletionTools {
	return &CompletionTools{completion: completion}
}

type Ack struct {
	TaskID int64  `json:"task_id"`
	Status string `json:"status"`
}

func (t *CompletionTools) CompleteDevelopment(ctx context.Context, _ *mcp.CallToolRequest, in usecase.CompleteDevelopmentInput) (*mcp.CallToolResult, Ack, error) {
	task, err := t.completion.CompleteDevelopment(ctx, in, time.Now())
	return nil, Ack{TaskID: task.ID, Status: string(task.Status)}, err
}

func (t *CompletionTools) CompleteReview(ctx context.Context, _ *mcp.CallToolRequest, in usecase.CompleteReviewInput) (*mcp.CallToolResult, Ack, error) {
	task, err := t.completion.CompleteReview(ctx, in, time.Now())
	return nil, Ack{TaskID: task.ID, Status: string(task.Status)}, err
}

func (t *CompletionTools) ReportBlocked(ctx context.Context, _ *mcp.CallToolRequest, in usecase.ReportBlockedInput) (*mcp.CallToolResult, Ack, error) {
	task, err := t.completion.ReportBlocked(ctx, in, time.Now())
	return nil, Ack{TaskID: task.ID, Status: string(task.Status)}, err
}
