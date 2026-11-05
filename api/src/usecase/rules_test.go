package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

var t0 = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

func TestNewTask(t *testing.T) {
	if _, err := newTask(1, "", "body", domain.ModeImmediate, t0); err == nil {
		t.Error("empty title accepted")
	}
	if _, err := newTask(1, "t", "b", domain.Mode("soon"), t0); err == nil {
		t.Error("unknown mode accepted")
	}
	task, err := newTask(1, "title", "body", domain.ModeImmediate, t0)
	if err != nil || task.Status != domain.StatusDevelop || task.EnqueuedAt == nil || !task.EnqueuedAt.Equal(t0) {
		t.Errorf("new task = %+v %v, want develop and enqueued at t0", task, err)
	}
}

func TestLifecycle(t *testing.T) {
	task, _ := newTask(1, "title", "body", domain.ModeImmediate, t0)
	step := func(f func() error) {
		t.Helper()
		if err := f(); err != nil {
			t.Fatal(err)
		}
		if got := task.EnqueuedAt != nil; got != waiting(task.Status) {
			t.Errorf("%s: enqueued=%v", task.Status, got)
		}
	}
	move := func(s domain.Status) func() error { return func() error { return moveTo(&task, s, t0) } }
	begin := func() error { return start(&task, t0) }
	step(begin) // developing
	step(move(domain.StatusCI))
	step(move(domain.StatusFix))
	step(begin) // fixing
	step(move(domain.StatusCI))
	step(move(domain.StatusReview))
	step(begin) // reviewing
	step(move(domain.StatusDone))
	step(move(domain.StatusFinished))
}

func TestInvalidTransitions(t *testing.T) {
	cases := []struct{ from, to domain.Status }{
		{domain.StatusDevelop, domain.StatusDone},
		{domain.StatusDeveloping, domain.StatusDone},
		{domain.StatusCI, domain.StatusDone},
		{domain.StatusDone, domain.StatusFix},
		{domain.StatusFinished, domain.StatusDevelop},
		{domain.StatusCancelled, domain.StatusDevelop},
		{domain.StatusReview, domain.StatusCancelled},
		{domain.StatusReviewing, domain.StatusFinished},
		{domain.StatusFixing, domain.StatusFinished},
	}
	for _, c := range cases {
		task := domain.Task{Status: c.from}
		if err := moveTo(&task, c.to, t0); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("%s -> %s: err = %v", c.from, c.to, err)
		}
	}
}

func TestStartAndRequeue(t *testing.T) {
	later := t0.Add(time.Minute)
	for from, want := range runs {
		task := domain.Task{Status: from, EnqueuedAt: &t0}
		if err := start(&task, t0); err != nil || task.Status != want || task.EnqueuedAt != nil {
			t.Errorf("start from %s = %+v %v", from, task, err)
		}
		if err := start(&task, t0); err == nil {
			t.Errorf("%s: started twice", from)
		}
		if err := requeue(&task, later, t0); err != nil || task.Status != from || !task.EnqueuedAt.Equal(later) {
			t.Errorf("requeue to %s = %+v %v", from, task, err)
		}
	}
	task := domain.Task{Status: domain.StatusFix}
	if err := requeue(&task, later, t0); err == nil {
		t.Error("requeued a task that is not running")
	}
	done := domain.Task{Status: domain.StatusDone}
	if err := start(&done, t0); err == nil {
		t.Error("started a done task")
	}
}

func TestEdit(t *testing.T) {
	task, _ := newTask(1, "title", "body", domain.ModeImmediate, t0)
	title := "new"
	if err := edit(&task, &title, nil, nil, t0); err != nil || task.Title != "new" || task.Description != "body" {
		t.Errorf("edit develop: %v %+v", err, task)
	}
	task.Status = domain.StatusReview
	if err := edit(&task, &title, nil, nil, t0); err == nil {
		t.Error("edited a task under review")
	}
	task.Status = domain.StatusError
	bad := domain.Mode("x")
	if err := edit(&task, nil, nil, &bad, t0); err == nil {
		t.Error("accepted unknown mode")
	}
}

func TestBranch(t *testing.T) {
	if got := branch(domain.Task{ID: 42}); got != "kobo/task-42" {
		t.Errorf("branch = %q", got)
	}
}

func TestNextStatus(t *testing.T) {
	cases := []struct {
		round  int
		result domain.ReviewResult
		want   domain.Status
	}{
		{1, domain.ReviewApproved, domain.StatusDone},
		{1, domain.ReviewChangesRequested, domain.StatusFix},
		{1, domain.ReviewCIFailed, domain.StatusFix},
		{2, domain.ReviewChangesRequested, domain.StatusUnresolved},
		{2, domain.ReviewCIFailed, domain.StatusUnresolved},
		{2, domain.ReviewApproved, domain.StatusDone},
	}
	for _, c := range cases {
		if got := nextStatus(domain.ReviewRound{RoundNo: c.round, Result: c.result}, 2); got != c.want {
			t.Errorf("round %d %s -> %s, want %s", c.round, c.result, got, c.want)
		}
	}
}

func TestOffPeakWindow(t *testing.T) {
	night, err := ParseOffPeakWindow("23:30-06:00")
	if err != nil {
		t.Fatal(err)
	}
	day, _ := ParseOffPeakWindow("09:00-17:00")
	clock := func(h, m int) time.Time { return time.Date(2026, 11, 4, h, m, 0, 0, time.UTC) }
	cases := []struct {
		w    OffPeakWindow
		t    time.Time
		want bool
	}{
		{night, clock(23, 30), true},
		{night, clock(2, 0), true},
		{night, clock(6, 0), false},
		{night, clock(12, 0), false},
		{day, clock(9, 0), true},
		{day, clock(16, 59), true},
		{day, clock(17, 0), false},
	}
	for _, c := range cases {
		if got := inOffPeak(c.w, c.t); got != c.want {
			t.Errorf("%v contains %s = %v, want %v", c.w, c.t.Format("15:04"), got, c.want)
		}
	}
	if _, err := ParseOffPeakWindow("night"); err == nil {
		t.Error("parsed garbage")
	}
}
