package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/schema"
	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

// listTasks takes ?status=a,b and ?all=true.
func (a *API) listTasks(c echo.Context) error {
	in := usecase.ListTasksInput{IncludeHidden: c.QueryParam("all") == "true"}
	if s := c.QueryParam("status"); s != "" {
		for _, name := range strings.Split(s, ",") {
			in.Statuses = append(in.Statuses, domain.Status(name))
		}
	}
	tasks, err := a.uc.ListTasks(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, schema.Map(tasks, schema.FromTaskSummary))
}

func (a *API) createTask(c echo.Context) error {
	var req schema.CreateTaskRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	task, err := a.uc.CreateTask(c.Request().Context(), req.Input(), time.Now())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, schema.FromTask(task))
}

func (a *API) getTask(c echo.Context) error {
	id, err := id(c)
	if err != nil {
		return err
	}
	detail, err := a.uc.GetTask(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, schema.FromTaskDetail(detail))
}

func (a *API) listTaskEvents(c echo.Context) error {
	id, err := id(c)
	if err != nil {
		return err
	}
	events, err := a.uc.ListTaskEvents(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, schema.Map(events, schema.FromEvent))
}

func (a *API) editTask(c echo.Context) error {
	id, err := id(c)
	if err != nil {
		return err
	}
	var req schema.EditTaskRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	task, err := a.uc.EditTask(c.Request().Context(), req.Input(id), time.Now())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, schema.FromTask(task))
}

func (a *API) cancelTask(c echo.Context) error {
	id, err := id(c)
	if err != nil {
		return err
	}
	task, err := a.uc.CancelTask(c.Request().Context(), id, time.Now())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, schema.FromTask(task))
}

func (a *API) retryTask(c echo.Context) error {
	id, err := id(c)
	if err != nil {
		return err
	}
	task, err := a.uc.RetryTask(c.Request().Context(), id, time.Now())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, schema.FromTask(task))
}

func (a *API) listProjects(c echo.Context) error {
	projects, err := a.uc.ListProjects(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, schema.Map(projects, schema.FromProject))
}

func (a *API) listProjectCandidates(c echo.Context) error {
	candidates, err := a.uc.ListProjectCandidates(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, schema.Map(candidates, schema.FromProjectCandidate))
}

func (a *API) addProject(c echo.Context) error {
	var req schema.AddProjectRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	project, err := a.uc.AddProject(c.Request().Context(), req.Dir, time.Now())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, schema.FromProject(project))
}

func (a *API) listQueue(c echo.Context) error {
	tasks, err := a.uc.ListQueue(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, schema.Map(tasks, schema.FromTask))
}

func (a *API) getQueueState(c echo.Context) error {
	state, err := a.uc.GetQueueState(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, schema.FromQueueState(state))
}

func (a *API) pauseQueue(c echo.Context) error {
	if err := a.uc.PauseQueue(c.Request().Context()); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, schema.QueueState{Paused: true})
}

func (a *API) resumeQueue(c echo.Context) error {
	if err := a.uc.ResumeQueue(c.Request().Context()); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, schema.QueueState{Paused: false})
}

// events streams a ChangeEvent for every task change until the client leaves,
// kobo stops, or the subscription is dropped.
func (a *API) events(c echo.Context) error {
	ids := a.changes.Subscribe(c.Request().Context())
	w := c.Response()
	w.Header().Set(echo.HeaderContentType, "text/event-stream")
	w.Header().Set(echo.HeaderCacheControl, "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Flush()
	for id := range ids {
		data, err := json.Marshal(schema.ChangeEvent{TaskID: id})
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return nil
		}
		w.Flush()
	}
	return nil
}

func (a *API) health(c echo.Context) error {
	return c.JSON(http.StatusOK, schema.Health{Status: "ok"})
}
