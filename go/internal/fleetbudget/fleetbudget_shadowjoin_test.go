package fleetbudget

// fleetbudget_shadowjoin_test.go — S8 (token-telemetry) RED contract: the
// SHADOW quota↔tokens join. ShadowJoin pairs the binding quota window
// (quotastate.TightestRemaining — headroom in NATIVE units) with the measured
// median tokens/cycle (budgethistory.Throughput.MedianTokensPerCycle) into one
// loggable observation. It is groundwork for admission control and carries the
// slice's hard constraint: ZERO behavior change — Plan's decision must be
// byte-identical with the join present.
//
// Pinned API (the TDD contract Builder implements; no verbatim name in the plan
// doc, so it is fixed HERE and must not drift):
//
//	type QuotaJoin struct {
//	    Family               string
//	    RemainingFraction    float64
//	    MedianTokensPerCycle int64
//	    Reason               string
//	}
//	func ShadowJoin(states []quotastate.QuotaState, tp budgethistory.Throughput) (QuotaJoin, bool)
//
// ok==false means "no honest join available" — the same absent-evidence
// discipline as the rest of the package: never fabricate a fraction or a count.

import (
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

// tokenPace is `pace` plus a measured median token count — the S8 input.
func tokenPace(medianMS int64, medianTokens int64) budgethistory.Throughput {
	tp := pace(medianMS)
	tp.MedianTokensPerCycle = medianTokens
	return tp
}

// TestShadowJoin_PairsTightestRemainingWithMedianTokens is the primary S8
// fleetbudget assertion: the join reports the BINDING (tightest) window's family
// and remaining fraction alongside the measured tokens/cycle, with a legible
// reason carrying both numbers.
func TestShadowJoin_PairsTightestRemainingWithMedianTokens(t *testing.T) {
	// codex is roomier (0.80); claude binds at 0.25.
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
	// The reason is the operator-facing evidence the shadow soak accumulates: it
	// must carry BOTH joined numbers, not just restate the quota.
	if !strings.Contains(got.Reason, "1110") {
		t.Errorf("Reason must carry the median token count; got %q", got.Reason)
	}
	if !strings.Contains(got.Reason, "claude") {
		t.Errorf("Reason must name the binding family; got %q", got.Reason)
	}
}

// TestShadowJoin_NoQuotaSignalYieldsNoJoin is the negative axis: with no healthy
// probed window there is nothing to join against, so ok must be false and the
// value zero — an implementation that defaults RemainingFraction to 1.0 (the
// quotastate min seed) and reports a join fails here.
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

// TestShadowJoin_NoTokenEvidenceYieldsNoJoin is the second negative axis: quota
// is known but the token half is absent (legacy timing logs, fresh repo). Half a
// join is not a join.
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

// TestQuotaJoin_ZeroValueIsNotAJoin pins the third absence axis — no states at
// all (the nil slice a wave sees before any family has been probed) — and names
// QuotaJoin's zero value explicitly as the "not a join" sentinel callers must
// branch on ok for, never on a partially-populated struct.
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

// TestPlan_DecisionUnchangedByShadowJoin is the zero-behavior-change PIN — the
// inbox item's load-bearing acceptance criterion ("plan.Lanes decisions
// unchanged"). It fixes Plan's full output as a golden for two fixtures and
// re-asserts it after ShadowJoin has run against the same inputs: the join is
// observation only and must never feed the allocator.
func TestPlan_DecisionUnchangedByShadowJoin(t *testing.T) {
	states := []quotastate.QuotaState{probed("claude", 0.5, 20*time.Hour, fixedNow)}
	tp := tokenPace(3_600_000, 5_000_000) // 1 cyc/h, big token median
	cfg := Config{Count: 4, Floor: 1, CapacityCycles: 200, Safety: 0.6}

	before := Plan(states, tp, cfg, fixedNow)
	// Golden: identical to TestPlan_BudgetBranchSizesAffordableLanes, which knows
	// nothing about tokens. A token-influenced allocator breaks this.
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

	// Same pin on the floor-fallback path: a zero-token Throughput and a huge one
	// must decide identically.
	noTokens := Plan(nil, pace(3_600_000), cfg, fixedNow)
	withTokens := Plan(nil, tokenPace(3_600_000, 9_999_999), cfg, fixedNow)
	if noTokens != withTokens {
		t.Errorf("token median changed the floor-fallback plan: %+v vs %+v", noTokens, withTokens)
	}
}
