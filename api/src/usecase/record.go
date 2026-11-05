package usecase

import (
	"context"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

// record adds one event to the task's history.
func (u *Usecase) record(ctx context.Context, taskID int64, sessionID *int64, typ domain.EventType, payload map[string]any, now time.Time) error {
	if payload == nil {
		payload = map[string]any{}
	}
	e := domain.Event{TaskID: taskID, SessionID: sessionID, Type: typ, Payload: payload, CreatedAt: now}
	return u.db.CreateEvent(ctx, &e)
}

// move changes the task's status, saves it and records the change.
func (u *Usecase) move(ctx context.Context, t *domain.Task, next domain.Status, reason string, now time.Time) error {
	from := t.Status
	if err := moveTo(t, next, now); err != nil {
		return err
	}
	if err := u.db.UpdateTask(ctx, *t); err != nil {
		return err
	}
	payload := map[string]any{"from": string(from), "to": string(next)}
	if reason != "" {
		payload["reason"] = reason
	}
	return u.record(ctx, t.ID, nil, domain.EventStatusChanged, payload, now)
}

// addRound records the task's next review round. CI failures count as rounds too.
func (u *Usecase) addRound(ctx context.Context, taskID int64, result domain.ReviewResult, comments string, now time.Time) (domain.ReviewRound, error) {
	existing, err := u.db.ListReviewRounds(ctx, taskID)
	if err != nil {
		return domain.ReviewRound{}, err
	}
	round := domain.ReviewRound{TaskID: taskID, RoundNo: len(existing) + 1, Result: result, Comments: comments, CreatedAt: now}
	if err := u.db.CreateReviewRound(ctx, &round); err != nil {
		return domain.ReviewRound{}, err
	}
	return round, u.record(ctx, taskID, nil, domain.EventReviewResult,
		map[string]any{"round_no": round.RoundNo, "result": string(result)}, now)
}
