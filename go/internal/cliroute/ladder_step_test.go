package cliroute

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestClampStep_AnUnknownBaseOrBoundSetsNoFloorAndNoCeiling(t *testing.T) {
	tiers := policy.TierNames()
	for _, tc := range []struct{ rung, bound, base string }{
		{"top", "", ""},
		{"fast", "deep", "claude-opus-5-5"},
	} {
		got, note, err := clampStep("tier", tc.rung, tiers, tc.bound, tc.base)
		if err != nil || got != tc.rung || note != "" {
			t.Errorf("clampStep(%+v) = (%q, %q, %v), want the rung %q kept with no note", tc, got, note, err, tc.rung)
		}
	}
}
