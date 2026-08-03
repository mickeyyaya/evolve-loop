package policy

import "testing"

// regressiontia_test.go — RED contract for cycle-1260 Task 1
// (`egps-regression-tia-shadow-wiring`, inbox item
// .evolve/inbox/2026-07-30T09-00-00Z-egps-regression-tia-selection.json,
// P1 weight 0.91, 3rd live instance).
//
// The config surface. Test-impact selection over the EGPS regression corpus is
// a staged rollout (off → shadow → enforce), and per the standing rules it is
// config-as-code in .evolve/policy.json — never a flag, never a Go literal
// ([[phase_settings_from_config_not_code]], [[no_feature_flags_use_design_patterns]]).
// The shape is copied verbatim from the sibling ParallelEvaluate/Disposition
// blocks: a `{"stage": "..."}` object, resolved through an accessor that
// applies defaults and fail-safes.
//
// The contract these tests freeze:
//
//	type RegressionTIAPolicy struct{ Stage string `json:"stage,omitempty"` }
//	func (p Policy) RegressionTIAConfig() RegressionTIAConfig   // {Stage string}
//
//   - absent block (nil pointer) / zero-value Policy{} ⇒ Stage "off". The
//     checked-in .evolve/policy.json carries NO regression_tia block, so "off"
//     is the live production default and must be a byte-identical no-op.
//   - the closed vocabulary {"off","shadow","enforce"} is honored verbatim.
//   - ANY unknown value (typo, empty-after-trim, "on", "true") maps to "off" —
//     fail-safe, so a misspelling can never silently arm test selection and
//     hide a regression class. This is the negative axis and the whole point:
//     the failure mode this item exists to prevent is selection that skips a
//     predicate which would have caught a real red.
//
// RED today: RegressionTIAConfig/RegressionTIAPolicy are undefined, so package
// policy's tests fail to COMPILE — a hard non-zero exit, never a silent pass.

// TestRegressionTIAConfig_DefaultsOff pins the production default. The
// checked-in policy.json has no regression_tia block, so the zero-value Policy
// and an explicitly-nil block must BOTH resolve to "off" — the stage under
// which acssuite must behave byte-identically to its pre-change self.
func TestRegressionTIAConfig_DefaultsOff(t *testing.T) {
	if got := (Policy{}).RegressionTIAConfig().Stage; got != "off" {
		t.Errorf("zero-value Policy{}.RegressionTIAConfig().Stage = %q, want \"off\" — the checked-in policy.json has no regression_tia block, so absent MUST mean dormant", got)
	}
	p := Policy{RegressionTIA: nil}
	if got := p.RegressionTIAConfig().Stage; got != "off" {
		t.Errorf("nil RegressionTIA block ⇒ Stage = %q, want \"off\"", got)
	}
	// An empty stage string inside a present block is still "absent".
	p = Policy{RegressionTIA: &RegressionTIAPolicy{Stage: ""}}
	if got := p.RegressionTIAConfig().Stage; got != "off" {
		t.Errorf("present block with empty stage ⇒ Stage = %q, want \"off\"", got)
	}
}

// TestRegressionTIAConfig_HonorsClosedVocabulary pins the three legal stages.
func TestRegressionTIAConfig_HonorsClosedVocabulary(t *testing.T) {
	for _, stage := range []string{"off", "shadow", "enforce"} {
		p := Policy{RegressionTIA: &RegressionTIAPolicy{Stage: stage}}
		if got := p.RegressionTIAConfig().Stage; got != stage {
			t.Errorf("stage %q resolved to %q, want it honored verbatim", stage, got)
		}
	}
}

// TestRegressionTIAConfig_UnknownStageFallsBackToOff is the NEGATIVE axis: an
// unrecognized stage must never arm selection. A no-op implementation that
// simply echoes the configured string back fails here.
func TestRegressionTIAConfig_UnknownStageFallsBackToOff(t *testing.T) {
	for _, stage := range []string{"on", "true", "Shadow", "ENFORCE", "advsory", "1", " shadow"} {
		p := Policy{RegressionTIA: &RegressionTIAPolicy{Stage: stage}}
		if got := p.RegressionTIAConfig().Stage; got != "off" {
			t.Errorf("unknown stage %q resolved to %q, want \"off\" — a typo must not silently arm regression selection", stage, got)
		}
	}
}
