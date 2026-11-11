package lifecycle

import "testing"

func TestPlanRoundAmplified_TryIsFailuresPlusOneAndFlagsNeverRegress(t *testing.T) {
	t.Parallel()

	for _, ceiling := range []int{0, 1, 2, 4, 100} {
		var prev Round
		for failures := 0; failures < 50; failures++ {
			got := PlanRound(failures, ceiling)
			if got.Try != failures+1 {
				t.Fatalf("PlanRound(%d, %d).Try = %d, want %d", failures, ceiling, got.Try, failures+1)
			}
			if failures > 0 {
				if prev.CarryFindings && !got.CarryFindings || prev.RaiseEffort && !got.RaiseEffort || prev.ChangeStrategy && !got.ChangeStrategy {
					t.Fatalf("flags regressed between failures %d and %d at ceiling %d: %+v then %+v", failures-1, failures, ceiling, prev, got)
				}
			}
			prev = got
		}
	}
}

func TestPlanRoundAmplified_FirstRunCarriesNothingEvenWithCeilingOfOne(t *testing.T) {
	t.Parallel()

	got := PlanRound(0, 1)
	if got.CarryFindings || got.RaiseEffort {
		t.Fatalf("a first run carries no findings and raises no effort, got %+v", got)
	}
}

func TestPlanRoundAmplified_NegativeFailuresDoNotPanicOrCarry(t *testing.T) {
	t.Parallel()

	got := PlanRound(-3, 4)
	if got.CarryFindings || got.ChangeStrategy {
		t.Fatalf("negative failures must behave as a first run, got %+v", got)
	}
	if got.Plan() == "" {
		t.Fatal("Plan() must never be empty")
	}
}
