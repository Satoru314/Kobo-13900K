package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	url, err := g.cmd.output(ctx, repoDir, "git", "remote", "get-url", "origin")
	if err != nil {
		return irepository.RemoteInfo{}, err
	}
	m := githubRemote.FindStringSubmatch(url)
	if m == nil {
		return irepository.RemoteInfo{}, fmt.Errorf("origin %q is not a GitHub repository", url)
	}
	branch, err := g.defaultBranch(ctx, repoDir, m[1], m[2])
	if err != nil {
		return irepository.RemoteInfo{}, err
	}
	return irepository.RemoteInfo{Owner: m[1], Name: m[2], DefaultBranch: branch}, nil
}

// defaultBranch reads origin/HEAD from the local clone, and asks GitHub through gh
// when the clone does not know it.
func (g *Git) defaultBranch(ctx context.Context, repoDir, owner, name string) (string, error) {
	if ref, err := g.cmd.output(ctx, repoDir, "git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if branch, ok := strings.CutPrefix(ref, "origin/"); ok && branch != "" {
			return branch, nil
		}
	}
	branch, err := g.cmd.output(ctx, repoDir, "gh", "api", "repos/"+owner+"/"+name, "--jq", ".default_branch")
	if err != nil {
		return "", fmt.Errorf("find default branch: %w", err)
	}
	if branch == "" {
		return "", fmt.Errorf("%s/%s has no default branch", owner, name)
	}
	return branch, nil
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
	if _, err := os.Stat(path); err == nil {
		// A broken worktree is still deleted from disk below.
		_, _ = g.cmd.output(ctx, repoDir, "git", "worktree", "remove", "--force", path)
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	_, err := g.cmd.output(ctx, repoDir, "git", "worktree", "prune")
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
