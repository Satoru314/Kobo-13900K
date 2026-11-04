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
	ID        int64        `json:"id"`
	TaskID    int64        `json:"task_id"`
	RoundNo   int          `json:"round_no"`
	Result    ReviewResult `json:"result"`
	Comments  string       `json:"comments"`
	CreatedAt time.Time    `json:"created_at"`
}

// NextStatus is where a task goes after this round, given the round limit.
func (r ReviewRound) NextStatus(maxRounds int) Status {
	if r.Result == ReviewApproved {
		return StatusDone
	}
	if r.RoundNo >= maxRounds {
		return StatusUnresolved
	}
	return StatusFix
}
