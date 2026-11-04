package usecase

import (
	"context"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

// tracker saves task changes together with the events that describe them.
type tracker struct {
	tasks  irepository.TaskRepository
	events irepository.EventRepository
}

func newTracker(tasks irepository.TaskRepository, events irepository.EventRepository) tracker {
	return tracker{tasks: tasks, events: events}
}

// record adds one event to the task's history.
func (k tracker) record(ctx context.Context, taskID int64, sessionID *int64, typ domain.EventType, payload map[string]any, now time.Time) error {
	e := domain.Event{TaskID: taskID, SessionID: sessionID, Type: typ, Payload: payload, CreatedAt: now}
	return k.events.Create(ctx, &e)
}

// move changes the task's status, saves it and records the change.
func (k tracker) move(ctx context.Context, t *domain.Task, next domain.Status, reason string, now time.Time) error {
	from := t.Status
	if err := t.MoveTo(next, now); err != nil {
		return err
	}
	if err := k.tasks.Update(ctx, *t); err != nil {
		return err
	}
	payload := map[string]any{"from": string(from), "to": string(next)}
	if reason != "" {
		payload["reason"] = reason
	}
	return k.record(ctx, t.ID, nil, domain.EventStatusChanged, payload, now)
}
