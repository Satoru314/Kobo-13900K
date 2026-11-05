package rest

import (
	"net/http"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/iusecase"
)

type QueueHandler struct {
	queue iusecase.QueueUsecase
	res   responder
}

func NewQueueHandler(queue iusecase.QueueUsecase) *QueueHandler {
	return &QueueHandler{queue: queue}
}

// State reports {"paused": bool}.
func (h *QueueHandler) State(w http.ResponseWriter, r *http.Request) {
	paused, err := h.queue.Paused(r.Context())
	if err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusOK, map[string]bool{"paused": paused})
}

func (h *QueueHandler) Pause(w http.ResponseWriter, r *http.Request) {
	if err := h.queue.Pause(r.Context()); err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusOK, map[string]bool{"paused": true})
}

func (h *QueueHandler) Resume(w http.ResponseWriter, r *http.Request) {
	if err := h.queue.Resume(r.Context()); err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusOK, map[string]bool{"paused": false})
}

func (h *QueueHandler) List(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.queue.Order(r.Context())
	if err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusOK, tasks)
}
