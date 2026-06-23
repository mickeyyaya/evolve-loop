package router

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/policy"
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

// FailureRouteFromPolicy is the policy→router fold (Phase 4a). The
// always_learn=false downgrade applies to the DEFAULT route only — an
// explicitly written audit_fail_routes_to wins (explicit beats derived).
func TestFailureRouteFromPolicy(t *testing.T) {
	f := func(b bool) *bool { return &b }
	cases := []struct {
		name  string
		floor *policy.FailureFloor
		want  string
	}{
		{"absent floor keeps legacy enable-chain", nil, ""},
		{"empty block routes the default", &policy.FailureFloor{}, "retrospective"},
		{"always_learn=false folds the default to memo", &policy.FailureFloor{AlwaysLearn: f(false)}, "memo"},
		{"explicit retrospective beats the always_learn fold", &policy.FailureFloor{AlwaysLearn: f(false), AuditFailRoutesTo: "retrospective"}, "retrospective"},
		{"explicit memo stands with always_learn=false", &policy.FailureFloor{AlwaysLearn: f(false), AuditFailRoutesTo: "memo"}, "memo"},
		{"explicit memo stands alone", &policy.FailureFloor{AuditFailRoutesTo: "memo"}, "memo"},
		{"typo route alone falls back to the default", &policy.FailureFloor{AuditFailRoutesTo: "retro"}, "retrospective"},
		{"typo route with always_learn=false folds to memo", &policy.FailureFloor{AlwaysLearn: f(false), AuditFailRoutesTo: "retro"}, "memo"},
	}
	for _, c := range cases {
		got := FailureRouteFromPolicy(policy.Policy{FailureFloor: c.floor})
		if got != c.want {
			t.Errorf("%s: FailureRouteFromPolicy = %q, want %q", c.name, got, c.want)
		}
	}
}
