package usecase

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

// Rules about tasks shared by the use cases.

var ErrInvalidTransition = errors.New("invalid status transition")

// transitions lists every status a task may move to from a given status.
var transitions = map[domain.Status][]domain.Status{
	domain.StatusDevelop:    {domain.StatusDeveloping, domain.StatusCancelled},
	domain.StatusDeveloping: {domain.StatusCI, domain.StatusReview, domain.StatusDevelop, domain.StatusError},
	domain.StatusCI:         {domain.StatusReview, domain.StatusFix, domain.StatusUnresolved, domain.StatusFinished},
	domain.StatusReview:     {domain.StatusReviewing, domain.StatusFinished},
	domain.StatusReviewing:  {domain.StatusDone, domain.StatusFix, domain.StatusUnresolved, domain.StatusReview, domain.StatusError},
	domain.StatusFix:        {domain.StatusFixing, domain.StatusFinished},
	domain.StatusFixing:     {domain.StatusCI, domain.StatusReview, domain.StatusFix, domain.StatusError},
	domain.StatusDone:       {domain.StatusFinished},
	domain.StatusUnresolved: {domain.StatusFinished},
	domain.StatusError:      {domain.StatusDevelop, domain.StatusReview, domain.StatusFix, domain.StatusFinished},
}

// runs pairs each waiting status with the status the task is in while the AI works on it.
var runs = map[domain.Status]domain.Status{
	domain.StatusDevelop: domain.StatusDeveloping,
	domain.StatusReview:  domain.StatusReviewing,
	domain.StatusFix:     domain.StatusFixing,
}

// runningStatuses are the statuses a task is in while the AI works on it.
var runningStatuses = []domain.Status{domain.StatusDeveloping, domain.StatusReviewing, domain.StatusFixing}

func canMoveTo(from, to domain.Status) bool { return slices.Contains(transitions[from], to) }

func waiting(s domain.Status) bool { _, ok := runs[s]; return ok }

func running(s domain.Status) bool { return slices.Contains(runningStatuses, s) }

// hidden statuses are left out of the default task list.
func hidden(s domain.Status) bool { return s == domain.StatusFinished || s == domain.StatusCancelled }

// waitingStatus is the status a run returns to when it has to be repeated; s
// itself for a status that is not running.
func waitingStatus(s domain.Status) domain.Status {
	for w, r := range runs {
		if r == s {
			return w
		}
	}
	return s
}

// newTask builds a task that waits in the queue from now.
func newTask(projectID int64, title, description string, mode domain.Mode, now time.Time) (domain.Task, error) {
	if title == "" || description == "" {
		return domain.Task{}, errors.New("title and description are required")
	}
	if !mode.Valid() {
		return domain.Task{}, fmt.Errorf("unknown mode %q", mode)
	}
	return domain.Task{ProjectID: projectID, Title: title, Description: description, Status: domain.StatusDevelop,
		Mode: mode, EnqueuedAt: &now, CreatedAt: now, UpdatedAt: now}, nil
}

// moveTo changes the status, and puts the task in the queue when the new status is a waiting one.
func moveTo(t *domain.Task, next domain.Status, now time.Time) error {
	if !canMoveTo(t.Status, next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, t.Status, next)
	}
	t.Status, t.EnqueuedAt, t.UpdatedAt = next, nil, now
	if waiting(next) {
		t.EnqueuedAt = &now
	}
	return nil
}

// start takes the task out of the queue and hands it to the AI.
func start(t *domain.Task, now time.Time) error {
	r, ok := runs[t.Status]
	if !ok {
		return fmt.Errorf("%w: task %d in %s cannot start", ErrInvalidTransition, t.ID, t.Status)
	}
	t.Status, t.EnqueuedAt, t.UpdatedAt = r, nil, now
	return nil
}

// requeue puts a running task back into the queue, to be taken out no earlier than at.
func requeue(t *domain.Task, at, now time.Time) error {
	if !running(t.Status) {
		return fmt.Errorf("%w: task %d in %s is not running", ErrInvalidTransition, t.ID, t.Status)
	}
	t.Status, t.EnqueuedAt, t.UpdatedAt = waitingStatus(t.Status), &at, now
	return nil
}

// edit replaces the content fields that are set, while the task is waiting in develop or in error.
func edit(t *domain.Task, title, description *string, mode *domain.Mode, now time.Time) error {
	if t.Status != domain.StatusDevelop && t.Status != domain.StatusError {
		return fmt.Errorf("task %d in %s cannot be edited", t.ID, t.Status)
	}
	if mode != nil && !mode.Valid() {
		return fmt.Errorf("unknown mode %q", *mode)
	}
	if title != nil {
		t.Title = *title
	}
	if description != nil {
		t.Description = *description
	}
	if mode != nil {
		t.Mode = *mode
	}
	t.UpdatedAt = now
	return nil
}

// branch is the branch the dev AI works on for the task.
func branch(t domain.Task) string { return "kobo/task-" + strconv.FormatInt(t.ID, 10) }

// fullName is the GitHub "owner/name" of a project.
func fullName(p domain.Project) string { return p.Owner + "/" + p.Name }

// nextStatus is where a task goes after a review round, given the round limit.
func nextStatus(r domain.ReviewRound, maxRounds int) domain.Status {
	if r.Result == domain.ReviewApproved {
		return domain.StatusDone
	}
	if r.RoundNo >= maxRounds {
		return domain.StatusUnresolved
	}
	return domain.StatusFix
}

// ClockRange is a range of local time within one day, in minutes since
// midnight. End may be 1440, the end of the day.
type ClockRange struct {
	Start int
	End   int
}

// String writes the range as "HH:MM-HH:MM".
func (r ClockRange) String() string {
	return clockText(r.Start) + "-" + clockText(r.End)
}

func clockText(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

// OffPeakSchedule is a week of cheap time ranges in kobo's local time. Days is
// indexed from Monday (0) to Sunday (6). A range whose End is earlier than its
// Start belongs to the day it starts on and runs until End on the next day.
type OffPeakSchedule struct {
	Days [7][]ClockRange
}

// Active reports whether t is inside one of the ranges.
func (s OffPeakSchedule) Active(t time.Time) bool {
	day := (int(t.Weekday()) + 6) % 7
	now := day*24*60 + t.Hour()*60 + t.Minute()
	// A Sunday range that runs past midnight reaches into the next week, so
	// also compare against the same time one week later.
	nextWeek := now + 7*24*60
	for d := 0; d < 7; d++ {
		for _, r := range s.Days[d] {
			start := d*24*60 + r.Start
			end := d*24*60 + r.End
			if r.End <= r.Start {
				end += 24 * 60
			}
			if (now >= start && now < end) || (nextWeek >= start && nextWeek < end) {
				return true
			}
		}
	}
	return false
}

// offPeakDays are the day keys of an off_peak schedule, from Monday.
var offPeakDays = [7]string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// ParseOffPeakSchedule reads the off_peak object of ai.json. Every day must be
// present; a day with no cheap time has an empty list. The ranges may overlap.
func ParseOffPeakSchedule(days map[string][]string) (OffPeakSchedule, error) {
	var s OffPeakSchedule
	if days == nil {
		return s, errors.New("off_peak is required")
	}
	known := make(map[string]bool, len(offPeakDays))
	for _, d := range offPeakDays {
		known[d] = true
	}
	for d := range days {
		if !known[d] {
			return s, fmt.Errorf("unknown day %q", d)
		}
	}
	for i, d := range offPeakDays {
		ranges, ok := days[d]
		if !ok {
			return s, fmt.Errorf("%s is missing", d)
		}
		for _, raw := range ranges {
			r, err := parseClockRange(raw)
			if err != nil {
				return s, fmt.Errorf("%s: %w", d, err)
			}
			s.Days[i] = append(s.Days[i], r)
		}
	}
	return s, nil
}

var clockPattern = regexp.MustCompile(`^([0-9]{1,2}):([0-9]{2})$`)

func parseClockRange(raw string) (ClockRange, error) {
	startText, endText, ok := strings.Cut(raw, "-")
	if !ok {
		return ClockRange{}, fmt.Errorf("%q: want HH:MM-HH:MM", raw)
	}
	start, err := parseClock(startText, false)
	if err != nil {
		return ClockRange{}, fmt.Errorf("%q: %w", raw, err)
	}
	end, err := parseClock(endText, true)
	if err != nil {
		return ClockRange{}, fmt.Errorf("%q: %w", raw, err)
	}
	if start == end {
		return ClockRange{}, fmt.Errorf("%q: start and end are the same", raw)
	}
	return ClockRange{Start: start, End: end}, nil
}

func parseClock(s string, end bool) (int, error) {
	m := clockPattern.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("time %q: want HH:MM", s)
	}
	hour, _ := strconv.Atoi(m[1])
	minute, _ := strconv.Atoi(m[2])
	if minute > 59 || hour > 24 || (!end && hour == 24) || (hour == 24 && minute != 0) {
		return 0, fmt.Errorf("time %q is out of range", s)
	}
	return hour*60 + minute, nil
}
