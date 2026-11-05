package schema

import (
	"slices"
	"testing"

	"github.com/Satoru314/Kobo-13900K/api/src/usecase"
)

func TestFromAIModelIncludesOffPeak(t *testing.T) {
	model := usecase.AIModel{
		ID: "x",
		OffPeak: usecase.OffPeakSchedule{Days: [7][]usecase.ClockRange{
			{{Start: 13 * 60, End: 15 * 60}},
			nil,
			nil,
			nil,
			nil,
			{{Start: 0, End: 24 * 60}},
			{{Start: 19 * 60, End: 10 * 60}},
		}},
	}
	got := FromAIModel(model)
	if len(got.OffPeak) != 7 {
		t.Fatalf("off_peak days = %d, want 7", len(got.OffPeak))
	}
	if want := []string{"13:00-15:00"}; !slices.Equal(got.OffPeak["mon"], want) {
		t.Errorf("mon = %v, want %v", got.OffPeak["mon"], want)
	}
	if want := []string{"00:00-24:00"}; !slices.Equal(got.OffPeak["sat"], want) {
		t.Errorf("sat = %v, want %v", got.OffPeak["sat"], want)
	}
	if want := []string{"19:00-10:00"}; !slices.Equal(got.OffPeak["sun"], want) {
		t.Errorf("sun = %v, want %v", got.OffPeak["sun"], want)
	}
	if got.OffPeak["tue"] == nil || len(got.OffPeak["tue"]) != 0 {
		t.Errorf("tue = %v, want an empty list", got.OffPeak["tue"])
	}
}
