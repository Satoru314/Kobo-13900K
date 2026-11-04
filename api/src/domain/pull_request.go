package domain

import "time"

type PRState string

const (
	PRStateOpen   PRState = "open"
	PRStateMerged PRState = "merged"
	PRStateClosed PRState = "closed"
)

type PullRequest struct {
	ID        int64      `json:"id"`
	TaskID    int64      `json:"task_id"`
	Number    int        `json:"number"`
	URL       string     `json:"url"`
	Branch    string     `json:"branch"`
	State     PRState    `json:"state"`
	CreatedAt time.Time  `json:"created_at"`
	MergedAt  *time.Time `json:"merged_at"`
}
