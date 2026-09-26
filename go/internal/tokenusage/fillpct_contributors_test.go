package tokenusage

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

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

func TestFillWarnWithContributors_EmptyContributorsOmitsBreakdown(t *testing.T) {
	got := FillWarnWithContributors("audit", 75.0, 60, cyclestate.TokenUsage{})
	base := FillWarn("audit", 75.0, 60)
	if got != base {
		t.Errorf("FillWarnWithContributors with zero-value contributors = %q, want the same message FillWarn produces (%q) — nothing to attribute means no breakdown, not a malformed one", got, base)
	}
}
