package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/iusecase"
	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type QueueTools struct {
	queue iusecase.QueueUsecase
}

func NewQueueTools(queue iusecase.QueueUsecase) *QueueTools {
	return &QueueTools{queue: queue}
}

type QueueList struct {
	Tasks []domain.Task `json:"tasks"`
}

func (t *QueueTools) Get(ctx context.Context, _ *mcp.CallToolRequest, _ NoInput) (*mcp.CallToolResult, QueueList, error) {
	tasks, err := t.queue.Order(ctx)
	return nil, QueueList{Tasks: tasks}, err
}
