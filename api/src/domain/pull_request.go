package domain

import "time"

type PRState string

const (
	PRStateOpen   PRState = "open"
	PRStateMerged PRState = "merged"
	PRStateClosed PRState = "closed"
)

func (s PRState) Valid() bool { return s == PRStateOpen || s == PRStateMerged || s == PRStateClosed }

type PullRequest struct {
	ID        int64
	TaskID    int64
	Number    int
	URL       string
	Branch    string
	State     PRState
	CreatedAt time.Time
	MergedAt  *time.Time
}
