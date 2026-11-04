package rest

import (
	"net/http"

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
