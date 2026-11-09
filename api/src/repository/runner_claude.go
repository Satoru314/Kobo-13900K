package repository

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

// claudeDisallowedTools lists the Claude Code tools kobo's AI never needs. They
// would let it notify the user, message the user's other sessions, or schedule
// work to run later, which is outside a task's worktree and lifetime. This is a
// deny list rather than an allow list so a new tool Claude needs is not lost.
var claudeDisallowedTools = []string{
	"PushNotification",
	"SendMessage",
	"ListAgents",
	"RemoteTrigger",
	"CronCreate",
	"CronDelete",
	"CronList",
	"ScheduleWakeup",
}

// ClaudeRunner runs Claude Code in headless mode with every permission granted,
// connected to kobo's agent MCP endpoint so it can report back, and to a headless
// browser so it can check screens.
type ClaudeRunner struct {
	bin      string
	agentURL string
	logDir   string
	browser  *playwright
}

func NewClaudeRunner(bin, agentURL, logDir string, browser *playwright) *ClaudeRunner {
	return &ClaudeRunner{bin: bin, agentURL: agentURL, logDir: logDir, browser: browser}
}

func (r *ClaudeRunner) Provider() string { return "claude" }

// NewSessionID returns a random UUID; Claude Code accepts it with --session-id.
func (r *ClaudeRunner) NewSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// servers lists the MCP servers this run gets. kobo-agent is named that, not
// kobo: Claude Code shares state such as "needs auth" between servers of the same
// name, and the user's own kobo server is often configured as kobo.
func (r *ClaudeRunner) servers(logName string) []mcpServer {
	servers := []mcpServer{agentServer(r.agentURL)}
	if r.browser != nil {
		servers = append(servers, r.browser.server(r.browser.outputDir(r.logDir, logName)))
	}
	return servers
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
	fmt.Fprintf(logFile, "\n===== %s %s ai=%s model=%s session=%s resume=%v\n",
		time.Now().Format(time.RFC3339), req.Role, req.AI, req.Model, req.SessionID, req.Resume)

	mcp, err := claudeMCPConfig(r.servers(req.LogName))
	if err != nil {
		return err
	}

	args := []string{
		"-p", "--dangerously-skip-permissions", "--mcp-config", mcp,
		// Comma-separated in one argument so the variadic option does not swallow
		// the prompt that follows the other options.
		"--disallowedTools", strings.Join(claudeDisallowedTools, ","),
		"--output-format", "stream-json", "--verbose",
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
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
