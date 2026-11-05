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

type claudeMCPEntry struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

type runnerEnv struct {
	t       *testing.T
	root    string
	runner  *ClaudeRunner
	browser string // the configured playwright-mcp path, empty when disabled
}

func newRunnerEnv(t *testing.T, withBrowser bool) *runnerEnv {
	t.Helper()
	root := t.TempDir()
	tools := filepath.Join(root, "tools")
	mustMkdir(t, tools)
	claude := filepath.Join(tools, "claude")
	writeScript(t, claude, `for a in "$@"; do printf '%s\n' "$a"; done > "`+root+`/claude.args"`)

	var browser *playwright
	browserPath := ""
	if withBrowser {
		browserPath = filepath.Join(tools, "playwright-mcp")
		writeScript(t, browserPath, "exit 0")
		browser = NewPlaywright(browserPath)
	}
	return &runnerEnv{t: t, root: root, browser: browserPath,
		runner: NewClaudeRunner(claude, "http://127.0.0.1:1/mcp/agent", filepath.Join(root, "logs"), browser)}
}

func (e *runnerEnv) run(req usecase.RunRequest) []string {
	e.t.Helper()
	req.SessionID = defaultString(req.SessionID, "s")
	req.WorkDir = e.root
	req.Prompt = defaultString(req.Prompt, "p")
	req.LogName = defaultString(req.LogName, "task-1")
	if err := e.runner.Run(context.Background(), req); err != nil {
		e.t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(readFile(e.t, filepath.Join(e.root, "claude.args"))), "\n")
}

func (e *runnerEnv) servers(args []string) map[string]claudeMCPEntry {
	e.t.Helper()
	for i, a := range args {
		if a == "--mcp-config" {
			var cfg struct {
				McpServers map[string]claudeMCPEntry `json:"mcpServers"`
			}
			if err := json.Unmarshal([]byte(args[i+1]), &cfg); err != nil {
				e.t.Fatal(err)
			}
			return cfg.McpServers
		}
	}
	e.t.Fatalf("no --mcp-config in %v", args)
	return nil
}

func defaultString(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func TestRunPassesAgentAndBrowser(t *testing.T) {
	e := newRunnerEnv(t, true)
	args := e.run(usecase.RunRequest{})
	servers := e.servers(args)
	if servers["kobo-agent"].URL != "http://127.0.0.1:1/mcp/agent" {
		t.Errorf("agent server = %+v", servers["kobo-agent"])
	}
	pw, ok := servers["kobo-playwright"]
	if !ok {
		t.Fatalf("playwright server missing: %+v", servers)
	}
	if pw.Command != e.browser {
		t.Errorf("command = %q, want %q", pw.Command, e.browser)
	}
	joined := strings.Join(pw.Args, " ")
	output := filepath.Join(e.root, "logs", "playwright", "task-1")
	for _, want := range []string{"--headless", "--browser chromium", "--isolated", "--output-dir " + output} {
		if !strings.Contains(joined, want) {
			t.Errorf("args %q do not contain %q", joined, want)
		}
	}
}

func TestRunWithoutBrowserLeavesPlaywrightOut(t *testing.T) {
	e := newRunnerEnv(t, false)
	servers := e.servers(e.run(usecase.RunRequest{}))
	if _, ok := servers["kobo-playwright"]; ok {
		t.Errorf("playwright should be absent: %+v", servers)
	}
}

func TestRunUsesTheSessionModelAndResume(t *testing.T) {
	e := newRunnerEnv(t, false)
	args := e.run(usecase.RunRequest{Model: "claude-sonnet-5-5"})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--model claude-sonnet-5-5") {
		t.Errorf("args = %v", args)
	}
	if !strings.Contains(joined, "--session-id s") {
		t.Errorf("new session args = %v", args)
	}

	args = e.run(usecase.RunRequest{Model: "claude-sonnet-5-5", Resume: true})
	joined = strings.Join(args, " ")
	if !strings.Contains(joined, "--resume s") {
		t.Errorf("resume args = %v", args)
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
