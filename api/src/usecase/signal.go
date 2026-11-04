package usecase

import (
	"context"
	"sync"
)

// QueueSignal wakes the queue worker when something may be ready to run.
type QueueSignal struct {
	ch chan struct{}
}

func NewQueueSignal() *QueueSignal {
	return &QueueSignal{ch: make(chan struct{}, 1)}
}

// Notify never blocks; several notifications before the worker wakes count as one.
func (s *QueueSignal) Notify() {
	select {
	case s.ch <- struct{}{}:
	default:
	}
}

func (s *QueueSignal) C() <-chan struct{} {
	return s.ch
}

// ActiveRun is the AI run in progress. Completion reports stop it.
type ActiveRun struct {
	mu     sync.Mutex
	taskID int64
	cancel context.CancelFunc
}

func NewActiveRun() *ActiveRun {
	return &ActiveRun{}
}

func (a *ActiveRun) set(taskID int64, cancel context.CancelFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.taskID, a.cancel = taskID, cancel
}

func (a *ActiveRun) clear() {
	a.set(0, nil)
}

// stop kills the AI process of taskID if it is the one running.
func (a *ActiveRun) stop(taskID int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.taskID == taskID && a.cancel != nil {
		a.cancel()
	}
}
