package repository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// playwrightMCP is pinned so that the server always finds the chromium revision
// that was installed for it; the browser build is tied to the package version.
const playwrightMCP = "@playwright/mcp@0.0.84"

const (
	shellLookupTimeout = 15 * time.Second
	browserInstallTime = 10 * time.Minute
)

// playwright gives the AI a browser through the Playwright MCP server. Runs are
// serial, so its state needs no lock.
type playwright struct {
	// npx is its absolute path; path is the PATH it works under. A version
	// manager's shim or an nvm npx is of no use without the rest of that PATH.
	npx, path string
	installed bool
}

// prepare finds npx and installs chromium on the first run after kobo starts.
// playwright install does nothing when the browser is already there. A failure
// is tried again on the next run.
func (p *playwright) prepare(ctx context.Context, dir string, log io.Writer) error {
	if p.installed {
		return nil
	}
	npx, path, err := findNpx(ctx)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, browserInstallTime)
	defer cancel()
	cmd := exec.CommandContext(ctx, npx, "-y", "-p", playwrightMCP, "playwright", "install", "chromium")
	cmd.Dir = dir // not the worktree: its package.json must not choose the playwright
	cmd.Env = withPath(path)
	cmd.Stdout, cmd.Stderr = log, log
	killGroupOnCancel(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("playwright install chromium: %w", err)
	}
	p.npx, p.path, p.installed = npx, path, true
	return nil
}

// server returns the MCP server entry. outputDir keeps its snapshots and logs out
// of the worktree, where the AI could commit them by accident.
func (p *playwright) server(outputDir string) map[string]any {
	return map[string]any{
		"type":    "stdio",
		"command": p.npx,
		"args": []string{"-y", playwrightMCP,
			"--headless",
			"--browser", "chromium", // the default is the system's Google Chrome
			"--isolated", // no profile shared between tasks, and no stale profile lock
			"--output-dir", outputDir,
		},
		"env": map[string]string{"PATH": p.path},
	}
}

// findNpx looks in kobo's own PATH, then in the user's interactive shell: under
// systemd no shell rc has run, so node installed with nvm and the like is not in
// PATH. To choose another one, set PATH in kobo's environment.
func findNpx(ctx context.Context) (npx, path string, err error) {
	path = os.Getenv("PATH")
	if npx, ok := npxIn(ctx, path); ok {
		return npx, path, nil
	}
	path, err = interactiveShellPath(ctx)
	if err != nil {
		return "", "", fmt.Errorf("npx is not in PATH, and reading the shell's PATH failed: %w", err)
	}
	if npx, ok := npxIn(ctx, path); ok {
		return npx, path, nil
	}
	return "", "", errors.New("npx not found, neither in PATH nor in the interactive shell's PATH")
}

// npxIn returns npx's absolute path if it also runs under path.
func npxIn(ctx context.Context, path string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, shellLookupTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", "command -v npx && npx --version >/dev/null")
	cmd.Env = withPath(path)
	killGroupOnCancel(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	npx, _, _ := strings.Cut(string(out), "\n")
	return npx, filepath.IsAbs(npx)
}

func interactiveShellPath(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, shellLookupTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-ic", `printf '\n%s' "$PATH"`)
	killGroupOnCancel(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// Anything the rc files print comes before the last line.
	return string(out[strings.LastIndexByte(string(out), '\n')+1:]), nil
}

func withPath(path string) []string {
	return append(os.Environ(), "PATH="+path)
}

// killGroupOnCancel makes cancelling kill cmd and everything it started.
func killGroupOnCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
}
