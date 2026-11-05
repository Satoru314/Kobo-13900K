package domain

import "time"

// AISetting is the single row of AI choices: the model for development and,
// when SeparateReview is set, a different model for review.
type AISetting struct {
	DevAI          string
	ReviewAI       string
	SeparateReview bool
	UpdatedAt      time.Time
}
