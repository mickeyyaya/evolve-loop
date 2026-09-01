package sandbox

import (
	"strings"
	"testing"
)

// TestConfinementSatisfied names the three-cell Specification both the bridge
// gate (sandboxRequiredButUnavailable) and preflight's host-capabilities check
// project from — extracted 2026-09-01 after the two sites diverged in the
// nested cell (preflight HALTed a launch the gate would have run) and the
// opt-out cell (the reverse).
func TestConfinementSatisfied(t *testing.T) {
	cases := []struct {
		name         string
		nested       bool
		mode         string
		wantOK       bool
		wantOptOut   bool
		reasonNeedle string
	}{
		{"nested satisfies with unverified-outer honesty", true, "", true, false, "UNVERIFIED"},
		{"nested wins even with mode off", true, "off", true, false, "outer session"},
		{"explicit opt-out satisfies loudly", false, "off", true, true, "UNCONFINED"},
		{"opt-out tolerates whitespace", false, " off ", true, true, "EVOLVE_SANDBOX=off"},
		{"unavailable fails closed", false, "", false, false, "required but unavailable"},
		{"unknown mode is not an opt-out", false, "auto", false, false, "required but unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, optOut, reason := ConfinementSatisfied(tc.nested, tc.mode)
			if ok != tc.wantOK || optOut != tc.wantOptOut {
				t.Fatalf("ConfinementSatisfied(%v,%q) = (%v,%v), want (%v,%v)", tc.nested, tc.mode, ok, optOut, tc.wantOK, tc.wantOptOut)
			}
			if !strings.Contains(reason, tc.reasonNeedle) {
				t.Fatalf("reason must carry %q; got %q", tc.reasonNeedle, reason)
			}
		})
	}
}
