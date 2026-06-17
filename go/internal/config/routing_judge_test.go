package config

import "testing"

// WS4-S3 (ADR-0052 advisor-maximization): EVOLVE_ROUTING_JUDGE is the
// route-quality judge toggle — a plain off/on bool (NOT a Stage: the judge is
// off the build path and cannot move behavior). This slice reserves the
// composition-root view + parse; the scoring call site reads it. A typo falls
// back to off (fail-safe, never silently enabling the judge).

func TestLoad_RoutingJudge_DefaultsOff(t *testing.T) {
	cfg, _ := Load("", map[string]string{})
	if cfg.RoutingJudge {
		t.Error("default RoutingJudge=true, want false (off — byte-identical default)")
	}
}

func TestLoad_RoutingJudge_EnvOverride(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"on", true},
		{"1", true},
		{"off", false},
		{"0", false},
	}
	for _, c := range cases {
		t.Run(c.val, func(t *testing.T) {
			cfg, ws := Load("", map[string]string{"EVOLVE_ROUTING_JUDGE": c.val})
			if cfg.RoutingJudge != c.want {
				t.Errorf("RoutingJudge=%v, want %v", cfg.RoutingJudge, c.want)
			}
			for _, w := range ws {
				if w.Code == "unknown-value" {
					t.Errorf("unexpected unknown-value warning on %q: %+v", c.val, w)
				}
			}
		})
	}
}

func TestLoad_RoutingJudge_UnknownValueFallsBackToOff(t *testing.T) {
	// "shadow"/"advisory" are exactly the stage-words a reader might wrongly
	// assume work here — they must NOT enable the judge (it's a bool, not a stage).
	for _, bogus := range []string{"shadow", "advisory", "bogus"} {
		cfg, ws := Load("", map[string]string{"EVOLVE_ROUTING_JUDGE": bogus})
		if cfg.RoutingJudge {
			t.Errorf("%q must NOT enable the judge (fail-safe to off); got true", bogus)
		}
		var sawWarn bool
		for _, w := range ws {
			if w.Code == "unknown-value" {
				sawWarn = true
			}
		}
		if !sawWarn {
			t.Errorf("%q: expected unknown-value warning; got %+v", bogus, ws)
		}
	}
}
