package usecase

import "time"

type Settings struct {
	// ProjectsDir holds one directory per project, e.g. ~/programF.
	ProjectsDir string
	// WorktreesDir is where task worktrees are created.
	WorktreesDir    string
	MaxReviewRounds int
	DevTimeout      time.Duration
	ReviewTimeout   time.Duration
	// MaxAutoRetries is how many times a failed run is repeated before the task goes to error.
	MaxAutoRetries int
	RetryDelay     time.Duration
	// ShotsRepo is the owner/name of the repository that holds PR screenshots.
	ShotsRepo string
}
