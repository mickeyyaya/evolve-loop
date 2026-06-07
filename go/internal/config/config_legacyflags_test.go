package config

import (
	"strings"
	"testing"
)

// Locks the legacy per-phase flag mapping that PhasePolicy depends on to
// reproduce pre-routing run/skip behavior. Loads with a missing registry so
// only defaults() + env overrides apply.
func TestLoad_LegacyPhaseFlags(t *testing.T) {
	cases := []struct {
		name  string
		env   map[string]string
		phase string
		want  Enable
	}{
		{"default triage runs", map[string]string{}, "triage", EnableOn},
		{"default tdd runs", map[string]string{}, "tdd", EnableOn},
		{"default build-planner skipped", map[string]string{}, "build-planner", EnableOff},
		{"EVOLVE_TRIAGE_DISABLE=1 off", map[string]string{"EVOLVE_TRIAGE_DISABLE": "1"}, "triage", EnableOff},
		{"EVOLVE_TRIAGE_DISABLE=0 on", map[string]string{"EVOLVE_TRIAGE_DISABLE": "0"}, "triage", EnableOn},
		{"EVOLVE_TEST_PHASE_ENABLED=0 off", map[string]string{"EVOLVE_TEST_PHASE_ENABLED": "0"}, "tdd", EnableOff},
		{"EVOLVE_TEST_PHASE_ENABLED=1 on", map[string]string{"EVOLVE_TEST_PHASE_ENABLED": "1"}, "tdd", EnableOn},
		{"EVOLVE_BUILD_PLANNER=1 on", map[string]string{"EVOLVE_BUILD_PLANNER": "1"}, "build-planner", EnableOn},
	}
	for _, c := range cases {
		cfg, _ := Load("/nonexistent/phase-registry.json", c.env)
		if got := cfg.PhaseEnable[c.phase]; got != c.want {
			t.Errorf("%s: PhaseEnable[%q]=%v, want %v", c.name, c.phase, got, c.want)
		}
	}
}

// The stale EVOLVE_TDD_PHASE key must NOT influence tdd anymore (the phase
// reads EVOLVE_TEST_PHASE_ENABLED; config now binds that one).
func TestLoad_EVOLVE_TDD_PHASE_NoLongerBound(t *testing.T) {
	cfg, _ := Load("/nonexistent/phase-registry.json", map[string]string{"EVOLVE_TDD_PHASE": "0"})
	if got := cfg.PhaseEnable["tdd"]; got != EnableOn {
		t.Errorf("tdd enable=%v, want EnableOn (EVOLVE_TDD_PHASE must be inert; only EVOLVE_TEST_PHASE_ENABLED binds)", got)
	}
}

// Phase 5 migration: EVOLVE_DISABLE_AUTO_RETROSPECTIVE is deprecated in
// favor of policy.json:failure_floor (the ONE surface, phase 4a) but stays
// honored for one more release — set, it still disables the retrospective
// phase AND emits a deprecation warning pointing at the replacement.
// (failure_floor beats the flag when both are set: the policy route bypasses
// the enable chain entirely — pinned by router's
// TestAuditFail_RoutesPerFailurePolicyNotEnableVar.)
func TestDisableAutoRetro_DeprecatedButHonored(t *testing.T) {
	for _, v := range []string{"1", "0"} {
		cfg, ws := Load("/nonexistent/phase-registry.json", map[string]string{"EVOLVE_DISABLE_AUTO_RETROSPECTIVE": v})
		want := EnableOff
		if v == "0" {
			want = EnableContent
		}
		if got := cfg.PhaseEnable["retrospective"]; got != want {
			t.Errorf("=%s: PhaseEnable[retrospective]=%v, want %v (honored for one more release)", v, got, want)
		}
		found := false
		for _, w := range ws {
			if w.Code == "deprecated-flag" &&
				strings.Contains(w.Message, "EVOLVE_DISABLE_AUTO_RETROSPECTIVE") &&
				strings.Contains(w.Message, "failure_floor") {
				found = true
			}
		}
		if !found {
			t.Errorf("=%s: no deprecated-flag warning pointing at policy.json failure_floor; got %v", v, ws)
		}
	}

	// Unset ⇒ no nudge (the warning fires only when the operator uses the flag).
	_, ws := Load("/nonexistent/phase-registry.json", map[string]string{})
	for _, w := range ws {
		if w.Code == "deprecated-flag" {
			t.Errorf("unset flag must not warn; got %+v", w)
		}
	}

	// Non-canonical value (the loop's early-continue path): no binding, no
	// warning — byte-identical to the old switch's implicit-no-op default.
	cfg, ws := Load("/nonexistent/phase-registry.json", map[string]string{"EVOLVE_DISABLE_AUTO_RETROSPECTIVE": "true"})
	if got := cfg.PhaseEnable["retrospective"]; got == EnableOff {
		t.Errorf("=true must not bind (only 1/0 do); got %v", got)
	}
	for _, w := range ws {
		if w.Code == "deprecated-flag" {
			t.Errorf("non-canonical value must not warn; got %+v", w)
		}
	}
}
