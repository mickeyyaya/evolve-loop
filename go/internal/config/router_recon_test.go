package config

import "testing"

// WS2-S0b (ADR-0052 advisor-maximization): EVOLVE_ROUTER_RECON_DIGEST toggles
// the deterministic pre-plan recon. A plain off/on bool (it injects facts the
// floor still clamps, so there is no shadow/advisory distinction). Default off =
// byte-identical initial plan; a typo falls back to off (fail-safe).

func TestLoad_ReconDigest_DefaultsOff(t *testing.T) {
	cfg, _ := Load("", map[string]string{})
	if cfg.ReconDigest {
		t.Error("default ReconDigest=true, want false (off — byte-identical default)")
	}
}

func TestLoad_ReconDigest_EnvOverride(t *testing.T) {
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
			cfg, ws := Load("", map[string]string{"EVOLVE_ROUTER_RECON_DIGEST": c.val})
			if cfg.ReconDigest != c.want {
				t.Errorf("ReconDigest=%v, want %v", cfg.ReconDigest, c.want)
			}
			for _, w := range ws {
				if w.Code == "unknown-value" {
					t.Errorf("unexpected unknown-value warning on %q: %+v", c.val, w)
				}
			}
		})
	}
}

func TestLoad_ReconDigest_UnknownValueFallsBackToOff(t *testing.T) {
	for _, bogus := range []string{"shadow", "advisory", "bogus"} {
		cfg, ws := Load("", map[string]string{"EVOLVE_ROUTER_RECON_DIGEST": bogus})
		if cfg.ReconDigest {
			t.Errorf("%q must NOT enable recon (fail-safe to off); got true", bogus)
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
