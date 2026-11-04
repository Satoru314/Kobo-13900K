package irepository

import (
	"context"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type CIState string

const (
	CIPending CIState = "pending"
	CISuccess CIState = "success"
	CIFailure CIState = "failure"
)

type CIResult struct {
	State CIState
	// Summary lists the failed checks with links when State is CIFailure.
	Summary string
}

type PRInfo struct {
	State    domain.PRState
	MergedAt *time.Time
}

type GitHub interface {
	// CI reports the combined state of the checks on the PR's latest commit.
	CI(ctx context.Context, fullName string, number int) (CIResult, error)
	PR(ctx context.Context, fullName string, number int) (PRInfo, error)
}
