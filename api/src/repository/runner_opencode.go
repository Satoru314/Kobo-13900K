package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

// OpenCodeRunner runs opencode in headless mode with permissions auto-approved.
// It is always standalone: kobo must not share the user's background opencode
// server, whose configuration and sessions belong to the user's own work.
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

	// opencode reads project config from the worktree. The file is restored
	// afterwards, and the whole worktree is removed when the run ends.
	restore, err := installOpenCodeConfig(req.WorkDir, r.servers(req.LogName))
	if err != nil {
		return err
	}
	defer restore()

	args := []string{"run", "--standalone", "--format", "json", "--auto", "--session", req.SessionID}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if r.agent != "" {
		args = append(args, "--agent", r.agent)
	}
	// The worktree has a temporary opencode.json with kobo's MCP servers. Tell
	// the AI not to commit it, in case it stages everything.
	args = append(args, req.Prompt+"\n\n# Kobo の設定ファイル\n作業ディレクトリ直下の opencode.json は Kobo が実行用に置いたものです。commit しないでください。\n")

	cmd := exec.CommandContext(ctx, r.bin, args...)
	cmd.Dir = req.WorkDir
	cmd.Env = withEnv(os.Environ(), "OPENCODE_DISABLE_AUTOUPDATE", "1")
	cmd.Stdout, cmd.Stderr = logFile, logFile
	// Kill the whole process group, including opencode's private server, so no
	// process is left after a timeout or a pause.
	killGroupOnCancel(cmd)
	if err := cmd.Run(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("opencode exited: %w (log: %s)", err, logFile.Name())
	}
	return nil
}

// installOpenCodeConfig merges kobo's MCP servers into the worktree's
// opencode.json and returns a function that puts the file back. The worktree is
// disposable, but restoring keeps the repository's own config intact while the
// AI is running. JSONC is not accepted: a commented config is reported instead
// of being silently rewritten.
func installOpenCodeConfig(dir string, servers []mcpServer) (func(), error) {
	path := filepath.Join(dir, "opencode.json")
	original, readErr := os.ReadFile(path)
	existed := readErr == nil
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return nil, readErr
	}
	cfg := map[string]any{}
	if existed {
		if err := json.Unmarshal(original, &cfg); err != nil {
			return nil, fmt.Errorf("worktree opencode.json is not valid JSON: %w", err)
		}
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	if _, ok := cfg["$schema"]; !ok {
		cfg["$schema"] = "https://opencode.ai/config.json"
	}
	mcp, _ := cfg["mcp"].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
		cfg["mcp"] = mcp
	}
	entries, _ := mcp["servers"].(map[string]any)
	if entries == nil {
		entries = map[string]any{}
		mcp["servers"] = entries
	}
	for _, s := range servers {
		entry, err := openCodeServerEntry(s)
		if err != nil {
			return nil, err
		}
		entries[s.Name] = entry
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return nil, err
	}
	return func() {
		if existed {
			_ = os.WriteFile(path, original, 0o644)
			return
		}
		_ = os.Remove(path)
	}, nil
}
