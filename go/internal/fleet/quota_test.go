package fleet

import (
	"bytes"
	"strings"
	"testing"
)

func TestQuotaAwareCount_BenchedFamilyShrinksAndWarns(t *testing.T) {
	var warn bytes.Buffer
	got := QuotaAwareCount(4, map[string]string{"codex": "rate_limit"}, 1, &warn)
	if got >= 4 {
		t.Errorf("QuotaAwareCount(4, one benched family) = %d, want < 4 (benched capacity must shrink the wave)", got)
	}
	if got < 1 {
		t.Errorf("QuotaAwareCount(4, one benched family) = %d, want >= 1 (min-1 floor)", got)
	}
	out := warn.String()
	if !strings.Contains(out, "codex") {
		t.Errorf("WARN must name the benched family %q; got: %q", "codex", out)
	}
	if !strings.Contains(out, "rate_limit") {
		t.Errorf("WARN must name the bench reason %q; got: %q", "rate_limit", out)
	}
}

func TestQuotaAwareCount_MinOneClamp(t *testing.T) {
	benched := map[string]string{
		"codex":  "rate_limit",
		"gemini": "quota_exhausted",
		"agy":    "rate_limit",
	}
	cases := []struct {
		name  string
		count int
	}{
		{"more-benches-than-count", 2},
		{"count-already-one", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var warn bytes.Buffer
			if got := QuotaAwareCount(tc.count, benched, 1, &warn); got != 1 {
				t.Errorf("QuotaAwareCount(%d, 3 benched families) = %d, want exactly 1 (min-1 clamp)", tc.count, got)
			}
		})
	}
}

func TestQuotaAwareCount_MinLanesFloorHoldsCapacity(t *testing.T) {
	var warn bytes.Buffer
	got := QuotaAwareCount(2, map[string]string{"codex": "rate_limit"}, 2, &warn)
	if got != 2 {
		t.Errorf("QuotaAwareCount(2, {codex}, minLanes=2) = %d, want 2 (floor holds capacity through the bench)", got)
	}
	out := warn.String()
	if !strings.Contains(out, "codex") || !strings.Contains(out, "rate_limit") {
		t.Errorf("WARN must still name the benched family + reason; got: %q", out)
	}
	if !strings.Contains(out, "min_lanes") {
		t.Errorf("WARN must report the floor held capacity (name fleet.min_lanes); got: %q", out)
	}
}

func TestQuotaAwareCount_MinLanesClampedToCount(t *testing.T) {
	var warn bytes.Buffer
	if got := QuotaAwareCount(2, map[string]string{"codex": "rate_limit"}, 5, &warn); got != 2 {
		t.Errorf("QuotaAwareCount(2, {codex}, minLanes=5) = %d, want 2 (floor clamped to count, not inflated)", got)
	}
}

func TestQuotaAwareCount_MinLanesFloorPartialShrink(t *testing.T) {
	var warn bytes.Buffer
	benched := map[string]string{"codex": "rate_limit", "gemini": "quota", "agy": "rate_limit"}
	if got := QuotaAwareCount(4, benched, 2, &warn); got != 2 {
		t.Errorf("QuotaAwareCount(4, 3 benched, minLanes=2) = %d, want 2 (shrinks to the floor, not below)", got)
	}
	out := warn.String()
	if !strings.Contains(out, "capacity held at 2 by fleet.min_lanes floor") {
		t.Errorf("a bench absorbed by the floor must WARN 'capacity held at 2 by fleet.min_lanes floor'; got:\n%s", out)
	}
}

func TestQuotaAwareCount_NoBenchesNoShrinkNoWarn(t *testing.T) {
	for name, benched := range map[string]map[string]string{"empty-map": {}, "nil-map": nil} {
		t.Run(name, func(t *testing.T) {
			var warn bytes.Buffer
			if got := QuotaAwareCount(4, benched, 1, &warn); got != 4 {
				t.Errorf("QuotaAwareCount(4, no benches) = %d, want 4 unchanged", got)
			}
			if warn.Len() != 0 {
				t.Errorf("QuotaAwareCount(4, no benches) wrote a WARN (%q), want silence", warn.String())
			}
		})
	}
}
