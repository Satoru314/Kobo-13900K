package repository

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/repository/model"
)

// CreateProject inserts p. When another request registered the same dir first,
// p becomes that project.
func (d *DB) CreateProject(ctx context.Context, p *domain.Project) error {
	m := projectToModel(*p)
	if err := d.conn(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "dir"}}, DoNothing: true}).
		Create(&m).Error; err != nil {
		return err
	}
	saved, err := d.GetProjectByDir(ctx, p.Dir)
	*p = saved
	return err
}

func (d *DB) GetProject(ctx context.Context, id int64) (domain.Project, error) {
	var m model.Project
	err := d.conn(ctx).First(&m, id).Error
	return projectFromModel(m), notFound(err)
}

func (d *DB) GetProjectByDir(ctx context.Context, dir string) (domain.Project, error) {
	var m model.Project
	err := d.conn(ctx).Where("dir = ?", dir).First(&m).Error
	return projectFromModel(m), notFound(err)
}

func (d *DB) ListProjects(ctx context.Context) ([]domain.Project, error) {
	var rows []model.Project
	err := d.conn(ctx).Order("dir").Find(&rows).Error
	return mapAll(rows, projectFromModel), err
}
