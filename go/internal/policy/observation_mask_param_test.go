package policy_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

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
