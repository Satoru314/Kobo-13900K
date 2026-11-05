package e2e

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

// The list carries what a card shows: project, PR and CI state.
func TestListShowsPRAndCI(t *testing.T) {
	h := newHarness(t)
	h.git.workflows = true
	task := h.createTask("card", "")
	var list []usecase.TaskSummary
	h.rest("GET", "/tasks", nil, &list)
	if len(list) != 1 || list[0].Project != "sandbox" || list[0].PRURL != "" || list[0].CI != "" {
		t.Fatalf("before PR: %+v", list)
	}

	h.runner.script(devDone(7))
	h.runNext()
	h.rest("GET", "/tasks", nil, &list)
	if list[0].PRNumber != 7 || !strings.HasSuffix(list[0].PRURL, "/pull/7") || list[0].CI != "pending" {
		t.Fatalf("in ci: %+v", list[0])
	}

	h.gh.ci = []irepository.CIResult{{State: irepository.CIFailure, Summary: "x"}}
	h.checkCI()
	h.wantStatus(task.ID, domain.StatusFix)
	h.rest("GET", "/tasks", nil, &list)
	if list[0].CI != "failure" {
		t.Fatalf("after CI failure: %+v", list[0])
	}
}

// Every directory under the projects directory is listed with why it cannot be added.
func TestProjectCandidatesAndAdd(t *testing.T) {
	h := newHarness(t)
	for _, d := range []string{"other/.git", "local/.git", "plain"} {
		if err := os.MkdirAll(filepath.Join(h.dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	h.createTask("registers sandbox", "")

	var got []usecase.ProjectCandidate
	if code := h.rest("GET", "/projects/candidates", nil, &got); code != 200 {
		t.Fatalf("candidates = %d", code)
	}
	want := []usecase.ProjectCandidate{
		{Dir: "local", Problem: usecase.ProblemNotGitHub},
		{Dir: "other"},
		{Dir: "plain", Problem: usecase.ProblemNotGit},
		{Dir: "sandbox", Registered: true},
	}
	if len(got) != len(want) {
		t.Fatalf("candidates = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candidate %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	var p domain.Project
	if code := h.rest("POST", "/projects", map[string]any{"dir": "other"}, &p); code != 201 || p.Dir != "other" {
		t.Fatalf("add = %d %+v", code, p)
	}
	if code := h.rest("POST", "/projects", map[string]any{"dir": "../x"}, nil); code != 400 {
		t.Fatalf("add outside = %d, want 400", code)
	}
}

// /events sends the ID of every task that changes.
func TestEventsStream(t *testing.T) {
	h := newHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", h.srv.URL+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type = %q", ct)
	}
	lines := bufio.NewScanner(resp.Body)
	next := func() string {
		t.Helper()
		for lines.Scan() {
			if data, ok := strings.CutPrefix(lines.Text(), "data: "); ok {
				return data
			}
		}
		t.Fatalf("stream ended: %v", lines.Err())
		return ""
	}

	task := h.createTask("watched", "")
	if got := next(); got != `{"task_id": 1}` || task.ID != 1 {
		t.Fatalf("after create: %s (task %d)", got, task.ID)
	}
	h.runner.script(devDone(3))
	h.runNext()
	for range 2 { // taken out, then the AI reported
		if got := next(); got != `{"task_id": 1}` {
			t.Fatalf("after run: %s", got)
		}
	}
}
