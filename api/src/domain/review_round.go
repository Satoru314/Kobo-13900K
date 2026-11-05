package domain

import "time"

type ReviewResult string

const (
	ReviewApproved         ReviewResult = "approved"
	ReviewChangesRequested ReviewResult = "changes_requested"
	ReviewCIFailed         ReviewResult = "ci_failed"
)

func (r ReviewResult) Valid() bool {
	return r == ReviewApproved || r == ReviewChangesRequested || r == ReviewCIFailed
}

// ReviewRound is one pass of review or CI. Both count toward the round limit.
type ReviewRound struct {
	ID        int64
	TaskID    int64
	RoundNo   int
	Result    ReviewResult
	Comments  string
	CreatedAt time.Time
}
