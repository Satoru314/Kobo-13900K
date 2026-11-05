// Package usecase is everything kobo does. One Usecase serves the GUI, the MCP
// servers and the workers; its methods are split over files by subject.
package usecase

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput   = errors.New("invalid input")
	ErrNothingRunning = errors.New("no task is running")
)

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
	// OffPeak is when off_peak tasks may start.
	OffPeak OffPeakWindow
}

type Usecase struct {
	db       DB
	repo     RepoHost
	ai       AI
	notify   Notifier
	settings Settings
	prompts  prompts
	active   activeRun
}

func New(db DB, repo RepoHost, ai AI, notify Notifier, settings Settings) *Usecase {
	return &Usecase{db: db, repo: repo, ai: ai, notify: notify, settings: settings,
		prompts: prompts{maxRounds: settings.MaxReviewRounds}}
}
