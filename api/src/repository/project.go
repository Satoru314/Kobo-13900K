package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

type ProjectRepository struct {
	pg *Postgres
}

func NewProjectRepository(pg *Postgres) *ProjectRepository {
	return &ProjectRepository{pg: pg}
}

const projectColumns = "id, dir, owner, name, default_branch, created_at"

func (r *ProjectRepository) scan(row pgx.Row) (domain.Project, error) {
	var p domain.Project
	err := row.Scan(&p.ID, &p.Dir, &p.Owner, &p.Name, &p.DefaultBranch, &p.CreatedAt)
	return p, err
}

// Create inserts p. When another request registered the same dir first, p becomes that project.
func (r *ProjectRepository) Create(ctx context.Context, p *domain.Project) error {
	saved, err := r.scan(r.pg.conn(ctx).QueryRow(ctx, `INSERT INTO project (dir, owner, name, default_branch, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (dir) DO UPDATE SET dir = EXCLUDED.dir
		RETURNING `+projectColumns,
		p.Dir, p.Owner, p.Name, p.DefaultBranch, p.CreatedAt))
	if err != nil {
		return err
	}
	*p = saved
	return nil
}

func (r *ProjectRepository) Get(ctx context.Context, id int64) (domain.Project, error) {
	p, err := r.scan(r.pg.conn(ctx).QueryRow(ctx, "SELECT "+projectColumns+" FROM project WHERE id = $1", id))
	return p, r.pg.notFound(err)
}

func (r *ProjectRepository) GetByDir(ctx context.Context, dir string) (domain.Project, error) {
	p, err := r.scan(r.pg.conn(ctx).QueryRow(ctx, "SELECT "+projectColumns+" FROM project WHERE dir = $1", dir))
	return p, r.pg.notFound(err)
}

func (r *ProjectRepository) List(ctx context.Context) ([]domain.Project, error) {
	rows, err := r.pg.conn(ctx).Query(ctx, "SELECT "+projectColumns+" FROM project ORDER BY dir")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Project
	for rows.Next() {
		p, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
