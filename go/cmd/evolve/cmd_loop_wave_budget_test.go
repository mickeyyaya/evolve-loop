package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

func tightQuota(now time.Time) []quotastate.QuotaState {
	return []quotastate.QuotaState{{
		Family:     "claude",
		Source:     quotastate.SourceProbed,
		Buckets:    []quotastate.Bucket{{Name: "week", UsedFraction: 0.9, ResetAt: now.Add(time.Hour)}},
		ObservedAt: now,
	}}
}

func fastPace() budgethistory.Throughput {
	return budgethistory.Throughput{SampleCount: 5, MedianCycleDurationMS: 360000, CyclesPerHour: 10}
}

func TestQuotaAwareWaveConfig_BudgetNilByteIdentical(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	fc := policy.FleetConfig{Count: 3, Concurrency: 3, MinLanes: 1,
		Budget: &policy.FleetBudgetConfig{Stage: "enforce", CapacityCycles: 10, Safety: 0.5, HistoryWindow: 10}}
	var buf bytes.Buffer

	got, pace := quotaAwareWaveConfig(fc, t.TempDir(), &buf, tightQuota(now), fastPace(), now)

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
	t.Parallel()
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
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
