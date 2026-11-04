package repository

import (
	"context"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type PullRequestRepository struct {
	pg *Postgres
}

func NewPullRequestRepository(pg *Postgres) *PullRequestRepository {
	return &PullRequestRepository{pg: pg}
}

func (r *PullRequestRepository) Create(ctx context.Context, pr *domain.PullRequest) error {
	return r.pg.conn(ctx).QueryRow(ctx, `INSERT INTO pull_request (task_id, number, url, branch, state, created_at, merged_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		pr.TaskID, pr.Number, pr.URL, pr.Branch, pr.State, pr.CreatedAt, pr.MergedAt).Scan(&pr.ID)
}

func (r *PullRequestRepository) GetByTask(ctx context.Context, taskID int64) (domain.PullRequest, error) {
	var pr domain.PullRequest
	err := r.pg.conn(ctx).QueryRow(ctx, `SELECT id, task_id, number, url, branch, state, created_at, merged_at
		FROM pull_request WHERE task_id = $1`, taskID).
		Scan(&pr.ID, &pr.TaskID, &pr.Number, &pr.URL, &pr.Branch, &pr.State, &pr.CreatedAt, &pr.MergedAt)
	return pr, r.pg.notFound(err)
}

func (r *PullRequestRepository) Update(ctx context.Context, pr domain.PullRequest) error {
	_, err := r.pg.conn(ctx).Exec(ctx, `UPDATE pull_request SET number = $2, url = $3, branch = $4, state = $5, merged_at = $6
		WHERE id = $1`, pr.ID, pr.Number, pr.URL, pr.Branch, pr.State, pr.MergedAt)
	return err
}
