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
	// ErrAIUnavailable is a configuration problem: the model or the runner it
	// names is gone. Repeating the run does not help, so the task stops at error.
	ErrAIUnavailable = errors.New("AI is not available")
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
	// ShotsRepo is the owner/name of the repository that holds PR screenshots.
	ShotsRepo string
}

type Usecase struct {
	db       DB
	repo     RepoHost
	ais      map[string]AI
	catalog  Catalog
	notify   Notifier
	settings Settings
	prompts  prompts
	active   activeRun
}

func New(db DB, repo RepoHost, ais map[string]AI, catalog Catalog, notify Notifier, settings Settings) *Usecase {
	return &Usecase{db: db, repo: repo, ais: ais, catalog: catalog, notify: notify, settings: settings,
		prompts: prompts{maxRounds: settings.MaxReviewRounds, shotsRepo: settings.ShotsRepo}}
}
