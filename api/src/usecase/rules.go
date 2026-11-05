package usecase

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

// Rules about tasks shared by the use cases.

var ErrInvalidTransition = errors.New("invalid status transition")

// transitions lists every status a task may move to from a given status.
var transitions = map[domain.Status][]domain.Status{
	domain.StatusDevelop:    {domain.StatusDeveloping, domain.StatusCancelled},
	domain.StatusDeveloping: {domain.StatusCI, domain.StatusReview, domain.StatusDevelop, domain.StatusError},
	domain.StatusCI:         {domain.StatusReview, domain.StatusFix, domain.StatusUnresolved, domain.StatusFinished},
	domain.StatusReview:     {domain.StatusReviewing, domain.StatusFinished},
	domain.StatusReviewing:  {domain.StatusDone, domain.StatusFix, domain.StatusUnresolved, domain.StatusReview, domain.StatusError},
	domain.StatusFix:        {domain.StatusFixing, domain.StatusFinished},
	domain.StatusFixing:     {domain.StatusCI, domain.StatusReview, domain.StatusFix, domain.StatusError},
	domain.StatusDone:       {domain.StatusFinished},
	domain.StatusUnresolved: {domain.StatusFinished},
	domain.StatusError:      {domain.StatusDevelop, domain.StatusReview, domain.StatusFix, domain.StatusFinished},
}

// runs pairs each waiting status with the status the task is in while the AI works on it.
var runs = map[domain.Status]domain.Status{
	domain.StatusDevelop: domain.StatusDeveloping,
	domain.StatusReview:  domain.StatusReviewing,
	domain.StatusFix:     domain.StatusFixing,
}

// runningStatuses are the statuses a task is in while the AI works on it.
var runningStatuses = []domain.Status{domain.StatusDeveloping, domain.StatusReviewing, domain.StatusFixing}

func canMoveTo(from, to domain.Status) bool { return slices.Contains(transitions[from], to) }

func waiting(s domain.Status) bool { _, ok := runs[s]; return ok }

func running(s domain.Status) bool { return slices.Contains(runningStatuses, s) }

// hidden statuses are left out of the default task list.
func hidden(s domain.Status) bool { return s == domain.StatusFinished || s == domain.StatusCancelled }

// waitingStatus is the status a run returns to when it has to be repeated; s
// itself for a status that is not running.
func waitingStatus(s domain.Status) domain.Status {
	for w, r := range runs {
		if r == s {
			return w
		}
	}
	return s
}

// newTask builds a task that waits in the queue from now.
func newTask(projectID int64, title, description string, mode domain.Mode, now time.Time) (domain.Task, error) {
	if title == "" || description == "" {
		return domain.Task{}, errors.New("title and description are required")
	}
	if !mode.Valid() {
		return domain.Task{}, fmt.Errorf("unknown mode %q", mode)
	}
	return domain.Task{ProjectID: projectID, Title: title, Description: description, Status: domain.StatusDevelop,
		Mode: mode, EnqueuedAt: &now, CreatedAt: now, UpdatedAt: now}, nil
}

// moveTo changes the status, and puts the task in the queue when the new status is a waiting one.
func moveTo(t *domain.Task, next domain.Status, now time.Time) error {
	if !canMoveTo(t.Status, next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, t.Status, next)
	}
	t.Status, t.EnqueuedAt, t.UpdatedAt = next, nil, now
	if waiting(next) {
		t.EnqueuedAt = &now
	}
	return nil
}

// start takes the task out of the queue and hands it to the AI.
func start(t *domain.Task, now time.Time) error {
	r, ok := runs[t.Status]
	if !ok {
		return fmt.Errorf("%w: task %d in %s cannot start", ErrInvalidTransition, t.ID, t.Status)
	}
	t.Status, t.EnqueuedAt, t.UpdatedAt = r, nil, now
	return nil
}

// requeue puts a running task back into the queue, to be taken out no earlier than at.
func requeue(t *domain.Task, at, now time.Time) error {
	if !running(t.Status) {
		return fmt.Errorf("%w: task %d in %s is not running", ErrInvalidTransition, t.ID, t.Status)
	}
	t.Status, t.EnqueuedAt, t.UpdatedAt = waitingStatus(t.Status), &at, now
	return nil
}

// edit replaces the content fields that are set, while the task is waiting in develop or in error.
func edit(t *domain.Task, title, description *string, mode *domain.Mode, now time.Time) error {
	if t.Status != domain.StatusDevelop && t.Status != domain.StatusError {
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

// branch is the branch the dev AI works on for the task.
func branch(t domain.Task) string { return "kobo/task-" + strconv.FormatInt(t.ID, 10) }

// fullName is the GitHub "owner/name" of a project.
func fullName(p domain.Project) string { return p.Owner + "/" + p.Name }

// nextStatus is where a task goes after a review round, given the round limit.
func nextStatus(r domain.ReviewRound, maxRounds int) domain.Status {
	if r.Result == domain.ReviewApproved {
		return domain.StatusDone
	}
	if r.RoundNo >= maxRounds {
		return domain.StatusUnresolved
	}
	return domain.StatusFix
}

// OffPeakWindow is a daily time range in local time. End may be earlier than
// Start, in which case the window wraps past midnight.
type OffPeakWindow struct {
	Start time.Duration
	End   time.Duration
}

// ParseOffPeakWindow reads "HH:MM-HH:MM".
func ParseOffPeakWindow(s string) (OffPeakWindow, error) {
	var sh, sm, eh, em int
	if _, err := fmt.Sscanf(s, "%d:%d-%d:%d", &sh, &sm, &eh, &em); err != nil {
		return OffPeakWindow{}, fmt.Errorf("off-peak window %q: want HH:MM-HH:MM", s)
	}
	clock := func(h, m int) time.Duration { return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute }
	return OffPeakWindow{Start: clock(sh, sm), End: clock(eh, em)}, nil
}

// inOffPeak reports whether t falls inside the window.
func inOffPeak(w OffPeakWindow, t time.Time) bool {
	y, m, d := t.Date()
	sinceMidnight := t.Sub(time.Date(y, m, d, 0, 0, 0, 0, t.Location()))
	if w.Start <= w.End {
		return sinceMidnight >= w.Start && sinceMidnight < w.End
	}
	return sinceMidnight >= w.Start || sinceMidnight < w.End
}
