package config

import "testing"

func TestLoad_ReviewGate_DefaultsOff(t *testing.T) {
	cfg, _ := Load("")
	if cfg.ReviewGate != StageOff {
		t.Errorf("default ReviewGate=%v, want StageOff", cfg.ReviewGate)
	}
}

func TestLoad_ReviewGate_IgnoresRemovedEnvOverride(t *testing.T) {
	cfg, ws := Load("")
	if cfg.ReviewGate != StageOff {
		t.Errorf("removed env override changed ReviewGate; got %v", cfg.ReviewGate)
	}
	for _, w := range ws {
		if w.Code == "unknown-value" {
			t.Errorf("removed env override should be ignored without warning: %+v", ws)
		}
	}
}
