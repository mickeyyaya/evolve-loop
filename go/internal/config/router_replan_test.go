package config

import "testing"

// WS0-S1 (ADR-0052 advisor-maximization): EVOLVE_ROUTER_REPLAN is the post-scout
// re-plan rollout dial — off → shadow (default) → advisory — mirroring
// EVOLVE_DYNAMIC_ROUTING's ladder. This slice only reserves the composition-root
// view + parse; the re-plan behavior wires in WS2-S3. A typo falls back to off
// (fail-safe, never silently enabling the re-plan).

func TestLoad_RouterReplan_DefaultsShadow(t *testing.T) {
	cfg, _ := Load("", map[string]string{})
	if cfg.RouterReplan != StageShadow {
		t.Errorf("default RouterReplan=%v, want StageShadow", cfg.RouterReplan)
	}
}

func TestLoad_RouterReplan_EnvOverride(t *testing.T) {
	cases := []struct {
		val  string
		want Stage
	}{
		{"off", StageOff},
		{"0", StageOff},
		{"shadow", StageShadow},
		{"advisory", StageAdvisory},
	}
	for _, c := range cases {
		t.Run(c.val, func(t *testing.T) {
			cfg, ws := Load("", map[string]string{"EVOLVE_ROUTER_REPLAN": c.val})
			if cfg.RouterReplan != c.want {
				t.Errorf("RouterReplan=%v, want %v", cfg.RouterReplan, c.want)
			}
			for _, w := range ws {
				if w.Code == "unknown-value" {
					t.Errorf("unexpected unknown-value warning on %q: %+v", c.val, w)
				}
			}
		})
	}
}

func TestLoad_RouterReplan_UnknownValueFallsBackToOff(t *testing.T) {
	cfg, ws := Load("", map[string]string{"EVOLVE_ROUTER_REPLAN": "bogus"})
	if cfg.RouterReplan != StageOff {
		t.Errorf("unknown value did NOT fall back to off; got %v", cfg.RouterReplan)
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
