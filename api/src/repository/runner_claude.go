package repository

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

// ClaudeRunner runs Claude Code in headless mode with every permission granted,
// connected to kobo's agent MCP endpoint so it can report back.
type ClaudeRunner struct {
	bin      string
	agentURL string
	logDir   string
}

func NewClaudeRunner(bin, agentURL, logDir string) *ClaudeRunner {
	return &ClaudeRunner{bin: bin, agentURL: agentURL, logDir: logDir}
}

func (r *ClaudeRunner) Provider() string {
	return "claude"
}

// NewSessionID returns a random UUID; Claude Code accepts it with --session-id.
func (r *ClaudeRunner) NewSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (r *ClaudeRunner) mcpConfig() (string, error) {
	cfg := map[string]any{"mcpServers": map[string]any{
		"kobo": map[string]any{"type": "http", "url": r.agentURL},
	}}
	b, err := json.Marshal(cfg)
	return string(b), err
}

func (r *ClaudeRunner) Run(ctx context.Context, req irepository.RunRequest) error {
	mcp, err := r.mcpConfig()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(r.logDir, 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(r.logDir, req.LogName+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	fmt.Fprintf(logFile, "\n===== %s %s session=%s resume=%v\n", time.Now().Format(time.RFC3339), req.Role, req.SessionID, req.Resume)

	args := []string{"-p", "--dangerously-skip-permissions", "--mcp-config", mcp, "--output-format", "stream-json", "--verbose"}
	if req.Resume {
		args = append(args, "--resume", req.SessionID)
	} else {
		args = append(args, "--session-id", req.SessionID)
	}
	cmd := exec.CommandContext(ctx, r.bin, append(args, req.Prompt)...)
	cmd.Dir = req.WorkDir
	cmd.Stdout, cmd.Stderr = logFile, logFile
	// Kill the whole process group so commands the AI started die with it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Run(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("claude exited: %w (log: %s)", err, logFile.Name())
	}
	return nil
}
