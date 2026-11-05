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

// Changes tells subscribers which task changed. Usecases publish after their
// transaction commits; the GUI's SSE connections subscribe.
type Changes struct {
	mu   sync.Mutex
	subs map[chan int64]struct{}
}

func NewChanges() *Changes {
	return &Changes{subs: map[chan int64]struct{}{}}
}

// Subscribe returns a channel of changed task IDs and a function that ends the
// subscription. The channel is closed when the subscription ends.
func (c *Changes) Subscribe() (<-chan int64, func()) {
	ch := make(chan int64, 64)
	c.mu.Lock()
	c.subs[ch] = struct{}{}
	c.mu.Unlock()
	return ch, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.drop(ch)
	}
}

// Publish never blocks. A subscriber that has fallen a whole buffer behind is
// dropped, so its connection ends and the GUI reloads everything on reconnect.
func (c *Changes) Publish(taskID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for ch := range c.subs {
		select {
		case ch <- taskID:
		default:
			c.drop(ch)
		}
	}
}

func (c *Changes) drop(ch chan int64) {
	if _, ok := c.subs[ch]; ok {
		delete(c.subs, ch)
		close(ch)
	}
}
