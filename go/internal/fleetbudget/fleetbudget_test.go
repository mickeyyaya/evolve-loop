package fleetbudget

import (
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

// probed builds a healthy (non-exhausted) probed QuotaState with a single
// window: usedFraction remaining, resetting resetIn from the fixed test now.
func probed(family string, remaining float64, resetIn time.Duration, now time.Time) quotastate.QuotaState {
	return quotastate.QuotaState{
		Family: family,
		Source: quotastate.SourceProbed,
		Buckets: []quotastate.Bucket{{
			Name:         "week",
			UsedFraction: 1 - remaining,
			ResetAt:      now.Add(resetIn),
		}},
	}
}

// pace builds a Throughput with a consistent median/rate pair (rate = 3.6e6/median).
func pace(medianMS int64) budgethistory.Throughput {
	tp := budgethistory.Throughput{SampleCount: 5, MedianCycleDurationMS: medianMS}
	if medianMS > 0 {
		tp.CyclesPerHour = 3_600_000.0 / float64(medianMS)
	}
	return tp
}

var fixedNow = time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)

func TestPlan_BudgetBranchSizesAffordableLanes(t *testing.T) {
	// rem 0.5, cap 200 → 100 budget-cycles; safety 0.6 → 60; pace 1 cyc/h, reset
	// 20h → 60/(1*20) = 3 affordable lanes. Count 4, Floor 1 → 3.
	states := []quotastate.QuotaState{probed("claude", 0.5, 20*time.Hour, fixedNow)}
	tp := pace(3_600_000) // 1 cyc/h
	cfg := Config{Count: 4, Floor: 1, CapacityCycles: 200, Safety: 0.6}

	got := Plan(states, tp, cfg, fixedNow)

	if got.DerivedFrom != FromBudget {
		t.Errorf("DerivedFrom = %q, want %q", got.DerivedFrom, FromBudget)
	}
	if got.Lanes != 3 {
		t.Errorf("Lanes = %d, want 3", got.Lanes)
	}
	if got.PaceDelay != 0 {
		t.Errorf("PaceDelay = %v, want 0 (lane count is the throttle)", got.PaceDelay)
	}
}

func TestPlan_BudgetBranchCapsAtCount(t *testing.T) {
	// Roomy: rem 0.9, cap 200, safety 0.9, reset 2h, pace 1 → 162/2 = 81 ≫ Count.
	states := []quotastate.QuotaState{probed("claude", 0.9, 2*time.Hour, fixedNow)}
	cfg := Config{Count: 4, Floor: 1, CapacityCycles: 200, Safety: 0.9}

	got := Plan(states, pace(3_600_000), cfg, fixedNow)

	if got.DerivedFrom != FromBudget || got.Lanes != 4 {
		t.Errorf("got {Lanes:%d From:%q}, want {4 budget} (capped at Count)", got.Lanes, got.DerivedFrom)
	}
}

func TestPlan_FloorForcedOverspendSetsPaceDelay(t *testing.T) {
	// Tight: rem 0.1, cap 100 → 10 budget-cycles; safety 0.5 → 5; pace 1, reset
	// 20h → 5/20 = 0.25 affordable < Floor 2. Lanes floored to 2; the extra lanes
	// are paced by a per-cycle idle gap: dutyGap = 2/0.25 - 1 = 7 → 7 × 1h median.
	states := []quotastate.QuotaState{probed("claude", 0.1, 20*time.Hour, fixedNow)}
	cfg := Config{Count: 4, Floor: 2, CapacityCycles: 100, Safety: 0.5}

	got := Plan(states, pace(3_600_000), cfg, fixedNow)

	if got.DerivedFrom != FromBudget {
		t.Errorf("DerivedFrom = %q, want budget", got.DerivedFrom)
	}
	if got.Lanes != 2 {
		t.Errorf("Lanes = %d, want 2 (floored to min)", got.Lanes)
	}
	if got.PaceDelay != 7*time.Hour {
		t.Errorf("PaceDelay = %v, want 7h (duty-cycle pacing of floor-forced lanes)", got.PaceDelay)
	}
}

func TestPlan_TightestWindowBindsAcrossFamilies(t *testing.T) {
	// claude 34% remaining vs codex 90% — the tighter (claude) must bind. With
	// cap 200, safety 0.6, pace 1, reset 10h: 0.34 → 40.8/10 = 4.08 → 4 lanes;
	// 0.90 would give 10.8 → capped 6. Asserting 4 proves the min binds.
	states := []quotastate.QuotaState{
		probed("codex", 0.90, 10*time.Hour, fixedNow),
		probed("claude", 0.34, 10*time.Hour, fixedNow),
	}
	cfg := Config{Count: 6, Floor: 1, CapacityCycles: 200, Safety: 0.6}

	got := Plan(states, pace(3_600_000), cfg, fixedNow)

	if got.Lanes != 4 {
		t.Errorf("Lanes = %d, want 4 (tightest 34%% window binds, not 90%%)", got.Lanes)
	}
}

func TestPlan_ExhaustedFamilyDropped(t *testing.T) {
	// The only family is exhausted → no healthy quota signal → floor fallback at
	// Count (budget layer doesn't handle exhaustion; the bench-shrink composes
	// downstream).
	exhausted := quotastate.QuotaState{
		Family:    "claude",
		Source:    quotastate.SourceProbed,
		Exhausted: true,
		Buckets:   []quotastate.Bucket{{Name: "week", UsedFraction: 1.0, ResetAt: fixedNow.Add(3 * time.Hour)}},
	}
	cfg := Config{Count: 3, Floor: 1, CapacityCycles: 200, Safety: 0.6}

	got := Plan([]quotastate.QuotaState{exhausted}, pace(3_600_000), cfg, fixedNow)

	if got.DerivedFrom != FromFloor || got.Lanes != 3 {
		t.Errorf("got {Lanes:%d From:%q}, want {3 floor-fallback}", got.Lanes, got.DerivedFrom)
	}
}

func TestPlan_ResetPaceWhenNoBudgetConfig(t *testing.T) {
	// Healthy family with a reset horizon, but NO fleet.budget config (capacity 0)
	// → cannot size a budget → run full width, DerivedFrom=reset-pace. This is the
	// shadow default: absent budget config never restricts below Count.
	states := []quotastate.QuotaState{probed("claude", 0.5, 5*time.Hour, fixedNow)}
	cfg := Config{Count: 3, Floor: 1} // CapacityCycles/Safety unset

	got := Plan(states, pace(3_600_000), cfg, fixedNow)

	if got.DerivedFrom != FromResetPace {
		t.Errorf("DerivedFrom = %q, want reset-pace", got.DerivedFrom)
	}
	if got.Lanes != 3 {
		t.Errorf("Lanes = %d, want 3 (full width — no budget config)", got.Lanes)
	}
}

func TestPlan_FloorFallbackWhenAllUnknown(t *testing.T) {
	// No probed numbers anywhere (unknown source) and an empty slice both fall
	// back to the configured Count (respecting, never dropping below, the floor).
	cfg := Config{Count: 2, Floor: 1, CapacityCycles: 200, Safety: 0.6}
	unknown := []quotastate.QuotaState{{Family: "ollama", Source: quotastate.SourceUnknown}}

	for _, states := range [][]quotastate.QuotaState{unknown, nil} {
		got := Plan(states, pace(3_600_000), cfg, fixedNow)
		if got.DerivedFrom != FromFloor || got.Lanes != 2 {
			t.Errorf("states=%v: got {Lanes:%d From:%q}, want {2 floor-fallback}", states, got.Lanes, got.DerivedFrom)
		}
	}
}

func TestPlan_ShadowByteIdenticalWithoutBudgetConfig(t *testing.T) {
	// The core shadow guarantee: with no fleet.budget config, Plan NEVER sizes
	// below Count no matter how tight the quota — behaviour is byte-identical to
	// the pre-budget world (Count, respecting the floor).
	tight := []quotastate.QuotaState{probed("claude", 0.02, 30*time.Hour, fixedNow)}
	cfg := Config{Count: 5, Floor: 2} // no CapacityCycles/Safety

	got := Plan(tight, pace(3_600_000), cfg, fixedNow)

	if got.Lanes != 5 {
		t.Errorf("Lanes = %d, want 5 (shadow: no budget config must not restrict)", got.Lanes)
	}
}

func TestPlan_PaceDelayCappedAtResetHorizon(t *testing.T) {
	// rem 0.01, cap 100 → 1 budget-cycle; safety 0.5 → 0.5; pace 1, reset 2h →
	// 0.25 affordable < Floor 2 → dutyGap 7 → 7h uncapped, but the reset is only
	// 2h away: never idle past reset, so PaceDelay caps at dt = 2h.
	states := []quotastate.QuotaState{probed("claude", 0.01, 2*time.Hour, fixedNow)}
	cfg := Config{Count: 4, Floor: 2, CapacityCycles: 100, Safety: 0.5}

	got := Plan(states, pace(3_600_000), cfg, fixedNow)

	if got.Lanes != 2 {
		t.Errorf("Lanes = %d, want 2", got.Lanes)
	}
	if got.PaceDelay != 2*time.Hour {
		t.Errorf("PaceDelay = %v, want 2h (capped at reset horizon)", got.PaceDelay)
	}
}

func TestPlan_ResetPassedShowsNoNegativeDuration(t *testing.T) {
	// Budget tunables present, but the measured window's reset is in the past
	// (dt ≤ 0) → the budget branch is skipped and reset-pace must render "reset
	// has passed", never a negative duration.
	states := []quotastate.QuotaState{probed("claude", 0.5, -30*time.Minute, fixedNow)}
	cfg := Config{Count: 3, Floor: 1, CapacityCycles: 200, Safety: 0.6}

	got := Plan(states, pace(3_600_000), cfg, fixedNow)

	if got.DerivedFrom != FromResetPace || got.Lanes != 3 {
		t.Errorf("got {Lanes:%d From:%q}, want {3 reset-pace}", got.Lanes, got.DerivedFrom)
	}
	if !strings.Contains(got.Reason, "reset has passed") {
		t.Errorf("Reason = %q, want it to contain \"reset has passed\"", got.Reason)
	}
	if strings.Contains(got.Reason, "-") {
		t.Errorf("Reason = %q, must not contain a negative duration", got.Reason)
	}
}

func TestPlan_HugeCapacityDoesNotOverflow(t *testing.T) {
	// A pathological CapacityCycles must not overflow the int cast: affordable is
	// capped at Count before the conversion, so the plan is Count, not garbage.
	states := []quotastate.QuotaState{probed("claude", 0.5, 2*time.Hour, fixedNow)}
	cfg := Config{Count: 4, Floor: 1, CapacityCycles: 1e19, Safety: 1.0}

	got := Plan(states, pace(3_600_000), cfg, fixedNow)

	if got.DerivedFrom != FromBudget || got.Lanes != 4 {
		t.Errorf("got {Lanes:%d From:%q}, want {4 budget} (capped, no overflow)", got.Lanes, got.DerivedFrom)
	}
}
