package repository

import (
	"context"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/repository/model"
)

func (d *DB) GetAISetting(ctx context.Context) (domain.AISetting, error) {
	var m model.AISetting
	err := d.conn(ctx).First(&m, 1).Error
	return aiSettingFromModel(m), notFound(err)
}

func (d *DB) SetAISetting(ctx context.Context, s domain.AISetting) error {
	m := aiSettingToModel(s)
	return d.conn(ctx).Model(&m).Select("dev_ai", "review_ai", "separate_review", "updated_at").Updates(&m).Error
}

func (d *DB) ListSessions(ctx context.Context, taskID int64) ([]domain.Session, error) {
	var rows []model.Session
	err := d.conn(ctx).Where("task_id = ?", taskID).Order("id").Find(&rows).Error
	return mapAll(rows, sessionFromModel), err
}
