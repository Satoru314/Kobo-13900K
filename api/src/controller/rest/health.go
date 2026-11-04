package rest

import "net/http"

type HealthHandler struct {
	res responder
}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Get(w http.ResponseWriter, _ *http.Request) {
	h.res.json(w, http.StatusOK, map[string]string{"status": "ok"})
}
