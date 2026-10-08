package bridge

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestLaunchIntentFor_EffortPrecedence(t *testing.T) {
	tiers := policy.BridgePolicy{}.TierEfforts()
	cases := []struct {
		name  string
		model string
		prof  Profile
		want  string
	}{
		{"override-wins", "deep", Profile{EffortLevel: "medium", EffortOverrides: map[string]string{"deep": "xhigh"}}, "xhigh"},
		{"level-beats-tier-default", "deep", Profile{EffortLevel: "low"}, "low"},
		{"tier-default-fills-a-gap/fast", "fast", Profile{}, "low"},
		{"tier-default-fills-a-gap/balanced", "balanced", Profile{}, "medium"},
		{"tier-default-fills-a-gap/deep", "deep", Profile{}, "high"},
		{"tier-default-fills-a-gap/top", "top", Profile{}, "xhigh"},
		{"legacy-alias-resolves-to-its-tier", "opus", Profile{}, "high"},
		{"model-id-keeps-its-own-effort", "Claude Sonnet 5.5 (High)", Profile{}, ""},
		{"unclassifiable-model-gets-none", "gpt-5.6-sol", Profile{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := launchIntentFor(&Config{Model: tc.model}, tc.prof, tiers).Effort
			if got != tc.want {
				t.Errorf("launchIntentFor(model=%q).Effort = %q, want %q", tc.model, got, tc.want)
			}
		})
	}
}

func TestDepsWithDefaults_TierEffortCompiled(t *testing.T) {
	got := Deps{}.withDefaults().TierEffort
	if got["deep"] != "high" || got["balanced"] != "medium" {
		t.Fatalf("Deps{}.withDefaults().TierEffort = %v, want the compiled policy table", got)
	}
	kept := Deps{TierEffort: map[string]string{"deep": "medium"}}.withDefaults().TierEffort
	if kept["deep"] != "medium" {
		t.Fatalf("withDefaults replaced a caller table: deep = %q, want %q", kept["deep"], "medium")
	}
}
