package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

// Git runs the git CLI. It never touches the checkout in repoDir itself, only worktrees.
type Git struct {
	cmd command
}

func NewGit() *Git {
	return &Git{cmd: command{timeout: 100 * time.Second}}
}

// githubRemote matches git@github.com:owner/name.git and https://github.com/owner/name(.git).
var githubRemote = regexp.MustCompile(`github\.com[:/]([^/]+)/([^/]+?)(?:\.git)?/?$`)

func (g *Git) Remote(ctx context.Context, repoDir string) (irepository.RemoteInfo, error) {
	owner, name, ok, err := g.GitHubOrigin(ctx, repoDir)
	if err != nil {
		return irepository.RemoteInfo{}, err
	}
	if !ok {
		return irepository.RemoteInfo{}, fmt.Errorf("origin of %s is not a GitHub repository", repoDir)
	}
	branch, err := g.defaultBranch(ctx, repoDir)
	if err != nil {
		return irepository.RemoteInfo{}, err
	}
	return irepository.RemoteInfo{Owner: owner, Name: name, DefaultBranch: branch}, nil
}

func (g *Git) GitHubOrigin(ctx context.Context, repoDir string) (owner, name string, ok bool, err error) {
	remotes, err := g.cmd.output(ctx, repoDir, "git", "remote")
	if err != nil {
		return "", "", false, err
	}
	if !slices.Contains(strings.Fields(remotes), "origin") {
		return "", "", false, nil
	}
	url, err := g.cmd.output(ctx, repoDir, "git", "remote", "get-url", "origin")
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
func (g *Git) defaultBranch(ctx context.Context, repoDir string) (string, error) {
	out, err := g.cmd.output(ctx, repoDir, "git", "ls-remote", "--symref", "origin", "HEAD")
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

func (g *Git) AddWorktree(ctx context.Context, repoDir, path, branch, base string) error {
	if _, err := g.cmd.output(ctx, repoDir, "git", "fetch", "--quiet", "origin"); err != nil {
		return err
	}
	if err := g.RemoveWorktree(ctx, repoDir, path); err != nil {
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
	_, err := g.cmd.output(ctx, repoDir, "git", append(args, path, "origin/"+base)...)
	return err
}

func (g *Git) RemoveWorktree(ctx context.Context, repoDir, path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	_, err := g.cmd.output(ctx, repoDir, "git", "worktree", "remove", "--force", path)
	return err
}

func (g *Git) HasWorkflows(ctx context.Context, repoDir, branch string) (bool, error) {
	if _, err := g.cmd.output(ctx, repoDir, "git", "fetch", "--quiet", "origin", branch); err != nil {
		return false, err
	}
	out, err := g.cmd.output(ctx, repoDir, "git", "ls-tree", "--name-only", "origin/"+branch, ".github/workflows/")
	if err != nil {
		return false, err
	}
	return out != "", nil
}
