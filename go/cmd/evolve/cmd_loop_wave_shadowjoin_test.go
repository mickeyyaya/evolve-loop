package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// tokenFastPace is fastPace() plus a measured median token count (S8's new
// Throughput field) — the token half of the join.
func tokenFastPace(medianTokens int64) budgethistory.Throughput {
	tp := fastPace()
	tp.MedianTokensPerCycle = medianTokens
	return tp
}

func TestQuotaAwareWaveConfig_LogsShadowQuotaTokenJoin(t *testing.T) {
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	fc := policy.FleetConfig{Count: 3, Concurrency: 3, MinLanes: 1,
		Budget: &policy.FleetBudgetConfig{Stage: "shadow", CapacityCycles: 10, Safety: 0.5, HistoryWindow: 10}}
	var buf bytes.Buffer

	got, pace := quotaAwareWaveConfig(fc, t.TempDir(), &buf, tightQuota(now), tokenFastPace(1110), now)

	log := buf.String()
	if !strings.Contains(log, "[budget] shadow-join") {
		t.Errorf("composed wave path must emit the quota↔tokens shadow join; got:\n%s", log)
	}
	if !strings.Contains(log, "1110") {
		t.Errorf("shadow-join line must carry the median tokens/cycle; got:\n%s", log)
	}
	if !strings.Contains(log, "claude") {
		t.Errorf("shadow-join line must name the binding quota family; got:\n%s", log)
	}
	if got.Count != 3 {
		t.Errorf("Count = %d, want 3 HELD (the join must not resize)", got.Count)
	}
	if pace != 0 {
		t.Errorf("PaceDelay = %v, want 0 (the join must not pace)", pace)
	}
}

func TestQuotaAwareWaveConfig_EnforceStillJoinsAndDecidesUnchanged(t *testing.T) {
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	fc := policy.FleetConfig{Count: 3, Concurrency: 3, MinLanes: 1,
		Budget: &policy.FleetBudgetConfig{Stage: "enforce", CapacityCycles: 10, Safety: 0.5, HistoryWindow: 10}}
	var buf bytes.Buffer

	got, pace := quotaAwareWaveConfig(fc, t.TempDir(), &buf, tightQuota(now), tokenFastPace(1110), now)

	if !strings.Contains(buf.String(), "[budget] shadow-join") {
		t.Errorf("enforce stage must also emit the shadow join (observation, not a decision); got:\n%s", buf.String())
	}
	// Golden from TestQuotaAwareWaveConfig_EnforceResizesDown — unchanged by S8.
	if got.Count != 1 {
		t.Errorf("Count = %d, want 1 (enforce sizing unchanged by the join)", got.Count)
	}
	if pace != time.Hour {
		t.Errorf("PaceDelay = %v, want 1h (pacing unchanged by the join)", pace)
	}
}

func TestQuotaAwareWaveConfig_NoTokenEvidenceEmitsNoJoin(t *testing.T) {
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	fc := policy.FleetConfig{Count: 3, Concurrency: 3, MinLanes: 1,
		Budget: &policy.FleetBudgetConfig{Stage: "shadow", CapacityCycles: 10, Safety: 0.5, HistoryWindow: 10}}
	var buf bytes.Buffer

	// fastPace() carries no MedianTokensPerCycle — half a join is not a join.
	quotaAwareWaveConfig(fc, t.TempDir(), &buf, tightQuota(now), fastPace(), now)

	log := buf.String()
	if strings.Contains(log, "shadow-join") {
		t.Errorf("no token evidence must yield NO join line (never fabricate); got:\n%s", log)
	}
	if !strings.Contains(log, "[budget] shadow:") {
		t.Errorf("the existing sizing log must be unaffected; got:\n%s", log)
	}
}

func TestQuotaAwareWaveConfig_NilBudgetEmitsNoJoin(t *testing.T) {
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	fc := policy.FleetConfig{Count: 3, Concurrency: 3, MinLanes: 1, Budget: nil}
	var buf bytes.Buffer

	got, pace := quotaAwareWaveConfig(fc, t.TempDir(), &buf, tightQuota(now), tokenFastPace(1110), now)

	if buf.Len() != 0 {
		t.Errorf("nil budget must stay silent (no probe, no join); got:\n%s", buf.String())
	}
	if got.Count != 3 || pace != 0 {
		t.Errorf("nil budget: Count=%d pace=%v, want 3 / 0", got.Count, pace)
	}
}
