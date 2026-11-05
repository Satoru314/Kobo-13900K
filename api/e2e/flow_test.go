package e2e

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

// Issue → develop → CI → review → done → merged → hidden.
func TestHappyPath(t *testing.T) {
	h := newHarness(t)
	h.git.workflows = true
	task := h.createTask("add login", "")
	h.wantStatus(task.ID, domain.StatusDevelop)

	var projects struct {
		Projects []domain.Project `json:"projects"`
	}
	if err := h.callUser("list_projects", map[string]any{}, &projects); err != nil || len(projects.Projects) != 1 ||
		projects.Projects[0].FullName() != "me/sandbox" {
		t.Fatalf("project not registered from git remote: %+v %v", projects, err)
	}

	h.runner.script(devDone(12))
	run := h.runNext()
	if run.Role != domain.RoleDev || run.Resume || !strings.Contains(run.Prompt, "kobo/task-1") {
		t.Fatalf("dev run = %+v", run)
	}
	d := h.wantStatus(task.ID, domain.StatusCI)
	if d.PullRequest == nil || d.PullRequest.Number != 12 || d.PullRequest.Branch != "kobo/task-1" {
		t.Fatalf("pull request = %+v", d.PullRequest)
	}

	h.checkCI() // pending
	h.wantStatus(task.ID, domain.StatusCI)
	h.gh.ci = []irepository.CIResult{{State: irepository.CISuccess}}
	h.checkCI()
	h.wantStatus(task.ID, domain.StatusReview)

	h.runner.script(approve())
	run = h.runNext()
	if run.Role != domain.RoleReview || !strings.Contains(run.Prompt, "1 周目") {
		t.Fatalf("review run = %+v", run)
	}
	d = h.wantStatus(task.ID, domain.StatusDone)
	if len(d.Rounds) != 1 || d.Rounds[0].Result != domain.ReviewApproved {
		t.Fatalf("rounds = %+v", d.Rounds)
	}

	h.checkPR() // still open
	h.wantStatus(task.ID, domain.StatusDone)
	h.gh.pr = domain.PRStateMerged
	h.checkPR()
	d = h.wantStatus(task.ID, domain.StatusFinished)
	if d.PullRequest.State != domain.PRStateMerged {
		t.Fatalf("pr state = %s", d.PullRequest.State)
	}

	var list []domain.Task
	h.rest("GET", "/tasks", nil, &list)
	if len(list) != 0 {
		t.Fatalf("finished task still listed: %+v", list)
	}
	h.rest("GET", "/tasks?all=true", nil, &list)
	if len(list) != 1 {
		t.Fatalf("all=true list = %+v", list)
	}

	want := []domain.EventType{domain.EventTaskCreated, domain.EventStatusChanged, domain.EventSessionStarted,
		domain.EventAIInput, domain.EventAIResult, domain.EventPRCreated, domain.EventStatusChanged,
		domain.EventCIResult, domain.EventStatusChanged, domain.EventStatusChanged, domain.EventSessionStarted, domain.EventAIInput,
		domain.EventAIResult, domain.EventReviewResult, domain.EventStatusChanged, domain.EventStatusChanged}
	if got := h.eventTypes(task.ID); !slices.Equal(got, want) {
		t.Fatalf("events =\n%v\nwant\n%v", got, want)
	}
	if len(h.runner.toolErrs) != 0 {
		t.Fatalf("tool errors: %v", h.runner.toolErrs)
	}
}

// Two rounds of change requests end in unresolved; the fix resumes the dev session.
func TestReviewLoopEndsUnresolved(t *testing.T) {
	h := newHarness(t)
	task := h.createTask("refactor", "")
	h.runner.script(devDone(3), requestChanges("rename x"), devDone(3), requestChanges("still bad"))

	h.runNext()
	h.wantStatus(task.ID, domain.StatusReview) // no CI in this repo

	h.runNext()
	h.wantStatus(task.ID, domain.StatusFix)

	fix := h.runNext()
	if fix.Role != domain.RoleDev || !fix.Resume || !strings.Contains(fix.Prompt, "rename x") {
		t.Fatalf("fix run = %+v", fix)
	}
	if got := h.git.added[len(h.git.added)-1]; got != "kobo/task-1 from kobo/task-1" {
		t.Fatalf("fix worktree = %q, want the PR branch", got)
	}
	if fix.WorkDir != h.runner.calls[0].WorkDir {
		t.Fatalf("fix worktree path %s differs from the first %s", fix.WorkDir, h.runner.calls[0].WorkDir)
	}
	h.wantStatus(task.ID, domain.StatusReview)

	review := h.runNext()
	if !review.Resume || !strings.Contains(review.Prompt, "2 周目") {
		t.Fatalf("second review = %+v", review)
	}
	d := h.wantStatus(task.ID, domain.StatusUnresolved)
	if len(d.Rounds) != 2 {
		t.Fatalf("rounds = %+v", d.Rounds)
	}
	if got := h.git.added[1]; got != " from kobo/task-1" {
		t.Fatalf("review worktree = %q, want detached on the PR branch", got)
	}

	h.gh.pr = domain.PRStateClosed
	h.checkPR()
	h.wantStatus(task.ID, domain.StatusFinished)
}

// A CI failure is a round: CI fail → fix → CI pass → review changes → unresolved.
func TestCIFailureCountsAsRound(t *testing.T) {
	h := newHarness(t)
	h.git.workflows = true
	task := h.createTask("ci", "")
	h.runner.script(devDone(5), devDone(5), requestChanges("nope"))

	h.runNext()
	h.gh.ci = []irepository.CIResult{{State: irepository.CIFailure, Summary: "test-job failed"}}
	h.checkCI()
	d := h.wantStatus(task.ID, domain.StatusFix)
	if d.Rounds[0].Result != domain.ReviewCIFailed {
		t.Fatalf("round = %+v", d.Rounds[0])
	}

	fix := h.runNext()
	if !strings.Contains(fix.Prompt, "CI の失敗") || !strings.Contains(fix.Prompt, "test-job failed") {
		t.Fatalf("fix prompt does not carry the CI failure:\n%s", fix.Prompt)
	}
	h.wantStatus(task.ID, domain.StatusCI)
	h.gh.ci = []irepository.CIResult{{State: irepository.CISuccess}}
	h.checkCI()
	h.runNext()
	h.wantStatus(task.ID, domain.StatusUnresolved)
}

// A GitHub outage during the CI check is retried on the next check.
func TestCICheckSurvivesGitHubErrors(t *testing.T) {
	h := newHarness(t)
	h.git.workflows = true
	task := h.createTask("flaky github", "")
	h.runner.script(devDone(1))
	h.runNext()
	h.gh.errs = 1
	h.gh.ci = []irepository.CIResult{{State: irepository.CISuccess}}
	if err := h.ci.Check(h.ctx, time.Now()); err == nil {
		t.Fatal("expected the outage to be reported")
	}
	h.wantStatus(task.ID, domain.StatusCI)
	h.checkCI()
	h.wantStatus(task.ID, domain.StatusReview)
}

// Silent exits are retried twice, then the task goes to error; retry resumes at the failed stage.
func TestSystemErrorsRetryThenError(t *testing.T) {
	h := newHarness(t)
	task := h.createTask("crashy", "")
	h.runner.script(exitSilently, exitSilently, exitSilently)

	h.runNext()
	h.wantStatus(task.ID, domain.StatusDevelop)
	h.runNext()
	h.wantStatus(task.ID, domain.StatusDevelop)
	h.runNext()
	d := h.wantStatus(task.ID, domain.StatusError)
	if !strings.Contains(d.LastError, "without reporting back") {
		t.Fatalf("last_error = %q", d.LastError)
	}
	h.nothingReady()
	if !h.runner.calls[1].Resume {
		t.Fatal("automatic retry did not resume the session")
	}

	var retried domain.Task
	if code := h.rest("POST", fmt.Sprintf("/tasks/%d/retry", task.ID), nil, &retried); code != 200 {
		t.Fatalf("retry = %d", code)
	}
	h.wantStatus(task.ID, domain.StatusDevelop)
	h.runner.script(devDone(9))
	h.runNext()
	h.wantStatus(task.ID, domain.StatusReview)
}

// A run that never reports is killed at the timeout and counts as a failure.
func TestTimeoutKillsRun(t *testing.T) {
	h := newHarness(t)
	task := h.createTask("slow", "")
	h.runner.script(hang)
	start := time.Now()
	h.runNext()
	if time.Since(start) > 10*time.Second {
		t.Fatal("run was not stopped at the timeout")
	}
	h.wantStatus(task.ID, domain.StatusDevelop)
	var events []domain.Event
	h.rest("GET", fmt.Sprintf("/tasks/%d/events", task.ID), nil, &events)
	last := events[len(events)-1]
	if last.Type != domain.EventSystemError || !strings.Contains(fmt.Sprint(last.Payload["message"]), "within") {
		t.Fatalf("last event = %+v", last)
	}
}

// The AI reports it cannot go on: error at once, editable, retryable.
func TestBlockedThenEditAndRetry(t *testing.T) {
	h := newHarness(t)
	task := h.createTask("impossible", "")
	h.runner.script(blocked("spec contradicts itself"))
	h.runNext()
	d := h.wantStatus(task.ID, domain.StatusError)
	if d.LastError != "blocked: spec contradicts itself" {
		t.Fatalf("last_error = %q", d.LastError)
	}
	var edited domain.Task
	if code := h.rest("PATCH", fmt.Sprintf("/tasks/%d", task.ID), map[string]any{"description": "clearer"}, &edited); code != 200 || edited.Description != "clearer" {
		t.Fatalf("edit in error = %d %+v", code, edited)
	}
	if err := h.callUser("retry_task", map[string]any{"id": task.ID}, nil); err != nil {
		t.Fatal(err)
	}
	h.wantStatus(task.ID, domain.StatusDevelop)
}

func TestCancelAndEditRules(t *testing.T) {
	h := newHarness(t)
	a := h.createTask("a", "")
	b := h.createTask("b", "")
	if code := h.rest("POST", fmt.Sprintf("/tasks/%d/cancel", a.ID), nil, nil); code != 200 {
		t.Fatalf("cancel develop = %d", code)
	}
	h.wantStatus(a.ID, domain.StatusCancelled)

	h.runner.script(devDone(2))
	h.runNext()
	h.wantStatus(b.ID, domain.StatusReview)
	if code := h.rest("POST", fmt.Sprintf("/tasks/%d/cancel", b.ID), nil, nil); code != 400 {
		t.Fatalf("cancel under review = %d, want 400", code)
	}
	if code := h.rest("PATCH", fmt.Sprintf("/tasks/%d", b.ID), map[string]any{"title": "x"}, nil); code != 400 {
		t.Fatalf("edit under review = %d, want 400", code)
	}
	if code := h.rest("POST", fmt.Sprintf("/tasks/%d/retry", b.ID), nil, nil); code != 400 {
		t.Fatalf("retry not in error = %d, want 400", code)
	}
	if code := h.rest("GET", "/tasks/999", nil, nil); code != 404 {
		t.Fatalf("missing task = %d, want 404", code)
	}
	if err := h.callUser("create_task", map[string]any{"project": "../etc", "title": "t", "description": "d"}, nil); err == nil {
		t.Fatal("accepted a project outside the projects directory")
	}
	if err := h.callUser("create_task", map[string]any{"project": "nope", "title": "t", "description": "d"}, nil); err == nil {
		t.Fatal("accepted a missing project directory")
	}
}

// Review fixes jump ahead of new tasks; off_peak tasks wait for the window.
func TestQueuePriority(t *testing.T) {
	h := newHarness(t)
	first := h.createTask("first", "")
	h.createTask("night job", domain.ModeOffPeak) // never runs: the default window is empty in tests
	second := h.createTask("second", "")

	h.runner.script(devDone(1), requestChanges("fix it"), devDone(1))
	h.runNext() // first: dev
	h.runNext() // first: review, ahead of second because reviews rank above new tasks
	h.wantStatus(first.ID, domain.StatusFix)
	h.wantStatus(second.ID, domain.StatusDevelop)

	var queue struct {
		Tasks []domain.Task `json:"tasks"`
	}
	if err := h.callUser("get_queue", map[string]any{}, &queue); err != nil {
		t.Fatal(err)
	}
	if len(queue.Tasks) != 3 || queue.Tasks[0].ID != first.ID || queue.Tasks[1].ID != second.ID {
		t.Fatalf("queue order = %+v", queue.Tasks)
	}

	fix := h.runNext()
	if fix.Task.ID != first.ID {
		t.Fatalf("ran task %d, want the fix of %d", fix.Task.ID, first.ID)
	}
	h.runner.script(approve(), devDone(2), approve())
	if run := h.runNext(); run.Task.ID != first.ID || run.Role != domain.RoleReview {
		t.Fatalf("ran task %d as %s, want the second review of %d", run.Task.ID, run.Role, first.ID)
	}
	h.wantStatus(first.ID, domain.StatusDone)
	if run := h.runNext(); run.Task.ID != second.ID {
		t.Fatalf("ran task %d, want %d", run.Task.ID, second.ID)
	}
	h.runNext()
	h.wantStatus(second.ID, domain.StatusDone)
	h.nothingReady() // only the off_peak task is left
}

// priority tasks go ahead of immediate ones that were created earlier.
func TestPriorityMode(t *testing.T) {
	h := newHarness(t)
	normal := h.createTask("normal", "")
	urgent := h.createTask("urgent", domain.ModePriority)
	h.runner.script(devDone(1), devDone(2))
	if run := h.runNext(); run.Task.ID != urgent.ID {
		t.Fatalf("ran task %d, want the priority task %d", run.Task.ID, urgent.ID)
	}
	if run := h.runNext(); run.Task.ID != urgent.ID || run.Role != domain.RoleReview {
		t.Fatalf("ran task %d as %s, want the review of %d", run.Task.ID, run.Role, urgent.ID)
	}
	h.wantStatus(normal.ID, domain.StatusDevelop)
}

// A task left running when kobo stopped is put back without counting as a failure.
func TestRecoverAfterRestart(t *testing.T) {
	h := newHarness(t)
	task := h.createTask("interrupted", "")
	if _, ok, err := h.queue.Take(h.ctx, time.Now()); !ok || err != nil {
		t.Fatalf("take: %v %v", ok, err)
	}
	h.wantStatus(task.ID, domain.StatusDeveloping)
	h.nothingReady() // one at a time
	if err := h.queue.Recover(h.ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	h.wantStatus(task.ID, domain.StatusDevelop)
	h.runner.script(devDone(4))
	h.runNext()
	h.wantStatus(task.ID, domain.StatusReview)
}

// Reports with nothing running, or from the wrong role, are rejected.
func TestAgentToolsRejectMisuse(t *testing.T) {
	h := newHarness(t)
	task := h.createTask("misuse", "")
	h.runner.script(approve(), devDone(1))
	h.runNext() // dev run calls complete_review: rejected, then exits → system error
	if len(h.runner.toolErrs) != 1 || !strings.Contains(h.runner.toolErrs[0], "complete_review is for reviews") {
		t.Fatalf("tool errors = %v", h.runner.toolErrs)
	}
	h.wantStatus(task.ID, domain.StatusDevelop)
	h.runNext()
	h.wantStatus(task.ID, domain.StatusReview)
}
