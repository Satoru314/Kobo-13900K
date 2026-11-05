package irepository

import "context"

// QueueStateRepository keeps whether the user paused the AI.
type QueueStateRepository interface {
	Paused(ctx context.Context) (bool, error)
	SetPaused(ctx context.Context, paused bool) error
}
