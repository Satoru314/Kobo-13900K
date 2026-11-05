package repository

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

// ClaudeRunner runs Claude Code in headless mode with every permission granted,
// connected to kobo's agent MCP endpoint so it can report back, and to a headless
// browser so it can check screens.
type ClaudeRunner struct {
	bin      string
	agentURL string
	logDir   string
	browser  playwright
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

// mcpConfig names kobo's server kobo-agent, not kobo: Claude Code shares state
// such as "needs auth" between servers of the same name, and the user's own kobo
// server is often configured as kobo.
func (r *ClaudeRunner) mcpConfig(browserOutputDir string) (string, error) {
	servers := map[string]any{
		"kobo-agent": map[string]any{"type": "http", "url": r.agentURL},
	}
	if r.browser.installed {
		servers["playwright"] = r.browser.server(browserOutputDir)
	}
	b, err := json.Marshal(map[string]any{"mcpServers": servers})
	return string(b), err
}

func (r *ClaudeRunner) Run(ctx context.Context, req usecase.RunRequest) error {
	if err := os.MkdirAll(r.logDir, 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(r.logDir, req.LogName+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	fmt.Fprintf(logFile, "\n===== %s %s session=%s resume=%v\n", time.Now().Format(time.RFC3339), req.Role, req.SessionID, req.Resume)

	// Most tasks don't need the browser, so they go on without it.
	if err := r.browser.prepare(ctx, r.logDir, logFile); err != nil {
		fmt.Fprintf(logFile, "kobo: running without the Playwright browser: %v\n", err)
	}
	mcp, err := r.mcpConfig(filepath.Join(r.logDir, "playwright", req.LogName))
	if err != nil {
		return err
	}

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
	killGroupOnCancel(cmd)
	if err := cmd.Run(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("claude exited: %w (log: %s)", err, logFile.Name())
	}
	return nil
}
