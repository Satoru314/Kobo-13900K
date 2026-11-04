package repository

import (
	"context"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type SessionRepository struct {
	pg *Postgres
}

func NewSessionRepository(pg *Postgres) *SessionRepository {
	return &SessionRepository{pg: pg}
}

func (r *SessionRepository) Create(ctx context.Context, s *domain.Session) error {
	return r.pg.conn(ctx).QueryRow(ctx, `INSERT INTO session (task_id, role, provider, external_session_id, created_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		s.TaskID, s.Role, s.Provider, s.ExternalSessionID, s.CreatedAt).Scan(&s.ID)
}

func (r *SessionRepository) Get(ctx context.Context, taskID int64, role domain.Role) (domain.Session, error) {
	var s domain.Session
	err := r.pg.conn(ctx).QueryRow(ctx, `SELECT id, task_id, role, provider, external_session_id, created_at
		FROM session WHERE task_id = $1 AND role = $2`, taskID, role).
		Scan(&s.ID, &s.TaskID, &s.Role, &s.Provider, &s.ExternalSessionID, &s.CreatedAt)
	return s, r.pg.notFound(err)
}
