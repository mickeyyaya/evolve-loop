package tokenusage

// fillpct_contributors_test.go — RED contract for cycle-1482 task
// `context-fill-warning-attribution` (scout-report.md Task 2).
//
// FillWarn today reports only a phase and a percentage — no contributor
// breakdown at all. The cycle-1458 audit (M1) flagged the failure mode a
// contributor breakdown WOULD fall into if it were ever added carelessly: a
// breakdown built off the whole-launch SUMMED usage would disagree with a
// percentage that fillpct.go's own windowOccupancy derives from a single PEAK
// turn (cycle-1455) — an operator would see a 75% reading annotated with
// components that total 1500% of the window.
//
// FillWarnWithContributors is undefined today, so this file fails to COMPILE
// until Builder adds it — the RED signal. The contract: whatever basis the
// caller hands in as `contributors` is what the message must show, verbatim
// and attributably, alongside the SAME phase/threshold/sentinel semantics
// FillWarn already promises. Basis SELECTION (peak turn vs whole-launch total)
// is the caller's job — proven by the internal/bridge wiring test in the sister
// task's RED contract — not this function's.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// TestFillWarnWithContributors_IncludesGivenBasis is the positive half: a
// warning above threshold carries both the phase and every contributor the
// caller handed in, using the SAME numbers — no re-derivation, no silent drop.
func TestFillWarnWithContributors_IncludesGivenBasis(t *testing.T) {
	contributors := cyclestate.TokenUsage{Input: 200, CacheRead: 6_800, CacheWrite: 0}
	got := FillWarnWithContributors("build", 70.0, 60, contributors)
	if got == "" {
		t.Fatalf("FillWarnWithContributors(\"build\", 70.0, 60, %+v) = \"\", want a warning — 70%% is above the 60%% threshold", contributors)
	}
	if !strings.Contains(got, "build") {
		t.Errorf("warn %q does not name the phase — an unattributed fill WARN cannot be acted on", got)
	}
	if !strings.Contains(got, "input=200") || !strings.Contains(got, "cache_read=6800") {
		t.Errorf("warn %q does not carry the given contributor basis (input=200, cache_read=6800)", got)
	}
}

// TestFillWarnWithContributors_SilentBelowThresholdOrSentinel is the negative
// half: adding a contributor breakdown must not weaken FillWarn's existing
// strict-above-threshold and sentinel-never-warns guarantees. A caller handing
// in a huge contributor basis alongside a silent percentage must still get "".
func TestFillWarnWithContributors_SilentBelowThresholdOrSentinel(t *testing.T) {
	big := cyclestate.TokenUsage{Input: 500_000, CacheRead: 500_000}
	cases := []struct {
		name string
		pct  float64
	}{
		{"below threshold", 59},
		{"exactly at threshold", 60},
		{"unmeasured sentinel", FillPctUnmeasured},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FillWarnWithContributors("build", c.pct, 60, big); got != "" {
				t.Errorf("FillWarnWithContributors(\"build\", %v, 60, big) = %q, want silence", c.pct, got)
			}
		})
	}
}

// TestFillWarnWithContributors_OverHundredPercentStaysLegible is the anti-
// regression case: a genuine overrun must still report its real, unclamped
// percentage (fillpct.go's documented over-100 promise) once a contributor
// breakdown is attached to the message.
func TestFillWarnWithContributors_OverHundredPercentStaysLegible(t *testing.T) {
	contributors := cyclestate.TokenUsage{Input: 40_000, CacheRead: 200_000}
	got := FillWarnWithContributors("build", 120.0, 60, contributors)
	if !strings.Contains(got, "120.0") {
		t.Errorf("warn %q does not carry the real 120.0%% reading — a contributor breakdown must not clamp or round away an honest overrun", got)
	}
	if !strings.Contains(got, "input=40000") || !strings.Contains(got, "cache_read=200000") {
		t.Errorf("warn %q does not carry the given contributor basis on an over-100%% reading", got)
	}
}

// TestFillWarnWithContributors_EmptyContributorsOmitsBreakdown is the edge
// case for tiers that never report a per-turn breakdown at all (events/
// scrollback with no data): a caller with nothing to attribute gets the same
// message FillWarn already produces, not a broken or empty parenthetical.
func TestFillWarnWithContributors_EmptyContributorsOmitsBreakdown(t *testing.T) {
	got := FillWarnWithContributors("audit", 75.0, 60, cyclestate.TokenUsage{})
	base := FillWarn("audit", 75.0, 60)
	if got != base {
		t.Errorf("FillWarnWithContributors with zero-value contributors = %q, want the same message FillWarn produces (%q) — nothing to attribute means no breakdown, not a malformed one", got, base)
	}
}
