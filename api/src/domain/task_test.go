package domain

import (
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

func newTestTask(t *testing.T) Task {
	t.Helper()
	task, err := NewTask(1, "title", "body", ModeImmediate, t0)
	if err != nil {
		t.Fatal(err)
	}
	task.ID = 7
	return task
}

func TestNewTaskValidates(t *testing.T) {
	if _, err := NewTask(1, "", "body", ModeImmediate, t0); err == nil {
		t.Error("empty title accepted")
	}
	if _, err := NewTask(1, "t", "b", Mode("soon"), t0); err == nil {
		t.Error("unknown mode accepted")
	}
	task := newTestTask(t)
	if task.Status != StatusRunnable || task.EnqueuedAt == nil || !task.EnqueuedAt.Equal(t0) {
		t.Errorf("new task = %+v, want runnable and enqueued at t0", task)
	}
}

func TestFullLifecycle(t *testing.T) {
	task := newTestTask(t)
	steps := []Status{StatusCI, StatusFix, StatusCI, StatusReviewing, StatusDone, StatusFinished}
	if err := task.Start(t0); err != nil {
		t.Fatal(err)
	}
	if task.Status != StatusDeveloping || !task.Running || task.EnqueuedAt != nil {
		t.Fatalf("after start = %+v", task)
	}
	for _, s := range steps {
		if err := task.MoveTo(s, t0); err != nil {
			t.Fatalf("move to %s: %v", s, err)
		}
		if got := task.EnqueuedAt != nil; got != s.Queued() {
			t.Errorf("%s: enqueued=%v, want %v", s, got, s.Queued())
		}
	}
}

func TestInvalidTransitions(t *testing.T) {
	cases := []struct{ from, to Status }{
		{StatusRunnable, StatusDone},
		{StatusDeveloping, StatusDone},
		{StatusCI, StatusDone},
		{StatusDone, StatusFix},
		{StatusFinished, StatusRunnable},
		{StatusCancelled, StatusRunnable},
		{StatusReviewing, StatusCancelled},
	}
	for _, c := range cases {
		task := Task{Status: c.from}
		if err := task.MoveTo(c.to, t0); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("%s -> %s: err = %v, want ErrInvalidTransition", c.from, c.to, err)
		}
	}
}

func TestStartKeepsReviewAndFixStatus(t *testing.T) {
	for _, s := range []Status{StatusReviewing, StatusFix} {
		task := Task{Status: s, EnqueuedAt: &t0}
		if err := task.Start(t0); err != nil {
			t.Fatal(err)
		}
		if task.Status != s || !task.Running {
			t.Errorf("start from %s = %+v", s, task)
		}
		if err := task.Start(t0); err == nil {
			t.Errorf("%s: started twice", s)
		}
	}
	task := Task{Status: StatusDone}
	if err := task.Start(t0); err == nil {
		t.Error("started a done task")
	}
}

func TestRequeue(t *testing.T) {
	later := t0.Add(time.Minute)
	task := newTestTask(t)
	_ = task.Start(t0)
	if err := task.Requeue(later, t0); err != nil {
		t.Fatal(err)
	}
	if task.Status != StatusRunnable || task.Running || !task.EnqueuedAt.Equal(later) {
		t.Errorf("requeue from developing = %+v", task)
	}

	fix := Task{Status: StatusFix, Running: true}
	if err := fix.Requeue(later, t0); err != nil {
		t.Fatal(err)
	}
	if fix.Status != StatusFix || fix.Running {
		t.Errorf("requeue from fix = %+v", fix)
	}
}

func TestEdit(t *testing.T) {
	task := newTestTask(t)
	title := "new"
	if err := task.Edit(&title, nil, nil, t0); err != nil || task.Title != "new" || task.Description != "body" {
		t.Errorf("edit runnable: %v %+v", err, task)
	}
	task.Status = StatusReviewing
	if err := task.Edit(&title, nil, nil, t0); err == nil {
		t.Error("edited a task under review")
	}
	task.Status = StatusError
	bad := Mode("x")
	if err := task.Edit(nil, nil, &bad, t0); err == nil {
		t.Error("accepted unknown mode")
	}
}

func TestBranch(t *testing.T) {
	if got := (Task{ID: 42}).Branch(); got != "kobo/task-42" {
		t.Errorf("Branch() = %q", got)
	}
}

func TestReviewRoundNextStatus(t *testing.T) {
	cases := []struct {
		round  int
		result ReviewResult
		want   Status
	}{
		{1, ReviewApproved, StatusDone},
		{1, ReviewChangesRequested, StatusFix},
		{1, ReviewCIFailed, StatusFix},
		{2, ReviewChangesRequested, StatusUnresolved},
		{2, ReviewCIFailed, StatusUnresolved},
		{2, ReviewApproved, StatusDone},
	}
	for _, c := range cases {
		r := ReviewRound{RoundNo: c.round, Result: c.result}
		if got := r.NextStatus(2); got != c.want {
			t.Errorf("round %d %s -> %s, want %s", c.round, c.result, got, c.want)
		}
	}
}
