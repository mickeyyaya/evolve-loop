package config

import "testing"

// Despite its name, this pins the compiled phase-enable defaults; the legacy env flags are gone.
func TestLoad_LegacyPhaseFlags(t *testing.T) {
	cases := []struct {
		name  string
		phase string
		want  Enable
	}{
		{"default triage runs", "triage", EnableOn},
		{"default tdd runs", "tdd", EnableOn},
		{"default build-planner skipped", "build-planner", EnableOff},
	}
	for _, c := range cases {
		cfg, _ := Load("/nonexistent/phase-registry.json", map[string]string{})
		if got := cfg.PhaseEnable[c.phase]; got != c.want {
			t.Errorf("%s: PhaseEnable[%q]=%v, want %v", c.name, c.phase, got, c.want)
		}
	}
}

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
