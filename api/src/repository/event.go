package repository

import (
	"context"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type EventRepository struct {
	pg *Postgres
}

func NewEventRepository(pg *Postgres) *EventRepository {
	return &EventRepository{pg: pg}
}

func (r *EventRepository) Create(ctx context.Context, e *domain.Event) error {
	if e.Payload == nil {
		e.Payload = map[string]any{}
	}
	return r.pg.conn(ctx).QueryRow(ctx, `INSERT INTO event (task_id, session_id, type, payload, created_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		e.TaskID, e.SessionID, e.Type, e.Payload, e.CreatedAt).Scan(&e.ID)
}

func (r *EventRepository) ListByTask(ctx context.Context, taskID int64) ([]domain.Event, error) {
	rows, err := r.pg.conn(ctx).Query(ctx, `SELECT id, task_id, session_id, type, payload, created_at
		FROM event WHERE task_id = $1 ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Event
	for rows.Next() {
		var e domain.Event
		if err := rows.Scan(&e.ID, &e.TaskID, &e.SessionID, &e.Type, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
