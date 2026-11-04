package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/iusecase"
)

type QueueTools struct {
	queue iusecase.QueueUsecase
}

func NewQueueTools(queue iusecase.QueueUsecase) *QueueTools {
	return &QueueTools{queue: queue}
}

func (t *QueueTools) Get(ctx context.Context, _ *mcp.CallToolRequest, _ NoInput) (*mcp.CallToolResult, TaskList, error) {
	tasks, err := t.queue.Order(ctx)
	return nil, TaskList{Tasks: tasks}, err
}
