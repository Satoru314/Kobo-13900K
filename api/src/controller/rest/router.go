// Package rest is the HTTP API for the GUI.
package rest

import "net/http"

type Router struct {
	tasks    *TaskHandler
	projects *ProjectHandler
	queue    *QueueHandler
	health   *HealthHandler
}

func NewRouter(tasks *TaskHandler, projects *ProjectHandler, queue *QueueHandler, health *HealthHandler) *Router {
	return &Router{tasks: tasks, projects: projects, queue: queue, health: health}
}

// Register adds every REST route to mux.
func (rt *Router) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /projects", rt.projects.List)
	mux.HandleFunc("POST /tasks", rt.tasks.Create)
	mux.HandleFunc("GET /tasks", rt.tasks.List)
	mux.HandleFunc("GET /tasks/{id}", rt.tasks.Get)
	mux.HandleFunc("GET /tasks/{id}/events", rt.tasks.Events)
	mux.HandleFunc("PATCH /tasks/{id}", rt.tasks.Update)
	mux.HandleFunc("POST /tasks/{id}/cancel", rt.tasks.Cancel)
	mux.HandleFunc("POST /tasks/{id}/retry", rt.tasks.Retry)
	mux.HandleFunc("GET /queue", rt.queue.List)
	mux.HandleFunc("GET /health", rt.health.Get)
}
