package main

// cmd_loop_wave_budget_test.go — Q4 wiring: quotaAwareWaveConfig composes the
// existing bench-shrink (availability envelope) with fleetbudget.Plan (size
// against measured quota headroom). Contracts pinned here:
//   - Budget==nil ⇒ byte-identical to the pre-Q4 bench-only behavior (no probe,
//     no resize, no pace) — the shadow-safe default.
//   - Stage=="shadow" ⇒ compute + LOG the decision, but HOLD the lane count.
//   - Stage=="enforce" ⇒ apply plan.Lanes and surface plan.PaceDelay.
// An empty projectRoot temp dir has no benches, so the bench-shrink is a no-op
// and each case isolates the budget branch.

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

// tightQuota is a probed family with only 10% of a 1-hour reset window left —
// enough headroom pressure that a modest budget sizes below a 3-lane wave.
func tightQuota(now time.Time) []quotastate.QuotaState {
	return []quotastate.QuotaState{{
		Family:     "claude",
		Source:     quotastate.SourceProbed,
		Buckets:    []quotastate.Bucket{{Name: "week", UsedFraction: 0.9, ResetAt: now.Add(time.Hour)}},
		ObservedAt: now,
	}}
}

// fastPace: 10 cycles/hour/lane, 6-minute median — the sizing denominator.
func fastPace() budgethistory.Throughput {
	return budgethistory.Throughput{SampleCount: 5, MedianCycleDurationMS: 360000, CyclesPerHour: 10}
}

func TestQuotaAwareWaveConfig_BudgetNilByteIdentical(t *testing.T) {
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	fc := policy.FleetConfig{Count: 3, Concurrency: 3, MinLanes: 1, Budget: nil}
	var buf bytes.Buffer

	got, pace := quotaAwareWaveConfig(fc, t.TempDir(), &buf, tightQuota(now), fastPace(), now)

	if got.Count != 3 {
		t.Errorf("Count = %d, want 3 (nil budget ⇒ no resize)", got.Count)
	}
	if pace != 0 {
		t.Errorf("PaceDelay = %v, want 0 (nil budget)", pace)
	}
	if strings.Contains(buf.String(), "[budget]") {
		t.Errorf("nil budget must not emit a [budget] line; got:\n%s", buf.String())
	}
}

func TestQuotaAwareWaveConfig_ShadowLogsButHolds(t *testing.T) {
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	fc := policy.FleetConfig{Count: 3, Concurrency: 3, MinLanes: 1,
		Budget: &policy.FleetBudgetConfig{Stage: "shadow", CapacityCycles: 10, Safety: 0.5, HistoryWindow: 10}}
	var buf bytes.Buffer

	got, pace := quotaAwareWaveConfig(fc, t.TempDir(), &buf, tightQuota(now), fastPace(), now)

	if got.Count != 3 {
		t.Errorf("Count = %d, want 3 HELD (shadow never resizes)", got.Count)
	}
	if pace != 0 {
		t.Errorf("PaceDelay = %v, want 0 in shadow", pace)
	}
	log := buf.String()
	if !strings.Contains(log, "[budget]") || !strings.Contains(strings.ToLower(log), "shadow") {
		t.Errorf("shadow must LOG the decision; got:\n%s", log)
	}
}

func TestQuotaAwareWaveConfig_EnforceResizesDown(t *testing.T) {
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	fc := policy.FleetConfig{Count: 3, Concurrency: 3, MinLanes: 1,
		Budget: &policy.FleetBudgetConfig{Stage: "enforce", CapacityCycles: 10, Safety: 0.5, HistoryWindow: 10}}
	var buf bytes.Buffer

	got, pace := quotaAwareWaveConfig(fc, t.TempDir(), &buf, tightQuota(now), fastPace(), now)

	// budgetCycles = 0.1*10 = 1; affordable = 1*0.5/(10*1h) = 0.05 → clamp up to
	// floor 1. affordable (0.05) < floor (1), so even the single floor lane is
	// floor-forced: the near-exhausted budget throttles it, PaceDelay capping at
	// the 1h reset horizon (run ~a cycle, then wait for the budget to refresh).
	if got.Count != 1 {
		t.Errorf("Count = %d, want 1 (enforce sizes to the affordable floor)", got.Count)
	}
	if pace != time.Hour {
		t.Errorf("PaceDelay = %v, want 1h (floor-forced pace capped at the reset horizon)", pace)
	}
	if !strings.Contains(strings.ToLower(buf.String()), "enforce") {
		t.Errorf("enforce must LOG the decision; got:\n%s", buf.String())
	}
}

func TestQuotaAwareWaveConfig_EnforceFloorForcedPaces(t *testing.T) {
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	// rem 0.2, capacity 10 ⇒ 2 budget cycles; safety 0.5, 2 cyc/h, dt 1h ⇒
	// affordable = 2*0.5/(2*1) = 0.5 < floor 2 ⇒ Lanes clamps up to floor 2 and
	// the surplus is paced: PaceDelay = 360000ms*(2/0.5 - 1) = 18m (< 1h cap).
	fc := policy.FleetConfig{Count: 4, Concurrency: 4, MinLanes: 2,
		Budget: &policy.FleetBudgetConfig{Stage: "enforce", CapacityCycles: 10, Safety: 0.5, HistoryWindow: 10}}
	states := []quotastate.QuotaState{{
		Family:     "claude",
		Source:     quotastate.SourceProbed,
		Buckets:    []quotastate.Bucket{{Name: "week", UsedFraction: 0.8, ResetAt: now.Add(time.Hour)}},
		ObservedAt: now,
	}}
	tp := budgethistory.Throughput{SampleCount: 5, MedianCycleDurationMS: 360000, CyclesPerHour: 2}
	var buf bytes.Buffer

	got, pace := quotaAwareWaveConfig(fc, t.TempDir(), &buf, states, tp, now)

	if got.Count != 2 {
		t.Errorf("Count = %d, want 2 (floor-forced)", got.Count)
	}
	if pace <= 0 {
		t.Errorf("PaceDelay = %v, want > 0 (floor forced more lanes than affordable)", pace)
	}
}
