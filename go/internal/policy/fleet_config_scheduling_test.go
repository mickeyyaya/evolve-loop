package policy_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestFleetConfig_SchedulingClosedVocab(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantSched   string
		wantWarning bool
	}{
		{"empty-defaults-to-wave-no-warning", "", "wave", false},
		{"explicit-wave-no-warning", "wave", "wave", false},
		{"explicit-pool-passthrough-no-warning", "pool", "pool", false},
		{"unknown-fails-safe-to-wave-with-warning", "yolo", "wave", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := policy.Policy{Fleet: &policy.FleetPolicy{Scheduling: tc.raw}}.FleetConfig()
			if got.Scheduling != tc.wantSched {
				t.Errorf("FleetConfig().Scheduling = %q, want %q (raw=%q)", got.Scheduling, tc.wantSched, tc.raw)
			}
			hasWarning := len(got.Warnings) > 0
			if hasWarning != tc.wantWarning {
				t.Errorf("FleetConfig().Warnings = %v (non-empty=%v), want non-empty=%v (raw=%q)", got.Warnings, hasWarning, tc.wantWarning, tc.raw)
			}
			if tc.wantWarning {
				found := false
				for _, w := range got.Warnings {
					if strings.Contains(w, tc.raw) {
						found = true
					}
				}
				if !found {
					t.Errorf("FleetConfig().Warnings = %v, want a warning naming the rejected value %q", got.Warnings, tc.raw)
				}
			}
		})
	}
}

func TestFleetConfig_SchedulingAbsentPreservesRestOfConfigByteIdentical(t *testing.T) {
	pol := policy.Policy{Fleet: &policy.FleetPolicy{Count: 2, MinLanes: 2, PlanSource: "manual"}}
	got := pol.FleetConfig()
	if got.Scheduling != "wave" {
		t.Fatalf("FleetConfig().Scheduling = %q, want \"wave\" (default) when scheduling is absent", got.Scheduling)
	}
	if got.Count != 2 || got.Concurrency != 2 || got.MinLanes != 2 || got.PlanSource != "manual" {
		t.Errorf("FleetConfig() = %+v, want Count=2 Concurrency=2 MinLanes=2 PlanSource=manual unaffected by the new Scheduling field", got)
	}
}
