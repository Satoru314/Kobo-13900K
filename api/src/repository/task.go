package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type TaskRepository struct {
	pg *Postgres
}

func NewTaskRepository(pg *Postgres) *TaskRepository {
	return &TaskRepository{pg: pg}
}

const taskColumns = "id, project_id, title, description, status, mode, running, enqueued_at, created_at, updated_at"

func (r *TaskRepository) scan(row pgx.Row) (domain.Task, error) {
	var t domain.Task
	err := row.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Status, &t.Mode,
		&t.Running, &t.EnqueuedAt, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func (r *TaskRepository) Create(ctx context.Context, t *domain.Task) error {
	return r.pg.conn(ctx).QueryRow(ctx, `INSERT INTO task
		(project_id, title, description, status, mode, running, enqueued_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		t.ProjectID, t.Title, t.Description, t.Status, t.Mode, t.Running, t.EnqueuedAt, t.CreatedAt, t.UpdatedAt,
	).Scan(&t.ID)
}

func (r *TaskRepository) Get(ctx context.Context, id int64) (domain.Task, error) {
	t, err := r.scan(r.pg.conn(ctx).QueryRow(ctx, "SELECT "+taskColumns+" FROM task WHERE id = $1", id))
	return t, r.pg.notFound(err)
}

func (r *TaskRepository) GetForUpdate(ctx context.Context, id int64) (domain.Task, error) {
	t, err := r.scan(r.pg.conn(ctx).QueryRow(ctx, "SELECT "+taskColumns+" FROM task WHERE id = $1 FOR UPDATE", id))
	return t, r.pg.notFound(err)
}

func (r *TaskRepository) List(ctx context.Context, statuses ...domain.Status) ([]domain.Task, error) {
	sql := "SELECT " + taskColumns + " FROM task"
	var args []any
	if len(statuses) > 0 {
		names := make([]string, len(statuses))
		for i, st := range statuses {
			names[i] = string(st)
		}
		sql += " WHERE status = ANY($1)"
		args = append(args, names)
	}
	rows, err := r.pg.conn(ctx).Query(ctx, sql+" ORDER BY id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Task
	for rows.Next() {
		t, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *TaskRepository) Update(ctx context.Context, t domain.Task) error {
	_, err := r.pg.conn(ctx).Exec(ctx, `UPDATE task SET
		title = $2, description = $3, status = $4, mode = $5, running = $6, enqueued_at = $7, updated_at = $8
		WHERE id = $1`,
		t.ID, t.Title, t.Description, t.Status, t.Mode, t.Running, t.EnqueuedAt, t.UpdatedAt)
	return err
}
