package usecase

import (
	"errors"
	"strings"
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

func TestOffPeakSchedule(t *testing.T) {
	weekday := []string{"13:00-15:00", "19:00-10:00"}
	schedule, err := ParseOffPeakSchedule(map[string][]string{
		"mon": weekday,
		"tue": weekday,
		"wed": weekday,
		"thu": weekday,
		"fri": weekday,
		"sat": {"00:00-24:00"},
		"sun": {"00:00-24:00", "19:00-10:00"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clock := func(day, h, m int) time.Time {
		return time.Date(2026, 11, day, h, m, 0, 0, time.Local)
	}
	cases := []struct {
		name string
		t    time.Time
		want bool
	}{
		{"mon 09:00 from sunday 19:00", clock(2, 9, 0), true},
		{"mon 10:30", clock(2, 10, 30), false},
		{"mon 14:00", clock(2, 14, 0), true},
		{"fri 20:00", clock(6, 20, 0), true},
		{"sat 12:00", clock(7, 12, 0), true},
		{"sun 23:00", clock(8, 23, 0), true},
	}
	for _, c := range cases {
		if got := schedule.Active(c.t); got != c.want {
			t.Errorf("%s: Active = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestParseOffPeakScheduleRejectsBadInput(t *testing.T) {
	days := func(mon []string) map[string][]string {
		return map[string][]string{"mon": mon, "tue": nil, "wed": nil, "thu": nil, "fri": nil, "sat": nil, "sun": nil}
	}
	for _, tc := range []struct {
		name string
		days map[string][]string
		want string
	}{
		{"missing", nil, "required"},
		{"missing day", map[string][]string{"mon": {}}, "tue is missing"},
		{"unknown day", map[string][]string{"mon": {}, "tue": {}, "wed": {}, "thu": {}, "fri": {}, "sat": {}, "sun": {}, "monday": {}}, "unknown day"},
		{"garbage", days([]string{"night"}), "want HH:MM-HH:MM"},
		{"same", days([]string{"09:00-09:00"}), "same"},
		{"bad minute", days([]string{"09:60-10:00"}), "out of range"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseOffPeakSchedule(tc.days)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}
