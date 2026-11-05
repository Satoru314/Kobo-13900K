// Package notify passes on what changed inside kobo: task changes to the GUI's
// open /events streams, and "a task may be ready" to the queue worker.
package notify

import (
	"context"
	"sync"
)

type Hub struct {
	mu   sync.Mutex
	subs map[chan int64]struct{}
	wake chan struct{}
}

func New() *Hub {
	return &Hub{subs: map[chan int64]struct{}{}, wake: make(chan struct{}, 1)}
}

// TaskChanged sends taskID to every subscriber. It never blocks: a subscriber
// that has fallen a whole buffer behind is dropped, so its stream ends and the
// GUI reloads everything when it reconnects.
func (h *Hub) TaskChanged(taskID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- taskID:
		default:
			h.drop(ch)
		}
	}
}

// Subscribe returns the IDs of tasks that change from now on. The channel is
// closed when ctx ends or the subscriber is dropped.
func (h *Hub) Subscribe(ctx context.Context) <-chan int64 {
	ch := make(chan int64, 64)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	go func() {
		<-ctx.Done()
		h.mu.Lock()
		defer h.mu.Unlock()
		h.drop(ch)
	}()
	return ch
}

func (h *Hub) drop(ch chan int64) {
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
}

// QueueMayRun wakes the queue worker. It never blocks; several calls before the
// worker wakes count as one.
func (h *Hub) QueueMayRun() {
	select {
	case h.wake <- struct{}{}:
	default:
	}
}

// Wake is what the queue worker waits on.
func (h *Hub) Wake() <-chan struct{} {
	return h.wake
}
