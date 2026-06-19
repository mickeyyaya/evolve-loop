package config

import "testing"

// Workstream E2 defaults. Runtime overrides are applied from policy.GatesConfig
// at the composition root rather than by config.Load.

func TestLoad_ReviewGate_DefaultsOff(t *testing.T) {
	cfg, _ := Load("", map[string]string{})
	if cfg.ReviewGate != StageOff {
		t.Errorf("default ReviewGate=%v, want StageOff", cfg.ReviewGate)
	}
}

func TestLoad_ReviewGate_IgnoresRemovedEnvOverride(t *testing.T) {
	cfg, ws := Load("", map[string]string{"EVOLVE_REVIEW_GATE": "advisory"})
	if cfg.ReviewGate != StageOff {
		t.Errorf("removed env override changed ReviewGate; got %v", cfg.ReviewGate)
	}
	for _, w := range ws {
		if w.Code == "unknown-value" {
			t.Errorf("removed env override should be ignored without warning: %+v", ws)
		}
	}
}
