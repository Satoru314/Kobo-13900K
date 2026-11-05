package rest

import (
	"net/http"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/iusecase"
)

type ProjectHandler struct {
	projects iusecase.ProjectUsecase
	res      responder
}

func NewProjectHandler(projects iusecase.ProjectUsecase) *ProjectHandler {
	return &ProjectHandler{projects: projects}
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, err := h.projects.List(r.Context())
	if err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusOK, projects)
}

func (h *ProjectHandler) Candidates(w http.ResponseWriter, r *http.Request) {
	candidates, err := h.projects.Candidates(r.Context())
	if err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusOK, candidates)
}

// Add takes {"dir": "emela"}.
func (h *ProjectHandler) Add(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Dir string `json:"dir"`
	}
	if !h.res.decode(w, r, &in) {
		return
	}
	project, err := h.projects.Ensure(r.Context(), in.Dir, time.Now())
	if err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusCreated, project)
}
