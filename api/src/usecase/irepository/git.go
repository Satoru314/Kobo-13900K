package irepository

import "context"

type RemoteInfo struct {
	Owner         string
	Name          string
	DefaultBranch string
}

type Git interface {
	// Remote reads the GitHub owner, name and default branch from the repository's origin.
	Remote(ctx context.Context, repoDir string) (RemoteInfo, error)
	// GitHubOrigin reads owner and name from origin without going to the network.
	// ok is false when there is no origin or it is not on GitHub.
	GitHubOrigin(ctx context.Context, repoDir string) (owner, name string, ok bool, err error)
	// AddWorktree fetches origin and creates a worktree at path from origin/base.
	// With branch set, the worktree checks out branch reset to origin/base; otherwise it is detached.
	AddWorktree(ctx context.Context, repoDir, path, branch, base string) error
	// RemoveWorktree deletes the worktree at path, even if it has changes.
	RemoveWorktree(ctx context.Context, repoDir, path string) error
	// HasWorkflows reports whether origin/branch contains GitHub Actions workflows.
	HasWorkflows(ctx context.Context, repoDir, branch string) (bool, error)
}
