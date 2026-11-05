// Package rest is the HTTP API for the GUI.
package rest

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/schema"
	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

// Usecase is what the REST API needs from the usecase.
type Usecase interface {
	CreateTask(ctx context.Context, in usecase.CreateTaskInput, now time.Time) (domain.Task, error)
	ListTasks(ctx context.Context, in usecase.ListTasksInput) ([]usecase.TaskSummary, error)
	GetTask(ctx context.Context, id int64) (usecase.TaskDetail, error)
	ListTaskEvents(ctx context.Context, id int64) ([]domain.Event, error)
	EditTask(ctx context.Context, in usecase.EditTaskInput, now time.Time) (domain.Task, error)
	CancelTask(ctx context.Context, id int64, now time.Time) (domain.Task, error)
	RetryTask(ctx context.Context, id int64, now time.Time) (domain.Task, error)
	ListProjects(ctx context.Context) ([]domain.Project, error)
	ListProjectCandidates(ctx context.Context) ([]usecase.ProjectCandidate, error)
	AddProject(ctx context.Context, dir string, now time.Time) (domain.Project, error)
	ListQueue(ctx context.Context) ([]domain.Task, error)
	GetQueueState(ctx context.Context) (domain.QueueState, error)
	PauseQueue(ctx context.Context) error
	ResumeQueue(ctx context.Context) error
}

// Changes is where GET /events gets the IDs of changed tasks.
type Changes interface {
	Subscribe(ctx context.Context) <-chan int64
}

type API struct {
	uc      Usecase
	changes Changes
}

// Register adds every REST route to e and maps usecase errors to HTTP statuses.
func Register(e *echo.Echo, uc Usecase, changes Changes) {
	a := &API{uc: uc, changes: changes}
	e.HTTPErrorHandler = errorHandler
	e.GET("/tasks", a.listTasks)
	e.POST("/tasks", a.createTask)
	e.GET("/tasks/:id", a.getTask)
	e.GET("/tasks/:id/events", a.listTaskEvents)
	e.PATCH("/tasks/:id", a.editTask)
	e.POST("/tasks/:id/cancel", a.cancelTask)
	e.POST("/tasks/:id/retry", a.retryTask)
	e.GET("/projects", a.listProjects)
	e.GET("/projects/candidates", a.listProjectCandidates)
	e.POST("/projects", a.addProject)
	e.GET("/queue", a.listQueue)
	e.GET("/queue/state", a.getQueueState)
	e.POST("/queue/pause", a.pauseQueue)
	e.POST("/queue/resume", a.resumeQueue)
	e.GET("/events", a.events)
	e.GET("/health", a.health)
}

func errorHandler(err error, c echo.Context) {
	status := http.StatusInternalServerError
	var he *echo.HTTPError
	switch {
	case errors.As(err, &he):
		status = he.Code
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, usecase.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, usecase.ErrNothingRunning):
		status = http.StatusConflict
	}
	if status == http.StatusInternalServerError {
		slog.Error("request failed", "path", c.Path(), "err", err)
	}
	if !c.Response().Committed {
		_ = c.JSON(status, schema.Error{Error: err.Error()})
	}
}

// id reads the :id path parameter.
func id(c echo.Context) (int64, error) {
	n, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "id must be a number")
	}
	return n, nil
}

// bind decodes the JSON body into v.
func bind(c echo.Context, v any) error {
	if err := (&echo.DefaultBinder{}).BindBody(c, v); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid JSON body: "+err.Error())
	}
	return nil
}
