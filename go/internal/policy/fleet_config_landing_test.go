package policy_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestFleetConfig_LandingClosedVocab(t *testing.T) {
	cases := []struct {
		raw, want   string
		wantWarning bool
	}{
		{"", "per-lane", false},
		{"per-lane", "per-lane", false},
		{"prefix-queue", "prefix-queue", false},
		{"group-commit", "per-lane", true},
	}
	for _, tc := range cases {
		got := policy.Policy{Fleet: &policy.FleetPolicy{Landing: tc.raw}}.FleetConfig()
		if got.Landing != tc.want {
			t.Errorf("Landing(%q) = %q, want %q", tc.raw, got.Landing, tc.want)
		}
		warned := len(got.Warnings) == 1 && strings.Contains(got.Warnings[0], tc.raw) && strings.Contains(got.Warnings[0], "per-lane")
		if warned != tc.wantWarning {
			t.Errorf("Landing(%q) warnings = %v, want a warning: %v", tc.raw, got.Warnings, tc.wantWarning)
		}
	}
}

func TestFleetConfig_StarvationKeysOverrideAndTheWeightHasAFloor(t *testing.T) {
	got := policy.Policy{Fleet: &policy.FleetPolicy{StarvationK: 5, StarvationWeight: 0.95}}.FleetConfig()
	if got.StarvationK != 5 || got.StarvationWeight != 0.95 {
		t.Errorf("starvation = %d %v, want 5 0.95", got.StarvationK, got.StarvationWeight)
	}
	if low := (policy.Policy{Fleet: &policy.FleetPolicy{StarvationWeight: 0.2}}).FleetConfig(); low.StarvationWeight != 0.9 || low.StarvationK != 3 {
		t.Errorf("starvation = %d %v, want the default k 3 and the weight raised to the 0.9 floor", low.StarvationK, low.StarvationWeight)
	}
}
