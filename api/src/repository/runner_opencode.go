package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

// OpenCodeRunner runs opencode in headless mode with permissions auto-approved.
// It is always standalone: kobo must not share the user's background opencode
// server, whose configuration and sessions belong to the user's own work. Only a
// private server reads OPENCODE_CONFIG_CONTENT, which carries kobo's MCP servers.
type OpenCodeRunner struct {
	bin      string
	agentURL string
	logDir   string
	agent    string
	browser  *playwright
}

func NewOpenCodeRunner(bin, agentURL, logDir, agent string, browser *playwright) *OpenCodeRunner {
	return &OpenCodeRunner{bin: bin, agentURL: agentURL, logDir: logDir, agent: agent, browser: browser}
}

func (r *OpenCodeRunner) Provider() string { return "opencode" }

// NewSessionID returns an id opencode accepts. Its session ids are prefixed with
// ses_; --session creates the session when the id does not exist yet.
func (r *OpenCodeRunner) NewSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "ses_" + hex.EncodeToString(b)
}

func (r *OpenCodeRunner) servers(logName string) []mcpServer {
	servers := []mcpServer{agentServer(r.agentURL)}
	if r.browser != nil {
		servers = append(servers, r.browser.server(r.browser.outputDir(r.logDir, logName)))
	}
	return servers
}

func (r *OpenCodeRunner) Run(ctx context.Context, req usecase.RunRequest) error {
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

	mcp, err := openCodeMCPConfig(r.servers(req.LogName))
	if err != nil {
		return err
	}

	args := []string{"run", "--standalone", "--format", "json", "--auto", "--session", req.SessionID}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if r.agent != "" {
		args = append(args, "--agent", r.agent)
	}
	args = append(args, req.Prompt)

	cmd := exec.CommandContext(ctx, r.bin, args...)
	cmd.Dir = req.WorkDir
	cmd.Env = withEnv(withEnv(os.Environ(), "OPENCODE_DISABLE_AUTOUPDATE", "1"), "OPENCODE_CONFIG_CONTENT", mcp)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	// Kill the whole process group, including opencode's private server, so no
	// process is left after a timeout or a pause.
	killGroupOnCancel(cmd)
	if err := cmd.Run(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("opencode exited: %w (log: %s)", err, logFile.Name())
	}
	return nil
}
