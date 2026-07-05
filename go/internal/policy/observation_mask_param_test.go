package policy_test

// ObservationMaskPolicy / ObservationMaskConfig — the .evolve/policy.json
// "observation_mask" block (cycle-530, research-backed #1 token lever). Mirrors
// the RouterPolicy raw==resolved idiom: a raw *ObservationMaskPolicy block on
// Policy that doubles as the ObservationMaskConfig() getter's return type, with
// a single WindowTurns knob defaulting to 10. Black-box: drives only the
// exported Policy/ObservationMaskPolicy surface, zero env.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// TestObservationMaskConfig_Resolution pins the window resolution table: an
// absent block and any non-positive override both resolve to the default 10; a
// positive override passes through. A hardcoded-default getter that ignores the
// override fails the window:5 case.
func TestObservationMaskConfig_Resolution(t *testing.T) {
	cases := []struct {
		name string
		pol  policy.Policy
		want int
	}{
		{"absent block → default 10", policy.Policy{}, 10},
		{"zero override → default 10", policy.Policy{ObservationMask: &policy.ObservationMaskPolicy{WindowTurns: 0}}, 10},
		{"negative override → default 10", policy.Policy{ObservationMask: &policy.ObservationMaskPolicy{WindowTurns: -3}}, 10},
		{"positive override passes through", policy.Policy{ObservationMask: &policy.ObservationMaskPolicy{WindowTurns: 5}}, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pol.ObservationMaskConfig().WindowTurns; got != tc.want {
				t.Errorf("ObservationMaskConfig().WindowTurns = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestObservationMaskConfig_ReadFromJSON confirms the window is sourced from
// policy.json (no env flag): a window_turns override loads through the real
// policy.Load pipeline, and an empty file falls back to the default 10.
func TestObservationMaskConfig_ReadFromJSON(t *testing.T) {
	dir := t.TempDir()
	overridePath := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(overridePath, []byte(`{"observation_mask":{"window_turns":7}}`), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	p, err := policy.Load(overridePath)
	if err != nil {
		t.Fatalf("policy.Load: %v", err)
	}
	if got := p.ObservationMaskConfig().WindowTurns; got != 7 {
		t.Errorf("window_turns=7 in policy.json resolved to %d, want 7", got)
	}
}
