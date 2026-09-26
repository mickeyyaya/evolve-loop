package policy_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestFleetConfig_Resolution(t *testing.T) {
	defaults := policy.FleetConfig{Count: 1, Concurrency: 1, PlanSource: "triage"}
	cases := []struct {
		name string
		pol  policy.Policy
		want policy.FleetConfig
	}{
		{"absent-defaults", policy.Policy{}, defaults},
		{"empty-block-defaults", policy.Policy{Fleet: &policy.FleetPolicy{}}, defaults},
		{"count-zero-clamps-to-one", policy.Policy{Fleet: &policy.FleetPolicy{Count: 0}}, defaults},
		{"count-negative-clamps-to-one", policy.Policy{Fleet: &policy.FleetPolicy{Count: -3}}, defaults},
		{
			"count-override",
			policy.Policy{Fleet: &policy.FleetPolicy{Count: 3}},
			policy.FleetConfig{Count: 3, Concurrency: 3, PlanSource: "triage"},
		},
		{
			"concurrency-zero-follows-resolved-count",
			policy.Policy{Fleet: &policy.FleetPolicy{Count: 2, Concurrency: 0}},
			policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"},
		},
		{
			"concurrency-override-independent-of-count",
			policy.Policy{Fleet: &policy.FleetPolicy{Count: 2, Concurrency: 5}},
			policy.FleetConfig{Count: 2, Concurrency: 5, PlanSource: "triage"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pol.FleetConfig()
			if got.Count != tc.want.Count {
				t.Errorf("FleetConfig().Count = %d, want %d", got.Count, tc.want.Count)
			}
			if got.Concurrency != tc.want.Concurrency {
				t.Errorf("FleetConfig().Concurrency = %d, want %d", got.Concurrency, tc.want.Concurrency)
			}
			if got.PlanSource != tc.want.PlanSource {
				t.Errorf("FleetConfig().PlanSource = %q, want %q", got.PlanSource, tc.want.PlanSource)
			}
			if len(got.Warnings) != 0 {
				t.Errorf("FleetConfig().Warnings = %v, want none for a valid/absent plan_source", got.Warnings)
			}
		})
	}
}

func TestFleetConfig_MinLanesResolution(t *testing.T) {
	cases := []struct {
		name string
		pol  policy.Policy
		want int
	}{
		{"absent-defaults-to-1", policy.Policy{}, 1},
		{"empty-block-defaults-to-1", policy.Policy{Fleet: &policy.FleetPolicy{}}, 1},
		{"zero-defaults-to-1", policy.Policy{Fleet: &policy.FleetPolicy{Count: 2, MinLanes: 0}}, 1},
		{"negative-defaults-to-1", policy.Policy{Fleet: &policy.FleetPolicy{Count: 2, MinLanes: -4}}, 1},
		{"one-is-1", policy.Policy{Fleet: &policy.FleetPolicy{Count: 2, MinLanes: 1}}, 1},
		{"override-2-of-count-2", policy.Policy{Fleet: &policy.FleetPolicy{Count: 2, MinLanes: 2}}, 2},
		{"override-3-of-count-4", policy.Policy{Fleet: &policy.FleetPolicy{Count: 4, MinLanes: 3}}, 3},
		{"override-above-count-clamps-down", policy.Policy{Fleet: &policy.FleetPolicy{Count: 2, MinLanes: 9}}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pol.FleetConfig().MinLanes; got != tc.want {
				t.Errorf("FleetConfig().MinLanes = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFleetConfig_PlanSourceClosedVocab(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantSource  string
		wantWarning bool
	}{
		{"empty-defaults-to-triage-no-warning", "", "triage", false},
		{"explicit-triage-no-warning", "triage", "triage", false},
		{"explicit-manual-passthrough-no-warning", "manual", "manual", false},
		{"unknown-fails-safe-to-manual-with-warning", "yolo", "manual", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := policy.Policy{Fleet: &policy.FleetPolicy{PlanSource: tc.raw}}.FleetConfig()
			if got.PlanSource != tc.wantSource {
				t.Errorf("FleetConfig().PlanSource = %q, want %q (raw=%q)", got.PlanSource, tc.wantSource, tc.raw)
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
