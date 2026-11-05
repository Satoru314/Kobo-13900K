package repository

import (
	"context"
	"time"

	"gorm.io/gorm/clause"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/repository/model"
)

func (d *DB) CreateTask(ctx context.Context, t *domain.Task) error {
	m := taskToModel(*t)
	if err := d.conn(ctx).Create(&m).Error; err != nil {
		return err
	}
	t.ID = m.ID
	return nil
}

func (d *DB) GetTask(ctx context.Context, id int64) (domain.Task, error) {
	var m model.Task
	err := d.conn(ctx).First(&m, id).Error
	return taskFromModel(m), notFound(err)
}

func (d *DB) GetTaskForUpdate(ctx context.Context, id int64) (domain.Task, error) {
	var m model.Task
	err := d.conn(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, id).Error
	return taskFromModel(m), notFound(err)
}

func (d *DB) ListTasks(ctx context.Context, statuses ...domain.Status) ([]domain.Task, error) {
	q := d.conn(ctx).Order("id")
	if len(statuses) > 0 {
		q = q.Where("status IN ?", statuses)
	}
	var rows []model.Task
	err := q.Find(&rows).Error
	return mapAll(rows, taskFromModel), err
}

func (d *DB) UpdateTask(ctx context.Context, t domain.Task) error {
	m := taskToModel(t)
	return d.conn(ctx).Model(&m).
		Select("title", "description", "status", "mode", "enqueued_at", "updated_at").Updates(&m).Error
}

// queueOrder is the order the queue takes waiting tasks in: fixes first, then
// reviews, then new tasks by mode (priority, immediate, off_peak), each oldest first.
const queueOrder = `CASE WHEN status = 'fix' THEN 0
	WHEN status = 'review' THEN 1
	WHEN mode = 'priority' THEN 2
	WHEN mode = 'immediate' THEN 3
	ELSE 4 END, enqueued_at, id`

// waitingStatuses are the statuses a task waits in the queue in.
var waitingStatuses = []string{string(domain.StatusDevelop), string(domain.StatusReview), string(domain.StatusFix)}

// NextRunnableTask locks and returns the waiting task the queue takes next: one
// whose enqueued_at has come, and, for a new off_peak task, only while the
// development AI's off-peak schedule is open.
func (d *DB) NextRunnableTask(ctx context.Context, now time.Time, offPeakOpen bool) (domain.Task, bool, error) {
	var rows []model.Task
	err := d.conn(ctx).
		Where("status IN ? AND enqueued_at <= ?", waitingStatuses, now).
		Where("status <> 'develop' OR mode <> 'off_peak' OR ?", offPeakOpen).
		Order(queueOrder).Limit(1).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return domain.Task{}, false, err
	}
	return taskFromModel(rows[0]), true, nil
}

func (d *DB) ListQueue(ctx context.Context) ([]domain.Task, error) {
	var rows []model.Task
	err := d.conn(ctx).Where("status IN ?", waitingStatuses).Order(queueOrder).Find(&rows).Error
	return mapAll(rows, taskFromModel), err
}
