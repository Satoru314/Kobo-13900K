package repository

import "context"

type QueueStateRepository struct {
	pg *Postgres
}

func NewQueueStateRepository(pg *Postgres) *QueueStateRepository {
	return &QueueStateRepository{pg: pg}
}

func (r *QueueStateRepository) Paused(ctx context.Context) (bool, error) {
	var paused bool
	err := r.pg.conn(ctx).QueryRow(ctx, "SELECT paused FROM queue_state WHERE id = 1").Scan(&paused)
	return paused, err
}

func (r *QueueStateRepository) SetPaused(ctx context.Context, paused bool) error {
	_, err := r.pg.conn(ctx).Exec(ctx, "UPDATE queue_state SET paused = $1 WHERE id = 1", paused)
	return err
}
