package envchain

import (
	"testing"
)

// TestResolve_PrecedenceOrder exhaustively walks the four-tier chain
// in a single table so the precedence semantics are visible at a glance.
func TestResolve_PrecedenceOrder(t *testing.T) {
	t.Setenv("TEST_KEY", "")

	cases := []struct {
		name    string
		reqEnv  map[string]string
		procEnv string // applied via t.Setenv
		profile string
		def     string
		want    string
	}{
		{name: "all-empty-returns-default", def: "fallback", want: "fallback"},
		{name: "only-default", def: "d", want: "d"},
		{name: "profile-beats-default", profile: "p", def: "d", want: "p"},
		{name: "process-env-beats-profile", procEnv: "e", profile: "p", def: "d", want: "e"},
		{name: "req-env-beats-process-env", reqEnv: map[string]string{"TEST_KEY": "r"}, procEnv: "e", profile: "p", def: "d", want: "r"},
		{name: "empty-req-env-falls-through-to-process", reqEnv: map[string]string{"TEST_KEY": ""}, procEnv: "e", profile: "p", want: "e"},
		{name: "empty-process-env-falls-through-to-profile", procEnv: "", profile: "p", want: "p"},
		{name: "nil-req-env-is-safe", reqEnv: nil, procEnv: "e", want: "e"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("TEST_KEY", c.procEnv)
			got := Resolve("TEST_KEY", c.reqEnv, c.profile, c.def)
			if got != c.want {
				t.Errorf("Resolve()=%q, want %q", got, c.want)
			}
		})
	}
}

// TestResolve_DistinctKeysDoNotCollide — sanity that two keys with
// the same value type don't leak across each other.
func TestResolve_DistinctKeysDoNotCollide(t *testing.T) {
	t.Setenv("KEY_A", "a-val")
	t.Setenv("KEY_B", "b-val")
	if got := Resolve("KEY_A", nil, "", ""); got != "a-val" {
		t.Errorf("KEY_A=%q, want a-val", got)
	}
	if got := Resolve("KEY_B", nil, "", ""); got != "b-val" {
		t.Errorf("KEY_B=%q, want b-val", got)
	}
	if got := Resolve("KEY_C_UNSET", nil, "", "default"); got != "default" {
		t.Errorf("KEY_C_UNSET=%q, want default", got)
	}
}

// TestPhaseEnvKey_CanonicalForm — every canonical phase maps to its
// documented env-var key. Includes hyphenated names for future-proofing.
func TestPhaseEnvKey_CanonicalForm(t *testing.T) {
	cases := []struct {
		phase, suffix, want string
	}{
		{"build", "PERMISSION_MODE", "EVOLVE_BUILD_PERMISSION_MODE"},
		{"scout", "MODEL", "EVOLVE_SCOUT_MODEL"},
		{"intent", "PERMISSION_MODE", "EVOLVE_INTENT_PERMISSION_MODE"},
		{"triage", "MODEL", "EVOLVE_TRIAGE_MODEL"},
		{"tdd", "PLAN_INPUT", "EVOLVE_TDD_PLAN_INPUT"},
		{"audit", "INTERACTIVE_POLICY", "EVOLVE_AUDIT_INTERACTIVE_POLICY"},
		{"tdd-engineer", "MODEL", "EVOLVE_TDD_ENGINEER_MODEL"},
		{"plan-reviewer", "PERMISSION_MODE", "EVOLVE_PLAN_REVIEWER_PERMISSION_MODE"},
	}
	for _, c := range cases {
		if got := PhaseEnvKey(c.phase, c.suffix); got != c.want {
			t.Errorf("PhaseEnvKey(%q,%q)=%q, want %q", c.phase, c.suffix, got, c.want)
		}
	}
}

// TestPhaseEnvKey_EmptyInputs — boundary cases. Empty phase and empty
// suffix are caller errors but we want defined (not panicking) behavior.
func TestPhaseEnvKey_EmptyInputs(t *testing.T) {
	if got := PhaseEnvKey("", "MODEL"); got != "EVOLVE__MODEL" {
		t.Errorf("empty phase: %q", got)
	}
	if got := PhaseEnvKey("build", ""); got != "EVOLVE_BUILD_" {
		t.Errorf("empty suffix: %q", got)
	}
}
