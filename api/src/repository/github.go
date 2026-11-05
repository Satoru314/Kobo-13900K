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

// ghCheck is one entry of statusCheckRollup: a check run (Actions) or a commit status.
type ghCheck struct {
	Typename   string `json:"__typename"` // CheckRun or StatusContext
	Name       string `json:"name"`       // CheckRun
	Status     string `json:"status"`     // CheckRun: QUEUED, IN_PROGRESS, COMPLETED, ...
	Conclusion string `json:"conclusion"` // CheckRun: SUCCESS, FAILURE, NEUTRAL, SKIPPED, CANCELLED, ...
	DetailsURL string `json:"detailsUrl"` // CheckRun
	Context    string `json:"context"`    // StatusContext
	State      string `json:"state"`      // StatusContext: PENDING, EXPECTED, SUCCESS, FAILURE, ERROR
	TargetURL  string `json:"targetUrl"`  // StatusContext
}

// bucket sorts a check into pending, pass or fail.
func (c ghCheck) bucket() string {
	if c.Typename == "StatusContext" {
		switch c.State {
		case "SUCCESS":
			return "pass"
		case "FAILURE", "ERROR":
			return "fail"
		default:
			return "pending"
		}
	}
	if c.Status != "COMPLETED" {
		return "pending"
	}
	switch c.Conclusion {
	case "SUCCESS", "NEUTRAL", "SKIPPED":
		return "pass"
	default:
		return "fail"
	}
}

func (g *GitHub) CI(ctx context.Context, fullName string, number int) (irepository.CIResult, error) {
	// gh pr view, not gh pr checks: the gh in Ubuntu's archive (2.45) has no gh pr checks --json.
	out, err := g.cmd.output(ctx, "", "gh", "pr", "view", strconv.Itoa(number), "--repo", fullName, "--json", "statusCheckRollup")
	if err != nil {
		return irepository.CIResult{}, err
	}
	var v struct {
		StatusCheckRollup []ghCheck `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return irepository.CIResult{}, fmt.Errorf("read checks: %w", err)
	}
	if len(v.StatusCheckRollup) == 0 {
		return irepository.CIResult{State: irepository.CIPending}, nil
	}
	var failed []string
	for _, c := range v.StatusCheckRollup {
		switch c.bucket() {
		case "pending":
			return irepository.CIResult{State: irepository.CIPending}, nil
		case "fail":
			failed = append(failed, fmt.Sprintf("- %s%s: %s%s", c.Name, c.Context, c.DetailsURL, c.TargetURL))
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
