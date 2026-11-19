package repository

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// killGroupOnCancel makes cancelling kill cmd and everything it started.
func killGroupOnCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
}

// killWorkingIn sends SIGKILL to every process but kobo itself whose working
// directory is dir or somewhere below it, found through Linux's /proc/<pid>/cwd.
// Such a process keeps writing into a directory that is being deleted, and a
// background shell that outlived its run is exactly what used to leave a
// worktree half-removed. It returns once no such process is left, or after a
// short wait: whatever still blocks the deletion is reported by the deletion
// itself. Errors along the scan are ignored; processes come and go while it
// runs, and an unreadable /proc entry is not ours to touch.
func killWorkingIn(dir string) {
	// /proc/<pid>/cwd holds the resolved path, without symlinks.
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	self := os.Getpid()
	for start := time.Now(); ; {
		found := false
		pids, err := os.ReadDir("/proc")
		if err != nil {
			return
		}
		for _, e := range pids {
			pid, err := strconv.Atoi(e.Name())
			if err != nil || pid == self {
				continue
			}
			cwd, err := os.Readlink(filepath.Join("/proc", e.Name(), "cwd"))
			if err != nil {
				continue
			}
			if cwd == dir || strings.HasPrefix(cwd, dir+string(filepath.Separator)) {
				found = true
				syscall.Kill(pid, syscall.SIGKILL)
			}
		}
		if !found || time.Since(start) > time.Second {
			return
		}
		// Give the kernel a moment to deliver the signals, then make sure
		// they took effect before the files are deleted.
		time.Sleep(20 * time.Millisecond)
	}
}

// withEnv sets key to value, replacing an inherited value instead of adding a
// duplicate that getenv might ignore.
func withEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return append(out, prefix+value)
}
