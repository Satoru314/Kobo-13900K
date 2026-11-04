package repository

import (
	"context"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type ReviewRoundRepository struct {
	pg *Postgres
}

func NewReviewRoundRepository(pg *Postgres) *ReviewRoundRepository {
	return &ReviewRoundRepository{pg: pg}
}

func (r *ReviewRoundRepository) Create(ctx context.Context, rr *domain.ReviewRound) error {
	return r.pg.conn(ctx).QueryRow(ctx, `INSERT INTO review_round (task_id, round_no, result, comments, created_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		rr.TaskID, rr.RoundNo, rr.Result, rr.Comments, rr.CreatedAt).Scan(&rr.ID)
}

func (r *ReviewRoundRepository) ListByTask(ctx context.Context, taskID int64) ([]domain.ReviewRound, error) {
	rows, err := r.pg.conn(ctx).Query(ctx, `SELECT id, task_id, round_no, result, comments, created_at
		FROM review_round WHERE task_id = $1 ORDER BY round_no`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ReviewRound
	for rows.Next() {
		var rr domain.ReviewRound
		if err := rows.Scan(&rr.ID, &rr.TaskID, &rr.RoundNo, &rr.Result, &rr.Comments, &rr.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}
