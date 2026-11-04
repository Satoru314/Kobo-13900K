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

func (h *QueueHandler) List(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.queue.Order(r.Context())
	if err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusOK, tasks)
}
