package bridge

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func effortTableOf(block policy.CLIRouting) policy.EffortTable {
	return policy.Policy{CLIRouting: &block}.Efforts()
}

func TestLaunchIntentFor_EffortPrecedence(t *testing.T) {
	table := effortTableOf(policy.CLIRouting{
		Tiers:  map[string]policy.TierRule{"deep": {Effort: "high"}},
		Agents: map[string]policy.AgentRule{"scout": {Effort: "low"}, "audit": {Effort: "xhigh"}},
	})
	cases := []struct {
		name, model, profile, agent, want string
	}{
		{"agent-by-profile-name-wins", "deep", "scout", "scout", "low"},
		{"a-phase-label-is-not-an-agent-key", "deep", "auditor", "audit", "high"},
		{"tier-when-the-agent-has-none", "deep", "builder", "build", "high"},
		{"default-fast", "fast", "builder", "build", "low"},
		{"default-balanced", "balanced", "builder", "build", "medium"},
		{"default-top", "top", "builder", "build", "medium"},
		{"legacy-alias-resolves-to-its-tier", "opus", "builder", "build", "high"},
		{"model-id-keeps-its-own-effort", "Claude Sonnet 5.5 (High)", "builder", "build", ""},
		{"model-id-still-takes-an-agent-effort", "Claude Sonnet 5.5 (High)", "scout", "scout", "low"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := launchIntentFor(&Config{Model: tc.model, Agent: tc.agent}, Profile{Name: tc.profile}, table).Effort
			if got != tc.want {
				t.Errorf("launchIntentFor(model=%q agent=%q).Effort = %q, want %q", tc.model, tc.agent, got, tc.want)
			}
		})
	}
}
