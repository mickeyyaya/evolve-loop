package policy_test

// FleetPolicy/FleetConfig — the .evolve/policy.json "fleet" block (S1 of the
// FLEET-AS-POLICY operator-priority goal, cycle 464), mirroring the
// SwarmPolicy/SwarmConfig precedent exactly (policy.go:779-845): a raw
// *FleetPolicy block on Policy, a resolved FleetConfig struct, and a
// FleetConfig() getter with fail-safe defaults. Absent block ⇒ Count=1
// (byte-identical sequential execution); Concurrency<=0 ⇒ follows the
// resolved Count; PlanSource is closed-vocabulary ("triage"|"manual") with
// an unknown value failing safe to "manual" PLUS a surfaced warning (unlike
// the swarm/parallel_evaluate precedents, which fail back to their DEFAULT
// value — this block's spec explicitly calls for the non-default fail-safe
// branch, so the getter is not I/O: it returns the warning as data on the
// resolved config rather than logging, matching the package's no-I/O-in-
// getters style).
//
// Black-box: drives only the exported Policy/FleetPolicy/FleetConfig
// surface, zero env.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// TestFleetConfig_Resolution pins the count/concurrency resolution table:
// absent/empty block and non-positive Count all clamp to 1 (never a
// zero-lane or negative-lane wave); Count overrides pass through; a
// non-positive Concurrency follows the RESOLVED Count (not the raw input),
// and an explicit positive Concurrency passes through independently of
// Count. A hardcoded-defaults getter (ignoring the override) fails the
// count:3 and count:2/concurrency:5 cases.
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

// TestFleetConfig_PlanSourceClosedVocab pins the plan_source closed
// vocabulary: "triage" is the default (empty/absent), "manual" passes
// through, and any OTHER value (an operator typo, e.g. "yolo") fails safe
// to "manual" — NOT to "yolo" (a passthrough getter with no vocabulary
// check) and NOT to the "triage" default (the swarm/parallel_evaluate
// precedent's fail-to-default idiom would be wrong here per the goal spec)
// — and the getter must surface a non-empty warning on the resolved config
// so callers can log/report it without the getter itself doing I/O. Valid
// values (explicit "triage", "manual", and the empty default) must NOT
// produce a warning.
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
