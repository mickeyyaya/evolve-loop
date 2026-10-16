package fleetbudget

import (
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

func tokenPace(medianMS int64, medianTokens int64) budgethistory.Throughput {
	tp := pace(medianMS)
	tp.MedianTokensPerCycle = medianTokens
	return tp
}

func TestShadowJoin_PairsTightestRemainingWithMedianTokens(t *testing.T) {
	states := []quotastate.QuotaState{
		probed("codex", 0.80, 6*time.Hour, fixedNow),
		probed("claude", 0.25, 6*time.Hour, fixedNow),
	}

	got, ok := ShadowJoin(states, tokenPace(600_000, 1110))

	if !ok {
		t.Fatalf("ShadowJoin ok = false, want true (probed quota + measured tokens present)")
	}
	if got.Family != "claude" {
		t.Errorf("Family = %q, want \"claude\" (the tightest window binds, not the first)", got.Family)
	}
	if got.RemainingFraction < 0.2499 || got.RemainingFraction > 0.2501 {
		t.Errorf("RemainingFraction = %v, want ~0.25 (tightest, per quotastate.TightestRemaining)", got.RemainingFraction)
	}
	if got.MedianTokensPerCycle != 1110 {
		t.Errorf("MedianTokensPerCycle = %d, want 1110", got.MedianTokensPerCycle)
	}
	if !strings.Contains(got.Reason, "1110") {
		t.Errorf("Reason must carry the median token count; got %q", got.Reason)
	}
	if !strings.Contains(got.Reason, "claude") {
		t.Errorf("Reason must name the binding family; got %q", got.Reason)
	}
}

func TestShadowJoin_NoQuotaSignalYieldsNoJoin(t *testing.T) {
	unknown := quotastate.QuotaState{Family: "claude", Source: quotastate.SourceUnknown}
	exhausted := probed("codex", 0.0, time.Hour, fixedNow)
	exhausted.Exhausted = true

	got, ok := ShadowJoin([]quotastate.QuotaState{unknown, exhausted}, tokenPace(600_000, 1110))

	if ok {
		t.Errorf("ShadowJoin ok = true, want false (no healthy probed window)")
	}
	if got.RemainingFraction != 0 || got.Family != "" || got.MedianTokensPerCycle != 0 {
		t.Errorf("ShadowJoin(no signal) = %+v, want the zero value (never fabricate)", got)
	}
}

func TestShadowJoin_NoTokenEvidenceYieldsNoJoin(t *testing.T) {
	states := []quotastate.QuotaState{probed("claude", 0.25, 6*time.Hour, fixedNow)}

	got, ok := ShadowJoin(states, tokenPace(600_000, 0))

	if ok {
		t.Errorf("ShadowJoin ok = true, want false (MedianTokensPerCycle unknown)")
	}
	if got.Reason != "" {
		t.Errorf("Reason = %q, want empty when no join is available", got.Reason)
	}
}

func TestQuotaJoin_ZeroValueIsNotAJoin(t *testing.T) {
	var zero QuotaJoin

	got, ok := ShadowJoin(nil, tokenPace(600_000, 1110))

	if ok {
		t.Errorf("ShadowJoin(nil states) ok = true, want false")
	}
	if got != zero {
		t.Errorf("ShadowJoin(nil states) = %+v, want the zero QuotaJoin %+v", got, zero)
	}
}

func TestPlan_DecisionUnchangedByShadowJoin(t *testing.T) {
	states := []quotastate.QuotaState{probed("claude", 0.5, 20*time.Hour, fixedNow)}
	tp := tokenPace(3_600_000, 5_000_000)
	cfg := Config{Count: 4, Floor: 1, CapacityCycles: 200, Safety: 0.6}

	before := Plan(states, tp, cfg, fixedNow)
	if before.Lanes != 3 || before.DerivedFrom != FromBudget || before.PaceDelay != 0 {
		t.Fatalf("Plan = %+v, want Lanes=3 DerivedFrom=%q PaceDelay=0 (golden, token-blind)", before, FromBudget)
	}

	if _, ok := ShadowJoin(states, tp); !ok {
		t.Fatalf("ShadowJoin ok = false, want true for this fixture")
	}

	after := Plan(states, tp, cfg, fixedNow)
	if after != before {
		t.Errorf("Plan after ShadowJoin = %+v, want byte-identical to %+v", after, before)
	}

	noTokens := Plan(nil, pace(3_600_000), cfg, fixedNow)
	withTokens := Plan(nil, tokenPace(3_600_000, 9_999_999), cfg, fixedNow)
	if noTokens != withTokens {
		t.Errorf("token median changed the floor-fallback plan: %+v vs %+v", noTokens, withTokens)
	}
}
