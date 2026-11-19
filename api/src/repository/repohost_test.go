package repository

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// runGit runs git in dir, failing the test on any error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

// newGitRepo makes a repository with one commit and a bare origin to fetch
// from, like the checkouts kobo works on.
func newGitRepo(t *testing.T) (repo string) {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	mustMkdir(t, origin)
	runGit(t, origin, "init", "-q", "--bare", "-b", "main")
	repo = filepath.Join(root, "repo")
	mustMkdir(t, repo)
	runGit(t, repo, "init", "-q", "-b", "main")
	runGit(t, repo, "config", "user.email", "kobo@example.com")
	runGit(t, repo, "config", "user.name", "kobo")
	if err := os.WriteFile(filepath.Join(repo, "f"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "f")
	runGit(t, repo, "commit", "-qm", "first")
	runGit(t, repo, "remote", "add", "origin", origin)
	runGit(t, repo, "push", "-q", "origin", "main")
	return repo
}

// waitKilled fails the test unless cmd, already sent a signal, has exited.
func waitKilled(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Errorf("the process working in %s was not killed", cmd.Dir)
	}
}

// TestRemoveWorktreeKillsWhatStillWorksInside covers the leak behind a broken
// removal: a server an AI left running in the worktree kept writing into it,
// which made the removal fail halfway and left the folder behind. A process
// whose working directory is elsewhere, even right next to the worktree, is
// left alone.
func TestRemoveWorktreeKillsWhatStillWorksInside(t *testing.T) {
	repo := newGitRepo(t)
	wt := filepath.Join(filepath.Dir(repo), "task-1-dev")
	runGit(t, repo, "worktree", "add", "-q", wt)
	mustMkdir(t, filepath.Join(wt, "sub"))

	// A background shell's child runs in its own process group, so killing
	// the group it came from cannot reach it; only its working directory
	// gives it away.
	startSleep := func(dir string) *exec.Cmd {
		t.Helper()
		cmd := exec.Command("sleep", "60")
		cmd.Dir = dir
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		return cmd
	}
	inside := startSleep(wt)
	deepInside := startSleep(filepath.Join(wt, "sub"))
	outside := startSleep(filepath.Dir(wt))
	t.Cleanup(func() {
		outside.Process.Kill()
		outside.Wait()
	})

	if err := NewRepoHost().RemoveWorktree(context.Background(), repo, wt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("worktree still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "worktrees", "task-1-dev")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("git's record of the worktree was not pruned: %v", err)
	}
	waitKilled(t, inside)
	waitKilled(t, deepInside)
	if err := syscall.Kill(outside.Process.Pid, 0); err != nil {
		t.Errorf("a process outside the worktree was killed too: %v", err)
	}
}

// TestAddWorktreeRecreatesAfterBrokenRemove covers the state that broken
// removal used to leave behind: git's records of the worktree were gone but
// the folder was not, and every retry failed on the leftover folder.
func TestAddWorktreeRecreatesAfterBrokenRemove(t *testing.T) {
	repo := newGitRepo(t)
	wt := filepath.Join(filepath.Dir(repo), "worktrees", "task-1-dev")
	host, ctx := NewRepoHost(), context.Background()
	if err := host.AddWorktree(ctx, repo, wt, "kobo/task-1", "main"); err != nil {
		t.Fatal(err)
	}

	// Break it the way a failed removal did: delete git's records by hand,
	// leave the folder.
	if err := os.RemoveAll(filepath.Join(repo, ".git", "worktrees", filepath.Base(wt))); err != nil {
		t.Fatal(err)
	}

	// The old RemoveWorktree failed here with "is not a working tree".
	if err := host.AddWorktree(ctx, repo, wt, "kobo/task-1", "main"); err != nil {
		t.Fatalf("recreate worktree: %v", err)
	}
	out, err := exec.Command("git", "-C", wt, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatalf("recreated worktree is not a checkout: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "kobo/task-1" {
		t.Errorf("recreated worktree checked out %q, want kobo/task-1", got)
	}
}

func TestRemoveWorktreeOnMissingPathSucceeds(t *testing.T) {
	// repoDir is never used: a missing path returns before git runs.
	if err := NewRepoHost().RemoveWorktree(context.Background(), "", filepath.Join(t.TempDir(), "nope")); err != nil {
		t.Fatalf("remove a missing worktree: %v", err)
	}
}
