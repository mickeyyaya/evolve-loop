package config

import "testing"

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

func TestDisableAutoRetro_RetiredAndIgnored(t *testing.T) {
	for _, v := range []string{"1", "0", "true"} {
		cfg, ws := Load("/nonexistent/phase-registry.json", map[string]string{"EVOLVE_DISABLE_AUTO_RETROSPECTIVE": v})
		if got, ok := cfg.PhaseEnable["retrospective"]; ok {
			t.Errorf("=%s: retired flag must not bind retrospective; got %v", v, got)
		}
		for _, w := range ws {
			if w.Code == "deprecated-flag" {
				t.Errorf("=%s: retired flag must not warn; got %+v", v, w)
			}
		}
	}
}
