// Package e2e drives kobo through REST and MCP against a real Postgres. The AI,
// GitHub and git are fakes; everything else is the real code.
//
// Needs KOBO_TEST_DATABASE_URL, e.g. postgres://kobo:kobo@127.0.0.1:5433/kobo_test?sslmode=disable
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/labstack/echo/v4"

	"github.com/Satoru314/Kobo-13900K/api/src/controller/mcp"
	"github.com/Satoru314/Kobo-13900K/api/src/controller/rest"
	"github.com/Satoru314/Kobo-13900K/api/src/controller/schema"
	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/notify"
	"github.com/Satoru314/Kobo-13900K/api/src/repository"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

// ---- fakes ----

// step decides what the fake AI does on one run: call a kobo agent tool, exit
// silently (tool ""), or hang until killed (tool "hang").
type step struct {
	tool string
	args map[string]any
}

// testCatalog is the ai.json used by the tests.
type testCatalog struct{}

var catalogModels = []usecase.AIModel{
	{ID: "claude", Label: "Claude", Provider: "claude"},
	{ID: "deepseek-v4.1-flash", Label: "DeepSeek V4.1 Flash", Provider: "opencode",
		Model: "opencode-go/deepseek-v4.1-flash"},
}

func (testCatalog) Get(id string) (usecase.AIModel, bool) {
	for _, m := range catalogModels {
		if m.ID == id {
			return m, true
		}
	}
	return usecase.AIModel{}, false
}

func (testCatalog) List() []usecase.AIModel { return catalogModels }

type fakeRunner struct {
	mu       sync.Mutex
	agentURL string
	steps    []step
	calls    []usecase.RunRequest
	toolErrs []string
	nextID   int
}

func (r *fakeRunner) Provider() string { return "fake" }

func (r *fakeRunner) NewSessionID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	return fmt.Sprintf("sess-%d", r.nextID)
}

func (r *fakeRunner) script(steps ...step) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = append(r.steps, steps...)
}

func (r *fakeRunner) Run(ctx context.Context, req usecase.RunRequest) error {
	r.mu.Lock()
	r.calls = append(r.calls, req)
	if len(r.steps) == 0 {
		r.mu.Unlock()
		return fmt.Errorf("fake runner: no step scripted")
	}
	s := r.steps[0]
	r.steps = r.steps[1:]
	r.mu.Unlock()

	if _, err := os.Stat(req.WorkDir); err != nil {
		return fmt.Errorf("fake runner: worktree missing: %w", err)
	}
	switch s.tool {
	case "":
		return nil
	case "hang":
		<-ctx.Done()
		return nil
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "fake-ai"}, nil)
	cs, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: r.agentURL, DisableStandaloneSSE: true}, nil)
	if err != nil {
		return err
	}
	defer cs.Close()
	res, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: s.tool, Arguments: s.args})
	if err != nil {
		return err
	}
	if res.IsError {
		r.mu.Lock()
		b, _ := json.Marshal(res.Content)
		r.toolErrs = append(r.toolErrs, string(b))
		r.mu.Unlock()
		return nil
	}
	// A real AI process is killed by kobo after it reports.
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("fake runner: kobo did not stop the run after %s", s.tool)
	}
}

type fakeGit struct {
	mu        sync.Mutex
	workflows bool
	worktrees map[string]string // path -> branch ("" when detached)
	added     []string
}

func (g *fakeGit) Remote(context.Context, string) (usecase.RemoteInfo, error) {
	return usecase.RemoteInfo{Owner: "me", Name: "sandbox", DefaultBranch: "main"}, nil
}

// GitHubOrigin treats every repository as on GitHub except one named "local".
func (g *fakeGit) GitHubOrigin(_ context.Context, repoDir string) (string, string, bool, error) {
	if filepath.Base(repoDir) == "local" {
		return "", "", false, nil
	}
	return "me", filepath.Base(repoDir), true, nil
}

func (g *fakeGit) AddWorktree(_ context.Context, _, path, branch, base string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.worktrees[path] = branch
	g.added = append(g.added, fmt.Sprintf("%s from %s", branch, base))
	return os.MkdirAll(path, 0o755)
}

func (g *fakeGit) RemoveWorktree(_ context.Context, _, path string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.worktrees, path)
	return os.RemoveAll(path)
}

func (g *fakeGit) HasWorkflows(context.Context, string, string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.workflows, nil
}

type fakeGitHub struct {
	mu   sync.Mutex
	ci   []usecase.CIResult // returned in order; the last one repeats
	pr   domain.PRState
	errs int // fail this many calls first
}

func (g *fakeGitHub) CI(context.Context, string, int) (usecase.CIResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.errs > 0 {
		g.errs--
		return usecase.CIResult{}, fmt.Errorf("github is down")
	}
	if len(g.ci) == 0 {
		return usecase.CIResult{State: usecase.CIPending}, nil
	}
	r := g.ci[0]
	if len(g.ci) > 1 {
		g.ci = g.ci[1:]
	}
	return r, nil
}

func (g *fakeGitHub) PullRequest(context.Context, string, int) (usecase.PRInfo, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.pr == "" {
		return usecase.PRInfo{State: domain.PRStateOpen}, nil
	}
	return usecase.PRInfo{State: g.pr}, nil
}

// fakeRepoHost is git and GitHub together, as usecase.RepoHost.
type fakeRepoHost struct {
	*fakeGit
	*fakeGitHub
}

// ---- harness ----

type harness struct {
	t      *testing.T
	ctx    context.Context
	srv    *httptest.Server
	dir    string // projects directory
	uc     *usecase.Usecase
	runner *fakeRunner
	git    *fakeGit
	gh     *fakeGitHub
	user   *sdk.ClientSession
	wt     string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	url := os.Getenv("KOBO_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KOBO_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := repository.NewDB(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.TruncateAll(ctx); err != nil {
		t.Fatal(err)
	}

	projectsDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(projectsDir, "sandbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := usecase.Settings{
		ProjectsDir: projectsDir, WorktreesDir: t.TempDir(), MaxReviewRounds: 2,
		DevTimeout: 3 * time.Second, ReviewTimeout: 3 * time.Second, MaxAutoRetries: 2, RetryDelay: 0,
		ShotsRepo: "acme/shots",
	}
	h := &harness{t: t, ctx: ctx, runner: &fakeRunner{}, git: &fakeGit{worktrees: map[string]string{}},
		gh: &fakeGitHub{}, wt: settings.WorktreesDir, dir: projectsDir}

	hub := notify.New()
	h.uc = usecase.New(db, fakeRepoHost{h.git, h.gh},
		map[string]usecase.AI{"claude": h.runner, "opencode": h.runner}, testCatalog{}, hub, settings)

	e := echo.New()
	rest.Register(e, h.uc, hub)
	e.Any("/mcp", echo.WrapHandler(mcp.UserHandler(h.uc)))
	e.Any("/mcp/agent", echo.WrapHandler(mcp.AgentHandler(h.uc)))
	h.srv = httptest.NewServer(e)
	t.Cleanup(h.srv.Close)
	h.runner.agentURL = h.srv.URL + "/mcp/agent"

	client := sdk.NewClient(&sdk.Implementation{Name: "e2e-user"}, nil)
	h.user, err = client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: h.srv.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.user.Close() })
	return h
}

// callUser calls a user MCP tool and decodes its structured result into out.
func (h *harness) callUser(tool string, args any, out any) error {
	h.t.Helper()
	res, err := h.user.CallTool(h.ctx, &sdk.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		h.t.Fatalf("%s: %v", tool, err)
	}
	if res.IsError {
		b, _ := json.Marshal(res.Content)
		return fmt.Errorf("%s: tool error: %s", tool, b)
	}
	b, _ := json.Marshal(res.StructuredContent)
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			h.t.Fatalf("%s: decode %s: %v", tool, b, err)
		}
	}
	return nil
}

func (h *harness) createTask(title string, mode domain.Mode) domain.Task {
	h.t.Helper()
	var task domain.Task
	args := map[string]any{"project": "sandbox", "title": title, "description": "do " + title}
	if mode != "" {
		args["mode"] = mode
	}
	if err := h.callUser("create_task", args, &task); err != nil {
		h.t.Fatal(err)
	}
	return task
}

func (h *harness) setAI(dev, review string, separate bool) {
	h.t.Helper()
	body := map[string]any{"dev_ai": dev, "review_ai": review, "separate_review": separate}
	if code := h.rest("PUT", "/ai/setting", body, nil); code != 200 {
		h.t.Fatalf("PUT /ai/setting = %d", code)
	}
}

// rest sends a request and decodes the JSON response into out. It returns the status code.
func (h *harness) rest(method, path string, body any, out any) int {
	h.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, h.srv.URL+path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func (h *harness) detail(id int64) schema.TaskDetail {
	h.t.Helper()
	var d schema.TaskDetail
	if code := h.rest("GET", fmt.Sprintf("/tasks/%d", id), nil, &d); code != 200 {
		h.t.Fatalf("GET /tasks/%d = %d", id, code)
	}
	return d
}

func (h *harness) wantStatus(id int64, want domain.Status) schema.TaskDetail {
	h.t.Helper()
	d := h.detail(id)
	if d.Task.Status != string(want) {
		h.t.Fatalf("task %d status = %s, want %s (last_error=%q, tool errors=%v)",
			id, d.Task.Status, want, d.LastError, h.runner.toolErrs)
	}
	return d
}

// ranRun is what the fake AI was started with on one run.
type ranRun struct {
	usecase.RunRequest
	Task struct{ ID int64 }
}

// runNext does one pass of the queue worker. It fails the test when nothing is ready.
func (h *harness) runNext() ranRun {
	h.t.Helper()
	before := len(h.runner.calls)
	ran, err := h.uc.RunNext(h.ctx, time.Now())
	if err != nil || !ran {
		h.t.Fatalf("run next: ran=%v err=%v", ran, err)
	}
	if len(h.git.worktrees) != 0 {
		h.t.Fatalf("worktrees left behind: %v", h.git.worktrees)
	}
	if len(h.runner.calls) == before {
		h.t.Fatal("the AI was not started")
	}
	r := ranRun{RunRequest: h.runner.calls[len(h.runner.calls)-1]}
	if _, err := fmt.Sscanf(r.LogName, "task-%d-", &r.Task.ID); err != nil {
		h.t.Fatalf("log name %q: %v", r.LogName, err)
	}
	return r
}

func (h *harness) nothingReady() {
	h.t.Helper()
	before := len(h.runner.calls)
	if ran, err := h.uc.RunNext(h.ctx, time.Now()); ran || len(h.runner.calls) != before {
		h.t.Fatalf("a task was ready, want nothing (err=%v)", err)
	}
}

func (h *harness) checkCI() {
	h.t.Helper()
	if err := h.uc.CheckCI(h.ctx, time.Now()); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) checkPR() {
	h.t.Helper()
	if err := h.uc.CheckPullRequests(h.ctx, time.Now()); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) eventTypes(id int64) []domain.EventType {
	h.t.Helper()
	var events []schema.Event
	h.rest("GET", fmt.Sprintf("/tasks/%d/events", id), nil, &events)
	out := make([]domain.EventType, len(events))
	for i, e := range events {
		out[i] = domain.EventType(e.Type)
	}
	return out
}

// ---- scripted AI behaviour ----

func devDone(n int) step {
	return step{tool: "complete_development", args: map[string]any{
		"pr_url": fmt.Sprintf("https://github.com/me/sandbox/pull/%d", n), "summary": "done"}}
}

func approve() step {
	return step{tool: "complete_review", args: map[string]any{"result": "approved"}}
}

func requestChanges(c string) step {
	return step{tool: "complete_review", args: map[string]any{"result": "changes_requested", "comments": c}}
}

func blocked(reason string) step {
	return step{tool: "report_blocked", args: map[string]any{"reason": reason}}
}

var exitSilently = step{}
var hang = step{tool: "hang"}
