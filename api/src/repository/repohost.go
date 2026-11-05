package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

// RepoHost is the target repositories: their checkouts through the git CLI, and
// their pull requests and CI on GitHub through the gh CLI, using its login. It
// never touches a checkout itself, only worktrees made from it. It implements
// usecase.RepoHost.
type RepoHost struct {
	git command
	gh  command
}

func NewRepoHost() *RepoHost {
	return &RepoHost{git: command{timeout: 100 * time.Second}, gh: command{timeout: 60 * time.Second}}
}

// githubRemote matches git@github.com:owner/name.git and https://github.com/owner/name(.git).
var githubRemote = regexp.MustCompile(`github\.com[:/]([^/]+)/([^/]+?)(?:\.git)?/?$`)

func (r *RepoHost) Remote(ctx context.Context, repoDir string) (usecase.RemoteInfo, error) {
	owner, name, ok, err := r.GitHubOrigin(ctx, repoDir)
	if err != nil {
		return usecase.RemoteInfo{}, err
	}
	if !ok {
		return usecase.RemoteInfo{}, fmt.Errorf("origin of %s is not a GitHub repository", repoDir)
	}
	branch, err := r.defaultBranch(ctx, repoDir)
	if err != nil {
		return usecase.RemoteInfo{}, err
	}
	return usecase.RemoteInfo{Owner: owner, Name: name, DefaultBranch: branch}, nil
}

func (r *RepoHost) GitHubOrigin(ctx context.Context, repoDir string) (owner, name string, ok bool, err error) {
	remotes, err := r.git.output(ctx, repoDir, "git", "remote")
	if err != nil {
		return "", "", false, err
	}
	if !slices.Contains(strings.Fields(remotes), "origin") {
		return "", "", false, nil
	}
	url, err := r.git.output(ctx, repoDir, "git", "remote", "get-url", "origin")
	if err != nil {
		return "", "", false, err
	}
	m := githubRemote.FindStringSubmatch(url)
	if m == nil {
		return "", "", false, nil
	}
	return m[1], m[2], true, nil
}

// defaultBranch asks origin which branch HEAD points to.
func (r *RepoHost) defaultBranch(ctx context.Context, repoDir string) (string, error) {
	out, err := r.git.output(ctx, repoDir, "git", "ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if ref, ok := strings.CutPrefix(line, "ref: refs/heads/"); ok {
			return strings.Fields(ref)[0], nil
		}
	}
	return "", fmt.Errorf("origin has no default branch")
}

func (r *RepoHost) AddWorktree(ctx context.Context, repoDir, path, branch, base string) error {
	if _, err := r.git.output(ctx, repoDir, "git", "fetch", "--quiet", "origin"); err != nil {
		return err
	}
	if err := r.RemoveWorktree(ctx, repoDir, path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	args := []string{"worktree", "add", "--quiet"}
	if branch != "" {
		args = append(args, "-B", branch)
	} else {
		args = append(args, "--detach")
	}
	_, err := r.git.output(ctx, repoDir, "git", append(args, path, "origin/"+base)...)
	return err
}

func (r *RepoHost) RemoveWorktree(ctx context.Context, repoDir, path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	_, err := r.git.output(ctx, repoDir, "git", "worktree", "remove", "--force", path)
	return err
}

func (r *RepoHost) HasWorkflows(ctx context.Context, repoDir, branch string) (bool, error) {
	if _, err := r.git.output(ctx, repoDir, "git", "fetch", "--quiet", "origin", branch); err != nil {
		return false, err
	}
	out, err := r.git.output(ctx, repoDir, "git", "ls-tree", "--name-only", "origin/"+branch, ".github/workflows/")
	if err != nil {
		return false, err
	}
	return out != "", nil
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

func (r *RepoHost) CI(ctx context.Context, fullName string, number int) (usecase.CIResult, error) {
	// gh pr view, not gh pr checks: the gh in Ubuntu's archive (2.45) has no gh pr checks --json.
	out, err := r.gh.output(ctx, "", "gh", "pr", "view", strconv.Itoa(number), "--repo", fullName, "--json", "statusCheckRollup")
	if err != nil {
		return usecase.CIResult{}, err
	}
	var v struct {
		StatusCheckRollup []ghCheck `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return usecase.CIResult{}, fmt.Errorf("read checks: %w", err)
	}
	if len(v.StatusCheckRollup) == 0 {
		return usecase.CIResult{State: usecase.CIPending}, nil
	}
	var failed []string
	for _, c := range v.StatusCheckRollup {
		switch c.bucket() {
		case "pending":
			return usecase.CIResult{State: usecase.CIPending}, nil
		case "fail":
			failed = append(failed, fmt.Sprintf("- %s%s: %s%s", c.Name, c.Context, c.DetailsURL, c.TargetURL))
		}
	}
	if len(failed) == 0 {
		return usecase.CIResult{State: usecase.CISuccess}, nil
	}
	summary := "CI が失敗しました。\n" + strings.Join(failed, "\n") +
		fmt.Sprintf("\n\n`gh pr checks %d` と `gh run view <run-id> --log-failed` でログを確認できます。", number)
	return usecase.CIResult{State: usecase.CIFailure, Summary: summary}, nil
}

func (r *RepoHost) PullRequest(ctx context.Context, fullName string, number int) (usecase.PRInfo, error) {
	out, err := r.gh.output(ctx, "", "gh", "pr", "view", strconv.Itoa(number), "--repo", fullName, "--json", "state,mergedAt")
	if err != nil {
		return usecase.PRInfo{}, err
	}
	var v struct {
		State    string     `json:"state"`
		MergedAt *time.Time `json:"mergedAt"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return usecase.PRInfo{}, err
	}
	switch v.State {
	case "MERGED":
		return usecase.PRInfo{State: domain.PRStateMerged, MergedAt: v.MergedAt}, nil
	case "CLOSED":
		return usecase.PRInfo{State: domain.PRStateClosed}, nil
	default:
		return usecase.PRInfo{State: domain.PRStateOpen}, nil
	}
}
