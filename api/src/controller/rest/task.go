package rest

import (
	"net/http"
	"strings"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/iusecase"
	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

type TaskHandler struct {
	tasks iusecase.TaskUsecase
	res   responder
}

func NewTaskHandler(tasks iusecase.TaskUsecase) *TaskHandler {
	return &TaskHandler{tasks: tasks}
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in usecase.CreateTaskInput
	if !h.res.decode(w, r, &in) {
		return
	}
	task, err := h.tasks.Create(r.Context(), in, time.Now())
	if err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusCreated, task)
}

// List takes ?status=a,b and ?all=true.
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	in := usecase.ListTasksInput{IncludeHidden: r.URL.Query().Get("all") == "true"}
	if s := r.URL.Query().Get("status"); s != "" {
		for _, name := range strings.Split(s, ",") {
			in.Statuses = append(in.Statuses, domain.Status(name))
		}
	}
	tasks, err := h.tasks.List(r.Context(), in)
	if err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusOK, tasks)
}

func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.res.id(w, r)
	if !ok {
		return
	}
	detail, err := h.tasks.Get(r.Context(), id)
	if err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusOK, detail)
}

func (h *TaskHandler) Events(w http.ResponseWriter, r *http.Request) {
	id, ok := h.res.id(w, r)
	if !ok {
		return
	}
	events, err := h.tasks.Events(r.Context(), id)
	if err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusOK, events)
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.res.id(w, r)
	if !ok {
		return
	}
	var in usecase.UpdateTaskInput
	if !h.res.decode(w, r, &in) {
		return
	}
	in.ID = id
	task, err := h.tasks.Update(r.Context(), in, time.Now())
	if err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusOK, task)
}

func (h *TaskHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := h.res.id(w, r)
	if !ok {
		return
	}
	task, err := h.tasks.Cancel(r.Context(), id, time.Now())
	if err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusOK, task)
}

func (h *TaskHandler) Retry(w http.ResponseWriter, r *http.Request) {
	id, ok := h.res.id(w, r)
	if !ok {
		return
	}
	task, err := h.tasks.Retry(r.Context(), id, time.Now())
	if err != nil {
		h.res.error(w, err)
		return
	}
	h.res.json(w, http.StatusOK, task)
}
