package domain

import (
	"testing"
	"time"
)

func at(min int) *time.Time {
	v := t0.Add(time.Duration(min) * time.Minute)
	return &v
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
		if got := c.w.Contains(c.t); got != c.want {
			t.Errorf("%v contains %s = %v, want %v", c.w, c.t.Format("15:04"), got, c.want)
		}
	}
	if _, err := ParseOffPeakWindow("night"); err == nil {
		t.Error("parsed garbage")
	}
}

func TestQueueOrder(t *testing.T) {
	tasks := []Task{
		{ID: 1, Status: StatusDevelop, Mode: ModeOffPeak, EnqueuedAt: at(0)},
		{ID: 2, Status: StatusDevelop, Mode: ModeImmediate, EnqueuedAt: at(5)},
		{ID: 3, Status: StatusReview, Mode: ModeImmediate, EnqueuedAt: at(9)},
		{ID: 4, Status: StatusFix, Mode: ModeOffPeak, EnqueuedAt: at(10)},
		{ID: 5, Status: StatusDevelop, Mode: ModeImmediate, EnqueuedAt: at(1)},
		{ID: 6, Status: StatusFixing},
		{ID: 7, Status: StatusDone},
		{ID: 8, Status: StatusDevelop, Mode: ModePriority, EnqueuedAt: at(20)},
	}
	got := Queue{}.Order(tasks)
	want := []int64{4, 3, 8, 5, 2, 1}
	if len(got) != len(want) {
		t.Fatalf("order = %v", got)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("position %d = task %d, want %d", i, got[i].ID, id)
		}
	}
}

func TestQueueNext(t *testing.T) {
	window, _ := ParseOffPeakWindow("01:00-07:00")
	q := Queue{OffPeak: window}
	noon := t0                                            // 12:00, outside the window
	night := time.Date(2026, 11, 5, 2, 0, 0, 0, time.UTC) // inside

	offPeak := Task{ID: 1, Status: StatusDevelop, Mode: ModeOffPeak, EnqueuedAt: at(0)}
	if _, ok := q.Next([]Task{offPeak}, noon); ok {
		t.Error("off_peak task ran outside the window")
	}
	if got, ok := q.Next([]Task{offPeak}, night); !ok || got.ID != 1 {
		t.Error("off_peak task did not run inside the window")
	}

	offPeakFix := Task{ID: 2, Status: StatusFix, Mode: ModeOffPeak, EnqueuedAt: at(0)}
	if got, ok := q.Next([]Task{offPeak, offPeakFix}, noon); !ok || got.ID != 2 {
		t.Error("fix of an off_peak task was held back")
	}

	future := Task{ID: 3, Status: StatusDevelop, Mode: ModeImmediate, EnqueuedAt: at(30)}
	if _, ok := q.Next([]Task{future}, noon); ok {
		t.Error("task ran before its enqueued_at")
	}
	waiting := Task{ID: 4, Status: StatusFix, EnqueuedAt: at(30)}
	ready := Task{ID: 5, Status: StatusDevelop, Mode: ModeImmediate, EnqueuedAt: at(0)}
	if got, ok := q.Next([]Task{waiting, ready}, noon); !ok || got.ID != 5 {
		t.Errorf("waiting fix blocked a ready task: %+v %v", got, ok)
	}
	priority := Task{ID: 6, Status: StatusDevelop, Mode: ModePriority, EnqueuedAt: at(0)}
	if got, ok := q.Next([]Task{ready, priority}, noon); !ok || got.ID != 6 {
		t.Errorf("priority task did not go first: %+v %v", got, ok)
	}
}
