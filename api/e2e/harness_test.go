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

	"github.com/Satoru314/Kobo-13900K/api/src/controller/mcp"
	"github.com/Satoru314/Kobo-13900K/api/src/controller/rest"
	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/repository"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

// ---- fakes ----

// step decides what the fake AI does on one run: call a kobo agent tool, exit
// silently (tool ""), or hang until killed (tool "hang").
type step struct {
	tool string
	args map[string]any
}

type fakeRunner struct {
	mu       sync.Mutex
	agentURL string
	steps    []step
	calls    []irepository.RunRequest
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

func (r *fakeRunner) Run(ctx context.Context, req irepository.RunRequest) error {
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

func (g *fakeGit) Remote(context.Context, string) (irepository.RemoteInfo, error) {
	return irepository.RemoteInfo{Owner: "me", Name: "sandbox", DefaultBranch: "main"}, nil
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
	ci   []irepository.CIResult // returned in order; the last one repeats
	pr   domain.PRState
	errs int // fail this many calls first
}

func (g *fakeGitHub) CI(context.Context, string, int) (irepository.CIResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.errs > 0 {
		g.errs--
		return irepository.CIResult{}, fmt.Errorf("github is down")
	}
	if len(g.ci) == 0 {
		return irepository.CIResult{State: irepository.CIPending}, nil
	}
	r := g.ci[0]
	if len(g.ci) > 1 {
		g.ci = g.ci[1:]
	}
	return r, nil
}

func (g *fakeGitHub) PR(context.Context, string, int) (irepository.PRInfo, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.pr == "" {
		return irepository.PRInfo{State: domain.PRStateOpen}, nil
	}
	return irepository.PRInfo{State: g.pr}, nil
}

// ---- harness ----

type harness struct {
	t      *testing.T
	ctx    context.Context
	srv    *httptest.Server
	queue  *usecase.QueueUsecase
	ci     *usecase.CIUsecase
	pr     *usecase.PRUsecase
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
	pg, err := repository.NewPostgres(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pg.Close)
	if err := pg.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pg.TruncateAll(ctx); err != nil {
		t.Fatal(err)
	}

	projectsDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(projectsDir, "sandbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := usecase.Settings{
		ProjectsDir: projectsDir, WorktreesDir: t.TempDir(), MaxReviewRounds: 2,
		DevTimeout: 3 * time.Second, ReviewTimeout: 3 * time.Second, MaxAutoRetries: 2, RetryDelay: 0,
	}
	h := &harness{t: t, ctx: ctx, runner: &fakeRunner{}, git: &fakeGit{worktrees: map[string]string{}},
		gh: &fakeGitHub{}, wt: settings.WorktreesDir}

	tasks := repository.NewTaskRepository(pg)
	sessions := repository.NewSessionRepository(pg)
	prs := repository.NewPullRequestRepository(pg)
	rounds := repository.NewReviewRoundRepository(pg)
	events := repository.NewEventRepository(pg)
	sig := usecase.NewQueueSignal()
	active := usecase.NewActiveRun()
	projects := usecase.NewProjectUsecase(repository.NewProjectRepository(pg), h.git, settings)
	taskUC := usecase.NewTaskUsecase(pg, tasks, prs, rounds, events, projects, sig)
	h.queue = usecase.NewQueueUsecase(pg, tasks, sessions, prs, rounds, events, projects, h.runner, h.git,
		domain.Queue{}, settings, active, sig)
	completionUC := usecase.NewCompletionUsecase(pg, tasks, sessions, prs, rounds, events, projects, h.git, settings, active, sig)
	h.ci = usecase.NewCIUsecase(pg, tasks, prs, rounds, events, projects, h.gh, settings, sig)
	h.pr = usecase.NewPRUsecase(pg, tasks, prs, events, projects, h.gh)

	mux := http.NewServeMux()
	rest.NewRouter(rest.NewTaskHandler(taskUC), rest.NewProjectHandler(projects),
		rest.NewQueueHandler(h.queue), rest.NewHealthHandler()).Register(mux)
	m := mcp.NewServer(mcp.NewTaskTools(taskUC), mcp.NewProjectTools(projects),
		mcp.NewQueueTools(h.queue), mcp.NewCompletionTools(completionUC))
	mux.Handle("/mcp", m.UserHandler())
	mux.Handle("/mcp/agent", m.AgentHandler())
	h.srv = httptest.NewServer(mux)
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

// rest sends a request and decodes the JSON response into out. It returns the status code.
func (h *harness) rest(method, path string, body any, out any) int {
	h.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, h.srv.URL+path, &buf)
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

func (h *harness) detail(id int64) usecase.TaskDetail {
	h.t.Helper()
	var d usecase.TaskDetail
	if code := h.rest("GET", fmt.Sprintf("/tasks/%d", id), nil, &d); code != 200 {
		h.t.Fatalf("GET /tasks/%d = %d", id, code)
	}
	return d
}

func (h *harness) wantStatus(id int64, want domain.Status) usecase.TaskDetail {
	h.t.Helper()
	d := h.detail(id)
	if d.Task.Status != want {
		h.t.Fatalf("task %d status = %s, want %s (last_error=%q, tool errors=%v)",
			id, d.Task.Status, want, d.LastError, h.runner.toolErrs)
	}
	return d
}

// runNext does one pass of the queue worker. It fails the test when nothing is ready.
func (h *harness) runNext() usecase.Run {
	h.t.Helper()
	run, ok, err := h.queue.Take(h.ctx, time.Now())
	if err != nil || !ok {
		h.t.Fatalf("take: ok=%v err=%v", ok, err)
	}
	runErr := h.queue.Execute(h.ctx, run)
	if err := h.queue.Settle(h.ctx, run, runErr, time.Now()); err != nil {
		h.t.Fatalf("settle: %v", err)
	}
	if len(h.git.worktrees) != 0 {
		h.t.Fatalf("worktrees left behind: %v", h.git.worktrees)
	}
	return run
}

func (h *harness) nothingReady() {
	h.t.Helper()
	if run, ok, _ := h.queue.Take(h.ctx, time.Now()); ok {
		h.t.Fatalf("task %d was ready, want nothing", run.Task.ID)
	}
}

func (h *harness) checkCI() {
	h.t.Helper()
	if err := h.ci.Check(h.ctx, time.Now()); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) checkPR() {
	h.t.Helper()
	if err := h.pr.Check(h.ctx, time.Now()); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) eventTypes(id int64) []domain.EventType {
	h.t.Helper()
	var events []domain.Event
	h.rest("GET", fmt.Sprintf("/tasks/%d/events", id), nil, &events)
	out := make([]domain.EventType, len(events))
	for i, e := range events {
		out[i] = e.Type
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
