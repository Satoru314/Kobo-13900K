package repository

import (
	"context"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/repository/model"
)

func (d *DB) CreatePullRequest(ctx context.Context, pr *domain.PullRequest) error {
	m := pullRequestToModel(*pr)
	if err := d.conn(ctx).Create(&m).Error; err != nil {
		return err
	}
	pr.ID = m.ID
	return nil
}

func (d *DB) GetPullRequestByTask(ctx context.Context, taskID int64) (domain.PullRequest, error) {
	var m model.PullRequest
	err := d.conn(ctx).Where("task_id = ?", taskID).First(&m).Error
	return pullRequestFromModel(m), notFound(err)
}

func (d *DB) UpdatePullRequest(ctx context.Context, pr domain.PullRequest) error {
	m := pullRequestToModel(pr)
	return d.conn(ctx).Model(&m).Select("number", "url", "branch", "state", "merged_at").Updates(&m).Error
}

func (d *DB) CreateSession(ctx context.Context, s *domain.Session) error {
	m := sessionToModel(*s)
	if err := d.conn(ctx).Create(&m).Error; err != nil {
		return err
	}
	s.ID = m.ID
	return nil
}

func (d *DB) GetSession(ctx context.Context, taskID int64, role domain.Role) (domain.Session, error) {
	var m model.Session
	err := d.conn(ctx).Where("task_id = ? AND role = ?", taskID, string(role)).First(&m).Error
	return sessionFromModel(m), notFound(err)
}

func (d *DB) CreateReviewRound(ctx context.Context, r *domain.ReviewRound) error {
	m := reviewRoundToModel(*r)
	if err := d.conn(ctx).Create(&m).Error; err != nil {
		return err
	}
	r.ID = m.ID
	return nil
}

func (d *DB) ListReviewRounds(ctx context.Context, taskID int64) ([]domain.ReviewRound, error) {
	var rows []model.ReviewRound
	err := d.conn(ctx).Where("task_id = ?", taskID).Order("round_no").Find(&rows).Error
	return mapAll(rows, reviewRoundFromModel), err
}

func (d *DB) CreateEvent(ctx context.Context, e *domain.Event) error {
	m := eventToModel(*e)
	if err := d.conn(ctx).Create(&m).Error; err != nil {
		return err
	}
	e.ID = m.ID
	return nil
}

func (d *DB) ListEvents(ctx context.Context, taskID int64) ([]domain.Event, error) {
	var rows []model.Event
	err := d.conn(ctx).Where("task_id = ?", taskID).Order("id").Find(&rows).Error
	return mapAll(rows, eventFromModel), err
}

func (d *DB) GetQueueState(ctx context.Context) (domain.QueueState, error) {
	var m model.QueueState
	err := d.conn(ctx).First(&m, 1).Error
	return domain.QueueState{Paused: m.Paused}, err
}

func (d *DB) SetQueuePaused(ctx context.Context, paused bool) error {
	return d.conn(ctx).Model(&model.QueueState{ID: 1}).Update("paused", paused).Error
}
