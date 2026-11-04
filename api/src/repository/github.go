package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

// GitHub asks GitHub through the gh CLI, using its login.
type GitHub struct {
	cmd command
}

func NewGitHub() *GitHub {
	return &GitHub{cmd: command{timeout: 60 * time.Second}}
}

type ghCheck struct {
	Name   string `json:"name"`
	Bucket string `json:"bucket"` // pass, fail, pending, skipping, cancel
	Link   string `json:"link"`
}

func (g *GitHub) CI(ctx context.Context, fullName string, number int) (irepository.CIResult, error) {
	// gh pr checks exits non-zero while checks are pending or failing, so the output decides.
	out, runErr := g.cmd.output(ctx, "", "gh", "pr", "checks", strconv.Itoa(number), "--repo", fullName,
		"--json", "name,bucket,link")
	var checks []ghCheck
	if err := json.Unmarshal([]byte(out), &checks); err != nil {
		if strings.Contains(fmt.Sprint(runErr), "no checks reported") {
			return irepository.CIResult{State: irepository.CIPending}, nil
		}
		return irepository.CIResult{}, fmt.Errorf("read checks: %v", runErr)
	}
	if len(checks) == 0 {
		return irepository.CIResult{State: irepository.CIPending}, nil
	}
	var failed []string
	for _, c := range checks {
		switch c.Bucket {
		case "pending":
			return irepository.CIResult{State: irepository.CIPending}, nil
		case "fail", "cancel":
			failed = append(failed, fmt.Sprintf("- %s: %s", c.Name, c.Link))
		}
	}
	if len(failed) == 0 {
		return irepository.CIResult{State: irepository.CISuccess}, nil
	}
	summary := "CI が失敗しました。\n" + strings.Join(failed, "\n") +
		fmt.Sprintf("\n\n`gh pr checks %d` と `gh run view <run-id> --log-failed` でログを確認できます。", number)
	return irepository.CIResult{State: irepository.CIFailure, Summary: summary}, nil
}

func (g *GitHub) PR(ctx context.Context, fullName string, number int) (irepository.PRInfo, error) {
	out, err := g.cmd.output(ctx, "", "gh", "pr", "view", strconv.Itoa(number), "--repo", fullName, "--json", "state,mergedAt")
	if err != nil {
		return irepository.PRInfo{}, err
	}
	var v struct {
		State    string     `json:"state"`
		MergedAt *time.Time `json:"mergedAt"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return irepository.PRInfo{}, err
	}
	switch v.State {
	case "MERGED":
		return irepository.PRInfo{State: domain.PRStateMerged, MergedAt: v.MergedAt}, nil
	case "CLOSED":
		return irepository.PRInfo{State: domain.PRStateClosed}, nil
	default:
		return irepository.PRInfo{State: domain.PRStateOpen}, nil
	}
}
