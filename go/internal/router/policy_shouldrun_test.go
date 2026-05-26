package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// ShouldRunPhase must be flag-faithful below Enforce (legacy Stage:Off path)
// and apply the kernel pin from Enforce up.
func TestShouldRunPhase_StageAware(t *testing.T) {
	base := func(stage config.Stage, enable map[string]config.Enable) config.RoutingConfig {
		c := config.RoutingConfig{
			Stage:       stage,
			Mandatory:   []string{"scout", "build", "audit", "ship"},
			Conditional: map[string]config.CondRule{"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial"}},
			PhaseEnable: map[string]config.Enable{"triage": config.EnableOn, "tdd": config.EnableOn, "build-planner": config.EnableOff},
			Triggers:    map[string]config.RoutingBlock{},
		}
		for k, v := range enable {
			c.PhaseEnable[k] = v
		}
		return c
	}

	cases := []struct {
		name  string
		cfg   config.RoutingConfig
		phase string
		want  bool
	}{
		{"off: triage default runs", base(config.StageOff, nil), "triage", true},
		{"off: tdd default runs", base(config.StageOff, nil), "tdd", true},
		{"off: build-planner default skips", base(config.StageOff, nil), "build-planner", false},
		{"off: tdd flag-off skips (no pin below enforce)", base(config.StageOff, map[string]config.Enable{"tdd": config.EnableOff}), "tdd", false},
		{"shadow: tdd flag-off still skips", base(config.StageShadow, map[string]config.Enable{"tdd": config.EnableOff}), "tdd", false},
		{"enforce: tdd flag-off is pinned (kernel)", base(config.StageEnforce, map[string]config.Enable{"tdd": config.EnableOff}), "tdd", true},
		{"enforce: mandatory always on", base(config.StageEnforce, nil), "build", true},
	}
	for _, c := range cases {
		if got := NewPhasePolicy(c.cfg).ShouldRunPhase(c.phase); got != c.want {
			t.Errorf("%s: ShouldRunPhase(%q)=%v, want %v", c.name, c.phase, got, c.want)
		}
	}
}

// PolicyForProject falls back to config defaults when the registry is absent,
// so a phase's enablement resolves correctly without a registry file present.
func TestPolicyForProject_FallsBackToDefaults(t *testing.T) {
	p := PolicyForProject("/nonexistent", map[string]string{})
	if !p.ShouldRunPhase("triage") {
		t.Error("triage should default to run with no registry")
	}
	if p.ShouldRunPhase("build-planner") {
		t.Error("build-planner should default to skip with no registry")
	}
}
