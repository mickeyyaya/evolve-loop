package config

import "testing"

// Workstream E2 config parsing. ReviewGate accepts the same off/shadow/enforce
// trichotomy as CommitEvidence; unknown values fall back to off with a
// warning (typo must never silently enable a kill path).

func TestLoad_ReviewGate_DefaultsOff(t *testing.T) {
	cfg, _ := Load("", map[string]string{})
	if cfg.ReviewGate != StageOff {
		t.Errorf("default ReviewGate=%v, want StageOff", cfg.ReviewGate)
	}
}

func TestLoad_ReviewGate_EnvOverride(t *testing.T) {
	cases := []struct {
		val  string
		want Stage
	}{
		{"off", StageOff},
		{"0", StageOff},
		{"shadow", StageShadow},
		{"enforce", StageEnforce},
	}
	for _, c := range cases {
		t.Run(c.val, func(t *testing.T) {
			cfg, ws := Load("", map[string]string{"EVOLVE_REVIEW_GATE": c.val})
			if cfg.ReviewGate != c.want {
				t.Errorf("ReviewGate=%v, want %v", cfg.ReviewGate, c.want)
			}
			for _, w := range ws {
				if w.Code == "unknown-value" {
					t.Errorf("unexpected unknown-value warning on %q: %+v", c.val, w)
				}
			}
		})
	}
}

func TestLoad_ReviewGate_UnknownValueFallsBackToOff(t *testing.T) {
	cfg, ws := Load("", map[string]string{"EVOLVE_REVIEW_GATE": "advisory"})
	if cfg.ReviewGate != StageOff {
		t.Errorf("unknown value did NOT fall back to off; got %v", cfg.ReviewGate)
	}
	var sawWarn bool
	for _, w := range ws {
		if w.Code == "unknown-value" {
			sawWarn = true
		}
	}
	if !sawWarn {
		t.Errorf("expected unknown-value warning; got %+v", ws)
	}
}
