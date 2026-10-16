package fleetbudget

import (
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

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

func pace(medianMS int64) budgethistory.Throughput {
	tp := budgethistory.Throughput{SampleCount: 5, MedianCycleDurationMS: medianMS}
	if medianMS > 0 {
		tp.CyclesPerHour = 3_600_000.0 / float64(medianMS)
	}
	return tp
}

var fixedNow = time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)

func TestPlan_BudgetBranchSizesAffordableLanes(t *testing.T) {
	states := []quotastate.QuotaState{probed("claude", 0.5, 20*time.Hour, fixedNow)}
	tp := pace(3_600_000)
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
	states := []quotastate.QuotaState{probed("claude", 0.9, 2*time.Hour, fixedNow)}
	cfg := Config{Count: 4, Floor: 1, CapacityCycles: 200, Safety: 0.9}

	got := Plan(states, pace(3_600_000), cfg, fixedNow)

	if got.DerivedFrom != FromBudget || got.Lanes != 4 {
		t.Errorf("got {Lanes:%d From:%q}, want {4 budget} (capped at Count)", got.Lanes, got.DerivedFrom)
	}
}

func TestPlan_FloorForcedOverspendSetsPaceDelay(t *testing.T) {
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
	states := []quotastate.QuotaState{probed("claude", 0.5, 5*time.Hour, fixedNow)}
	cfg := Config{Count: 3, Floor: 1}

	got := Plan(states, pace(3_600_000), cfg, fixedNow)

	if got.DerivedFrom != FromResetPace {
		t.Errorf("DerivedFrom = %q, want reset-pace", got.DerivedFrom)
	}
	if got.Lanes != 3 {
		t.Errorf("Lanes = %d, want 3 (full width — no budget config)", got.Lanes)
	}
}

func TestPlan_FloorFallbackWhenAllUnknown(t *testing.T) {
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
	tight := []quotastate.QuotaState{probed("claude", 0.02, 30*time.Hour, fixedNow)}
	cfg := Config{Count: 5, Floor: 2}

	got := Plan(tight, pace(3_600_000), cfg, fixedNow)

	if got.Lanes != 5 {
		t.Errorf("Lanes = %d, want 5 (shadow: no budget config must not restrict)", got.Lanes)
	}
}

func TestPlan_PaceDelayCappedAtResetHorizon(t *testing.T) {
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
	states := []quotastate.QuotaState{probed("claude", 0.5, 2*time.Hour, fixedNow)}
	cfg := Config{Count: 4, Floor: 1, CapacityCycles: 1e19, Safety: 1.0}

	got := Plan(states, pace(3_600_000), cfg, fixedNow)

	if got.DerivedFrom != FromBudget || got.Lanes != 4 {
		t.Errorf("got {Lanes:%d From:%q}, want {4 budget} (capped, no overflow)", got.Lanes, got.DerivedFrom)
	}
}
