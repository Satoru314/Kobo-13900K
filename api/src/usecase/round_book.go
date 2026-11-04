package usecase

import (
	"context"
	"time"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
	"github.com/Satoru314/Kobo-13900K/api/src/usecase/irepository"
)

// roundBook records review rounds. CI failures count as rounds too.
type roundBook struct {
	rounds irepository.ReviewRoundRepository
	track  tracker
}

func newRoundBook(rounds irepository.ReviewRoundRepository, track tracker) roundBook {
	return roundBook{rounds: rounds, track: track}
}

// add records the task's next round.
func (b roundBook) add(ctx context.Context, taskID int64, result domain.ReviewResult, comments string, now time.Time) (domain.ReviewRound, error) {
	existing, err := b.rounds.ListByTask(ctx, taskID)
	if err != nil {
		return domain.ReviewRound{}, err
	}
	round := domain.ReviewRound{TaskID: taskID, RoundNo: len(existing) + 1, Result: result, Comments: comments, CreatedAt: now}
	if err := b.rounds.Create(ctx, &round); err != nil {
		return domain.ReviewRound{}, err
	}
	return round, b.track.record(ctx, taskID, nil, domain.EventReviewResult,
		map[string]any{"round_no": round.RoundNo, "result": string(result)}, now)
}
