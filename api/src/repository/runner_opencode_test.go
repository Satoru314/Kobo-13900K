package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

type openCodeEnv struct {
	t      *testing.T
	root   string
	runner *OpenCodeRunner
}

func newOpenCodeEnv(t *testing.T, withBrowser bool) *openCodeEnv {
	t.Helper()
	root := t.TempDir()
	tools := filepath.Join(root, "tools")
	mustMkdir(t, tools)
	opencode := filepath.Join(tools, "opencode")
	writeScript(t, opencode, `for a in "$@"; do printf '%s\n' "$a"; done > "`+root+`/opencode.args"
printf '%s' "$OPENCODE_CONFIG_CONTENT" > "`+root+`/opencode.seen.json"`)

	var browser *playwright
	if withBrowser {
		bin := filepath.Join(tools, "playwright-mcp")
		writeScript(t, bin, "exit 0")
		browser = NewPlaywright(bin)
	}
	return &openCodeEnv{t: t, root: root,
		runner: NewOpenCodeRunner(opencode, "http://127.0.0.1:1/mcp/agent", filepath.Join(root, "logs"), "build", browser)}
}

func (e *openCodeEnv) run(req usecase.RunRequest) ([]string, error) {
	e.t.Helper()
	req.SessionID = defaultString(req.SessionID, "ses_test")
	req.WorkDir = e.root
	req.Prompt = defaultString(req.Prompt, "p")
	req.LogName = defaultString(req.LogName, "task-1")
	if err := e.runner.Run(context.Background(), req); err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(readFile(e.t, filepath.Join(e.root, "opencode.args"))), "\n"), nil
}

type openCodeConfig struct {
	Model string `json:"model"`
	MCP   struct {
		Servers map[string]struct {
			Type        string            `json:"type"`
			URL         string            `json:"url"`
			Command     []string          `json:"command"`
			Environment map[string]string `json:"environment"`
		} `json:"servers"`
	} `json:"mcp"`
}

func TestOpenCodeRunPassesMCPViaEnvironment(t *testing.T) {
	e := newOpenCodeEnv(t, true)
	args, err := e.run(usecase.RunRequest{AI: "deepseek-v4.1-flash", Model: "opencode-go/deepseek-v4.1-flash"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"run", "--standalone", "--format json", "--auto", "--session ses_test",
		"--model opencode-go/deepseek-v4.1-flash", "--agent build"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args %q do not contain %q", joined, want)
		}
	}

	var cfg openCodeConfig
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(e.root, "opencode.seen.json"))), &cfg); err != nil {
		t.Fatal(err)
	}
	agent, ok := cfg.MCP.Servers["kobo-agent"]
	if !ok || agent.Type != "remote" || agent.URL != "http://127.0.0.1:1/mcp/agent" {
		t.Errorf("agent server = %+v", agent)
	}
	pw, ok := cfg.MCP.Servers["kobo-playwright"]
	if !ok || pw.Type != "local" || len(pw.Command) == 0 {
		t.Fatalf("playwright server = %+v", pw)
	}
	if !strings.Contains(strings.Join(pw.Command, " "), "--headless") {
		t.Errorf("playwright command = %v", pw.Command)
	}
	if _, err := os.Stat(filepath.Join(e.root, "opencode.json")); !os.IsNotExist(err) {
		t.Errorf("opencode.json was written into the worktree: %v", err)
	}
}

func TestOpenCodeSessionIDHasItsPrefix(t *testing.T) {
	e := newOpenCodeEnv(t, false)
	if id := e.runner.NewSessionID(); !strings.HasPrefix(id, "ses_") {
		t.Fatalf("session id = %q", id)
	}
}
