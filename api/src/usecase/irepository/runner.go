package irepository

import (
	"context"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type RunRequest struct {
	Role      domain.Role
	SessionID string
	// Resume continues SessionID instead of starting a new conversation with it.
	Resume  bool
	WorkDir string
	Prompt  string
	// LogName names the file the AI's output is written to.
	LogName string
}

// Runner starts an AI on a prompt. Each AI provider has its own implementation.
type Runner interface {
	Provider() string
	NewSessionID() string
	// Run blocks until the AI process exits. Cancelling ctx kills the process.
	Run(ctx context.Context, req RunRequest) error
}
