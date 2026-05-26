package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestPhasePolicy_Enabled(t *testing.T) {
	cfg := testCfg()
	cfg.PhaseEnable["plan-review"] = config.EnableOff
	cfg.PhaseEnable["intent"] = config.EnableOn
	p := NewPhasePolicy(cfg)

	trivial := RoutingSignals{Scout: ScoutSignals{CycleSizeEstimate: "trivial", Present: true}}
	medium := RoutingSignals{Scout: ScoutSignals{CycleSizeEstimate: "medium", Present: true}}
	red := RoutingSignals{Build: BuildSignals{ACSRed: 4, Present: true}}

	cases := []struct {
		name  string
		phase string
		sig   RoutingSignals
		want  bool
	}{
		{"mandatory always on", "build", trivial, true},
		{"tdd pinned non-trivial", "tdd", medium, true},
		{"tdd skippable trivial", "tdd", trivial, false},
		{"forced off", "plan-review", medium, false},
		{"forced on", "intent", trivial, true},
		{"tester trigger fires on red", "tester", red, true},
		{"tester trigger silent without red", "tester", medium, false},
	}
	for _, c := range cases {
		if got := p.Enabled(c.phase, c.sig); got != c.want {
			t.Errorf("%s: Enabled(%q) = %v, want %v", c.name, c.phase, got, c.want)
		}
	}
}
