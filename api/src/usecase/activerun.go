package usecase

import (
	"context"
	"sync"
)

// activeRun holds how to stop the AI run in progress. The queue worker sets it;
// an AI report or a pause, handled on other goroutines, stops the run through it.
type activeRun struct {
	mu     sync.Mutex
	taskID int64
	cancel context.CancelFunc
}

func (a *activeRun) set(taskID int64, cancel context.CancelFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.taskID, a.cancel = taskID, cancel
}

func (a *activeRun) clear() { a.set(0, nil) }

// stop kills the AI process of taskID if it is the one running.
func (a *activeRun) stop(taskID int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.taskID == taskID && a.cancel != nil {
		a.cancel()
	}
}

// stopAny kills the AI process that is running, if any.
func (a *activeRun) stopAny() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}
