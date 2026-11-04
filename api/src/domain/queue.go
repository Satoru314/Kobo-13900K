package domain

import (
	"fmt"
	"sort"
	"time"
)

// OffPeakWindow is a daily time range in local time. End may be earlier than
// Start, in which case the window wraps past midnight.
type OffPeakWindow struct {
	Start time.Duration
	End   time.Duration
}

// ParseOffPeakWindow reads "HH:MM-HH:MM".
func ParseOffPeakWindow(s string) (OffPeakWindow, error) {
	var sh, sm, eh, em int
	if _, err := fmt.Sscanf(s, "%d:%d-%d:%d", &sh, &sm, &eh, &em); err != nil {
		return OffPeakWindow{}, fmt.Errorf("off-peak window %q: want HH:MM-HH:MM", s)
	}
	clock := func(h, m int) time.Duration { return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute }
	return OffPeakWindow{Start: clock(sh, sm), End: clock(eh, em)}, nil
}

// Contains reports whether t falls inside the window.
func (w OffPeakWindow) Contains(t time.Time) bool {
	y, m, d := t.Date()
	sinceMidnight := t.Sub(time.Date(y, m, d, 0, 0, 0, 0, t.Location()))
	if w.Start <= w.End {
		return sinceMidnight >= w.Start && sinceMidnight < w.End
	}
	return sinceMidnight >= w.Start || sinceMidnight < w.End
}

type Queue struct {
	OffPeak OffPeakWindow
}

// rank orders queued tasks: review fixes first, then reviews, then new tasks.
func (Queue) rank(t Task) int {
	switch {
	case t.Status == StatusFix:
		return 0
	case t.Status == StatusReviewing:
		return 1
	case t.Mode == ModeImmediate:
		return 2
	default:
		return 3
	}
}

// Order returns the queued, not running tasks in the order they will be taken out.
func (q Queue) Order(tasks []Task) []Task {
	var out []Task
	for _, t := range tasks {
		if t.Status.Queued() && !t.Running && t.EnqueuedAt != nil {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if ri, rj := q.rank(out[i]), q.rank(out[j]); ri != rj {
			return ri < rj
		}
		if !out[i].EnqueuedAt.Equal(*out[j].EnqueuedAt) {
			return out[i].EnqueuedAt.Before(*out[j].EnqueuedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Ready reports whether a queued task may be taken out at now. The mode only
// holds back new tasks; reviews and fixes of a started task always go ahead.
func (q Queue) Ready(t Task, now time.Time) bool {
	if t.EnqueuedAt == nil || t.EnqueuedAt.After(now) {
		return false
	}
	return t.Status != StatusRunnable || t.Mode == ModeImmediate || q.OffPeak.Contains(now)
}

// Next picks the task to run at now, if any.
func (q Queue) Next(tasks []Task, now time.Time) (Task, bool) {
	for _, t := range q.Order(tasks) {
		if q.Ready(t, now) {
			return t, true
		}
	}
	return Task{}, false
}
