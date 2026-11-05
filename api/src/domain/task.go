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
	StatusDevelop    Status = "develop"
	StatusDeveloping Status = "developing"
	StatusCI         Status = "ci"
	StatusReview     Status = "review"
	StatusReviewing  Status = "reviewing"
	StatusFix        Status = "fix"
	StatusFixing     Status = "fixing"
	StatusDone       Status = "done"
	StatusUnresolved Status = "unresolved"
	StatusError      Status = "error"
	StatusCancelled  Status = "cancelled"
	StatusFinished   Status = "finished"
)

// transitions lists every status a task may move to from a given status.
var transitions = map[Status][]Status{
	StatusDevelop:    {StatusDeveloping, StatusCancelled},
	StatusDeveloping: {StatusCI, StatusReview, StatusDevelop, StatusError},
	StatusCI:         {StatusReview, StatusFix, StatusUnresolved, StatusFinished},
	StatusReview:     {StatusReviewing, StatusFinished},
	StatusReviewing:  {StatusDone, StatusFix, StatusUnresolved, StatusReview, StatusError},
	StatusFix:        {StatusFixing, StatusFinished},
	StatusFixing:     {StatusCI, StatusReview, StatusFix, StatusError},
	StatusDone:       {StatusFinished},
	StatusUnresolved: {StatusFinished},
	StatusError:      {StatusDevelop, StatusReview, StatusFix, StatusFinished},
}

// runs pairs each waiting status with the status the task is in while the AI works on it.
var runs = map[Status]Status{
	StatusDevelop: StatusDeveloping,
	StatusReview:  StatusReviewing,
	StatusFix:     StatusFixing,
}

var ErrInvalidTransition = errors.New("invalid status transition")

// WaitingStatuses are the statuses a task waits in the queue for the AI.
var WaitingStatuses = []Status{StatusDevelop, StatusReview, StatusFix}

// RunningStatuses are the statuses a task is in while the AI works on it.
var RunningStatuses = []Status{StatusDeveloping, StatusReviewing, StatusFixing}

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	_, ok := transitions[s]
	return ok || s == StatusCancelled || s == StatusFinished
}

// Waiting reports whether a task in this status waits in the queue for the AI.
func (s Status) Waiting() bool {
	return slices.Contains(WaitingStatuses, s)
}

// Running reports whether the AI is working on a task in this status.
func (s Status) Running() bool {
	return slices.Contains(RunningStatuses, s)
}

// Hidden reports whether a task in this status is left out of the default list.
func (s Status) Hidden() bool {
	return s == StatusFinished || s == StatusCancelled
}

// WaitingStatus is the status a task returns to when its run has to be repeated.
// It is s itself for statuses that are not running.
func (s Status) WaitingStatus() Status {
	for waiting, running := range runs {
		if running == s {
			return waiting
		}
	}
	return s
}

// CanMoveTo reports whether the transition s -> next is allowed.
func (s Status) CanMoveTo(next Status) bool {
	return slices.Contains(transitions[s], next)
}

type Mode string

const (
	ModePriority  Mode = "priority"
	ModeImmediate Mode = "immediate"
	ModeOffPeak   Mode = "off_peak"
)

func (m Mode) Valid() bool {
	return m == ModePriority || m == ModeImmediate || m == ModeOffPeak
}

type Task struct {
	ID          int64      `json:"id"`
	ProjectID   int64      `json:"project_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      Status     `json:"status"`
	Mode        Mode       `json:"mode"`
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
		Status:      StatusDevelop,
		Mode:        mode,
		EnqueuedAt:  &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// MoveTo changes the status, and puts the task in the queue when the new
// status is a waiting one.
func (t *Task) MoveTo(next Status, now time.Time) error {
	if !t.Status.CanMoveTo(next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, t.Status, next)
	}
	t.Status = next
	t.EnqueuedAt = nil
	if next.Waiting() {
		t.EnqueuedAt = &now
	}
	t.UpdatedAt = now
	return nil
}

// Start takes the task out of the queue and hands it to the AI.
func (t *Task) Start(now time.Time) error {
	running, ok := runs[t.Status]
	if !ok {
		return fmt.Errorf("%w: task %d in %s cannot start", ErrInvalidTransition, t.ID, t.Status)
	}
	t.Status = running
	t.EnqueuedAt = nil
	t.UpdatedAt = now
	return nil
}

// Requeue puts a task whose run failed back into the queue, to be taken out
// again no earlier than at.
func (t *Task) Requeue(at, now time.Time) error {
	if !t.Status.Running() {
		return fmt.Errorf("%w: task %d in %s is not running", ErrInvalidTransition, t.ID, t.Status)
	}
	t.Status = t.Status.WaitingStatus()
	t.EnqueuedAt = &at
	t.UpdatedAt = now
	return nil
}

// Editable reports whether the user may still change the task's content.
func (t Task) Editable() bool {
	return t.Status == StatusDevelop || t.Status == StatusError
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
