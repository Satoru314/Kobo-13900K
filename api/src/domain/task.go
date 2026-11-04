package domain

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"
)

type Status string

const (
	StatusRunnable   Status = "runnable"
	StatusDeveloping Status = "developing"
	StatusCI         Status = "ci"
	StatusReviewing  Status = "reviewing"
	StatusFix        Status = "fix"
	StatusDone       Status = "done"
	StatusUnresolved Status = "unresolved"
	StatusError      Status = "error"
	StatusCancelled  Status = "cancelled"
	StatusFinished   Status = "finished"
)

// transitions lists every status a task may move to from a given status.
var transitions = map[Status][]Status{
	StatusRunnable:   {StatusDeveloping, StatusCancelled},
	StatusDeveloping: {StatusCI, StatusReviewing, StatusRunnable, StatusError},
	StatusCI:         {StatusReviewing, StatusFix, StatusUnresolved, StatusFinished},
	StatusReviewing:  {StatusDone, StatusFix, StatusUnresolved, StatusError, StatusFinished},
	StatusFix:        {StatusCI, StatusReviewing, StatusError, StatusFinished},
	StatusDone:       {StatusFinished},
	StatusUnresolved: {StatusFinished},
	StatusError:      {StatusRunnable, StatusReviewing, StatusFix, StatusFinished},
}

var ErrInvalidTransition = errors.New("invalid status transition")

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	_, ok := transitions[s]
	return ok || s == StatusCancelled || s == StatusFinished
}

// Queued reports whether a task in this status waits in the queue for the AI.
func (s Status) Queued() bool {
	return s == StatusRunnable || s == StatusReviewing || s == StatusFix
}

// Hidden reports whether a task in this status is left out of the default list.
func (s Status) Hidden() bool {
	return s == StatusFinished || s == StatusCancelled
}

// QueueStatus is the status a task returns to when its run has to be repeated.
func (s Status) QueueStatus() Status {
	if s == StatusDeveloping {
		return StatusRunnable
	}
	return s
}

// CanMoveTo reports whether the transition s -> next is allowed.
func (s Status) CanMoveTo(next Status) bool {
	return slices.Contains(transitions[s], next)
}

type Mode string

const (
	ModeImmediate Mode = "immediate"
	ModeOffPeak   Mode = "off_peak"
)

func (m Mode) Valid() bool {
	return m == ModeImmediate || m == ModeOffPeak
}

type Task struct {
	ID          int64      `json:"id"`
	ProjectID   int64      `json:"project_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      Status     `json:"status"`
	Mode        Mode       `json:"mode"`
	Running     bool       `json:"running"`
	EnqueuedAt  *time.Time `json:"enqueued_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// NewTask builds a task that waits in the queue from now.
func NewTask(projectID int64, title, description string, mode Mode, now time.Time) (Task, error) {
	if title == "" || description == "" {
		return Task{}, errors.New("title and description are required")
	}
	if !mode.Valid() {
		return Task{}, fmt.Errorf("unknown mode %q", mode)
	}
	return Task{
		ProjectID:   projectID,
		Title:       title,
		Description: description,
		Status:      StatusRunnable,
		Mode:        mode,
		EnqueuedAt:  &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// MoveTo changes the status. The task stops running, and enters the queue when
// the new status is a queued one.
func (t *Task) MoveTo(next Status, now time.Time) error {
	if !t.Status.CanMoveTo(next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, t.Status, next)
	}
	t.Status = next
	t.Running = false
	t.EnqueuedAt = nil
	if next.Queued() {
		t.EnqueuedAt = &now
	}
	t.UpdatedAt = now
	return nil
}

// Start marks the task as taken out of the queue and handed to the AI.
func (t *Task) Start(now time.Time) error {
	if !t.Status.Queued() || t.Running {
		return fmt.Errorf("%w: task %d in %s cannot start", ErrInvalidTransition, t.ID, t.Status)
	}
	if t.Status == StatusRunnable {
		t.Status = StatusDeveloping
	}
	t.Running = true
	t.EnqueuedAt = nil
	t.UpdatedAt = now
	return nil
}

// Requeue puts a task whose run failed back into the queue, to be taken out
// again no earlier than at.
func (t *Task) Requeue(at, now time.Time) error {
	back := t.Status.QueueStatus()
	if back != t.Status && !t.Status.CanMoveTo(back) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, t.Status, back)
	}
	t.Status = back
	t.Running = false
	t.EnqueuedAt = &at
	t.UpdatedAt = now
	return nil
}

// Editable reports whether the user may still change the task's content.
func (t Task) Editable() bool {
	return t.Status == StatusRunnable || t.Status == StatusError
}

// Edit replaces the content fields that are set.
func (t *Task) Edit(title, description *string, mode *Mode, now time.Time) error {
	if !t.Editable() {
		return fmt.Errorf("task %d in %s cannot be edited", t.ID, t.Status)
	}
	if mode != nil && !mode.Valid() {
		return fmt.Errorf("unknown mode %q", *mode)
	}
	if title != nil {
		t.Title = *title
	}
	if description != nil {
		t.Description = *description
	}
	if mode != nil {
		t.Mode = *mode
	}
	t.UpdatedAt = now
	return nil
}

// Branch is the branch the dev AI works on for this task.
func (t Task) Branch() string {
	return "kobo/task-" + strconv.FormatInt(t.ID, 10)
}
