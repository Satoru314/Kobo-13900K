package domain

import (
	"slices"
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

var statuses = []Status{StatusDevelop, StatusDeveloping, StatusCI, StatusReview, StatusReviewing, StatusFix,
	StatusFixing, StatusDone, StatusUnresolved, StatusError, StatusCancelled, StatusFinished}

func (s Status) Valid() bool { return slices.Contains(statuses, s) }

type Mode string

const (
	ModePriority  Mode = "priority"
	ModeImmediate Mode = "immediate"
	ModeOffPeak   Mode = "off_peak"
)

func (m Mode) Valid() bool { return m == ModePriority || m == ModeImmediate || m == ModeOffPeak }

type Task struct {
	ID          int64
	ProjectID   int64
	Title       string
	Description string
	Status      Status
	Mode        Mode
	// EnqueuedAt is set while the task waits in the queue: the earliest time it may run.
	EnqueuedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
