package repository

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

type mcpEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// runnerEnv is a machine with a fake claude that records its arguments, and no
// node: PATH holds only a shell. HOME is empty, so bash reads no rc file.
type runnerEnv struct {
	t      *testing.T
	root   string
	runner *ClaudeRunner
}

func newRunnerEnv(t *testing.T) *runnerEnv {
	t.Helper()
	root := t.TempDir()
	tools := filepath.Join(root, "tools")
	mustMkdir(t, tools)
	for _, name := range []string{"sh", "bash"} {
		real, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s is not available", name)
		}
		if err := os.Symlink(real, filepath.Join(tools, name)); err != nil {
			t.Fatal(err)
		}
	}
	mustMkdir(t, filepath.Join(root, "home"))
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("PATH", tools)
	writeScript(t, filepath.Join(tools, "claude"), `for a in "$@"; do printf '%s\n' "$a"; done > "`+root+`/claude.args"`)
	return &runnerEnv{t: t, root: root,
		runner: NewClaudeRunner("claude", "http://127.0.0.1:1/mcp/agent", filepath.Join(root, "logs"))}
}

// fakeNpx puts an npx in dir that records its calls and exits with code.
func (e *runnerEnv) fakeNpx(dir string, code string) string {
	e.t.Helper()
	mustMkdir(e.t, dir)
	npx := filepath.Join(dir, "npx")
	writeScript(e.t, npx, `[ "$1" = --version ] || printf '%s\n' "$*" >> "`+e.root+`/npx.calls"
exit `+code)
	return npx
}

func (e *runnerEnv) run() map[string]mcpEntry {
	e.t.Helper()
	err := e.runner.Run(context.Background(), usecase.RunRequest{
		SessionID: "s", WorkDir: e.root, Prompt: "p", LogName: "task-1",
	})
	if err != nil {
		e.t.Fatal(err)
	}
	args := strings.Split(readFile(e.t, filepath.Join(e.root, "claude.args")), "\n")
	for i, a := range args {
		if a == "--mcp-config" {
			var cfg struct {
				McpServers map[string]mcpEntry `json:"mcpServers"`
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

func (e *runnerEnv) npxCalls() []string {
	e.t.Helper()
	b, err := os.ReadFile(filepath.Join(e.root, "npx.calls"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		e.t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestRunPassesHeadlessChromiumAndInstallsItOnce(t *testing.T) {
	e := newRunnerEnv(t)
	tools := os.Getenv("PATH")
	npx := e.fakeNpx(tools, "0")

	servers := e.run()
	if _, ok := servers["kobo-agent"]; !ok {
		t.Errorf("the agent server is missing: %v", servers)
	}
	pw, ok := servers["playwright"]
	if !ok {
		t.Fatalf("the playwright server is missing: %v", servers)
	}
	if pw.Command != npx {
		t.Errorf("command = %q, want %q", pw.Command, npx)
	}
	joined := strings.Join(pw.Args, " ")
	output := filepath.Join(e.root, "logs", "playwright", "task-1")
	for _, want := range []string{playwrightMCP, "--headless", "--browser chromium", "--output-dir " + output} {
		if !strings.Contains(joined, want) {
			t.Errorf("args %q do not contain %q", joined, want)
		}
	}

	e.run()
	install := "-y -p " + playwrightMCP + " playwright install chromium"
	if calls := e.npxCalls(); len(calls) != 1 || calls[0] != install {
		t.Errorf("npx calls = %q, want one %q", calls, install)
	}
}

// Under systemd node is only on the PATH the user's shell rc sets up.
func TestRunFindsNpxThroughInteractiveShell(t *testing.T) {
	e := newRunnerEnv(t)
	nodeBin := filepath.Join(e.root, "nvm", "bin")
	npx := e.fakeNpx(nodeBin, "0")
	rc := `echo noise from the rc file
export PATH="` + nodeBin + `:$PATH"
`
	if err := os.WriteFile(filepath.Join(e.root, "home", ".bashrc"), []byte(rc), 0o644); err != nil {
		t.Fatal(err)
	}

	pw, ok := e.run()["playwright"]
	if !ok {
		t.Fatal("the playwright server is missing")
	}
	if pw.Command != npx {
		t.Errorf("command = %q, want %q", pw.Command, npx)
	}
	if !strings.HasPrefix(pw.Env["PATH"], nodeBin+":") {
		t.Errorf("env PATH = %q, want it to start with %q", pw.Env["PATH"], nodeBin)
	}
}

func TestRunGoesOnWithoutBrowserWhenNpxIsMissing(t *testing.T) {
	e := newRunnerEnv(t)
	servers := e.run()
	if _, ok := servers["kobo-agent"]; !ok {
		t.Errorf("the agent server is missing: %v", servers)
	}
	if _, ok := servers["playwright"]; ok {
		t.Errorf("the playwright server should be left out: %v", servers)
	}
	if log := readFile(t, filepath.Join(e.root, "logs", "task-1.log")); !strings.Contains(log, "without the Playwright browser") {
		t.Errorf("the log does not say why: %q", log)
	}
}

func TestRunRetriesInstallAfterFailure(t *testing.T) {
	e := newRunnerEnv(t)
	tools := os.Getenv("PATH")
	writeScript(t, filepath.Join(tools, "npx"), `[ "$1" = --version ] && exit 0
printf 'x\n' >> "`+e.root+`/npx.calls"
exit 1`)
	if _, ok := e.run()["playwright"]; ok {
		t.Error("the playwright server should be left out while the install fails")
	}

	e.fakeNpx(tools, "0")
	if _, ok := e.run()["playwright"]; !ok {
		t.Error("the playwright server should be back once the install works")
	}
	if calls := e.npxCalls(); len(calls) != 2 {
		t.Errorf("npx calls = %q, want the failed install and the retry", calls)
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
