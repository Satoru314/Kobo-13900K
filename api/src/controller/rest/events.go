package rest

import (
	"fmt"
	"net/http"

	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

type EventsHandler struct {
	changes *usecase.Changes
}

func NewEventsHandler(changes *usecase.Changes) *EventsHandler {
	return &EventsHandler{changes: changes}
}

// Stream sends {"task_id": N} for every task change until the client leaves
// or the subscription is dropped.
func (h *EventsHandler) Stream(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	ids, unsubscribe := h.changes.Subscribe()
	defer unsubscribe()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case id, ok := <-ids:
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(w, "data: {\"task_id\": %d}\n\n", id); err != nil {
				return
			}
			if err := rc.Flush(); err != nil {
				return
			}
		}
	}
}
