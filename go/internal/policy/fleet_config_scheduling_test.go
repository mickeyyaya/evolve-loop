package policy_test

// fleet_config_scheduling_test.go — RED contract for cycle-550's
// supervisor-continuous-lane-keeping task (L5, the fleet-width architecture's
// "ceiling-keeper").
//
// PROBLEM (inbox 2026-07-06T16-00-00Z-supervisor-continuous-lane-keeping.json,
// operator directive verbatim: "the supervisor must try its best to honor the
// setting"): today's wave scheduler is synchronized on a barrier — plan wave,
// dispatch N lanes, WAIT for every lane to finish, THEN plan the next wave. A
// lane that exits early (PASS or FAIL) cannot be replaced until every sibling
// lane in its wave also finishes, so realized concurrency is min-over-time,
// not the operator's configured fleet.count. The fix is a rolling lane pool
// (fleet.RunPool, see pool_test.go) that backfills a replacement lane
// immediately on any lane exit. Rollout is config-gated per repo precedent
// (mirrors SwarmPolicy.Stage / FleetBudgetPolicy.Stage's shadow-first idiom):
// a NEW `policy.fleet.scheduling` closed-vocab knob, "wave" (today's
// behavior, the default) or "pool" (the rolling lane pool).
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this package's test build fails to compile today; that compile failure IS
// the RED evidence, mirroring the cycle-465/507/547 precedent):
//
//	FleetPolicy gains a `Scheduling string `json:"scheduling,omitempty"``
//	field (raw .evolve/policy.json value) and FleetConfig gains a resolved
//	`Scheduling string` field. FleetConfig() resolves it as a closed
//	vocabulary: empty/absent AND explicit "wave" -> "wave" (byte-identical
//	regression: every OTHER resolved field on FleetConfig is untouched by
//	this knob); explicit "pool" -> "pool"; any OTHER value (operator typo)
//	fails safe to "wave" -- NOT "pool" -- plus a surfaced warning naming the
//	rejected value, because "wave" is the current/safe default this knob
//	must never silently escalate away from (unlike PlanSource, whose unknown
//	value fails to "manual" per its own spec — see
//	TestFleetConfig_PlanSourceClosedVocab in fleet_config_param_test.go for
//	the contrasting precedent this test intentionally does NOT mirror).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive/Semantic : empty/"wave"/"pool" all resolve to their own value,
//     no warning.
//   - Negative           : an unknown value ("yolo") must NOT pass through
//     unchanged (a naive passthrough getter fails this) and must NOT resolve
//     to "pool" (a fail-open-to-the-new-mode bug would be worse than a
//     passthrough — it would silently opt an operator into the unsoaked
//     rolling pool). It must fail safe to "wave" WITH a warning.
//   - Regression         : an absent/empty fleet.scheduling must leave
//     Count/Concurrency/PlanSource/MinLanes resolution completely unaffected
//     (byte-identical wave-mode config), independent of this new field.
import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// TestFleetConfig_SchedulingClosedVocab pins the fleet.scheduling closed
// vocabulary: "wave" is the default (empty/absent) AND the explicit
// passthrough, "pool" opts in explicitly, and any OTHER value fails safe to
// "wave" (not "pool", and not a bare passthrough of the typo) with a
// surfaced warning naming the rejected value -- so an operator's typo can
// never silently enable the new unsoaked scheduling mode.
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

// TestFleetConfig_SchedulingAbsentPreservesRestOfConfigByteIdentical is the
// regression guard: introducing fleet.scheduling must not perturb the
// pre-existing Count/Concurrency/MinLanes/PlanSource resolution when the
// operator's policy.json has no scheduling key at all (today's on-disk
// shape, universally) — the wave-mode byte-identical-behavior acceptance
// criterion starts at the config layer.
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
