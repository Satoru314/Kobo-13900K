package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

// GetAIView returns the catalog and the current development/review choices.
func (u *Usecase) GetAIView(ctx context.Context) (AIView, error) {
	setting, err := u.db.GetAISetting(ctx)
	if err != nil {
		return AIView{}, err
	}
	models := u.catalog.List()
	if models == nil {
		models = []AIModel{}
	}
	return AIView{Models: models, Setting: setting}, nil
}

// SetAISetting changes the models new sessions start with. Both ids must be in
// ai.json, so the queue cannot pick a dead model.
func (u *Usecase) SetAISetting(ctx context.Context, in SetAISettingInput, now time.Time) (domain.AISetting, error) {
	for _, id := range []string{in.DevAI, in.ReviewAI} {
		if _, ok := u.catalog.Get(id); !ok {
			return domain.AISetting{}, fmt.Errorf("%w: unknown AI model %q", ErrInvalidInput, id)
		}
	}
	setting := domain.AISetting{DevAI: in.DevAI, ReviewAI: in.ReviewAI, SeparateReview: in.SeparateReview, UpdatedAt: now}
	if err := u.db.SetAISetting(ctx, setting); err != nil {
		return domain.AISetting{}, err
	}
	return setting, nil
}

// ValidateAISetting checks the stored choice against the catalog. It runs at
// startup so a removed model is noticed before a task reaches it.
func (u *Usecase) ValidateAISetting(ctx context.Context) error {
	setting, err := u.db.GetAISetting(ctx)
	if err != nil {
		return fmt.Errorf("read AI setting: %w", err)
	}
	if _, ok := u.catalog.Get(setting.DevAI); !ok {
		return fmt.Errorf("AI setting: development model %q is not in the catalog", setting.DevAI)
	}
	if setting.SeparateReview {
		if _, ok := u.catalog.Get(setting.ReviewAI); !ok {
			return fmt.Errorf("AI setting: review model %q is not in the catalog", setting.ReviewAI)
		}
	}
	return nil
}

// sessionAI picks the runner and model for a new session of role. Review uses
// its own model only when the setting separates review.
func (u *Usecase) sessionAI(ctx context.Context, role domain.Role) (AIModel, AI, error) {
	setting, err := u.db.GetAISetting(ctx)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return AIModel{}, nil, fmt.Errorf("%w: AI setting is missing", ErrAIUnavailable)
		}
		return AIModel{}, nil, fmt.Errorf("read AI setting: %w", err)
	}
	id := setting.DevAI
	if role == domain.RoleReview && setting.SeparateReview {
		id = setting.ReviewAI
	}
	model, ok := u.catalog.Get(id)
	if !ok {
		return AIModel{}, nil, fmt.Errorf("%w: AI model %q is not in the catalog", ErrAIUnavailable, id)
	}
	runner, ok := u.ais[model.Provider]
	if !ok {
		return AIModel{}, nil, fmt.Errorf("%w: provider %q for AI model %q is not configured", ErrAIUnavailable, model.Provider, id)
	}
	return model, runner, nil
}
