package cliroute_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestResolve_ALadderEffortReachesThePlanBesideAnAgentModel(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{
		CLIs: agyClaude, Default: agyClaude,
		Agents: map[string]policy.AgentRule{"scout": {CLI: []string{"claude"}, Model: "balanced"}},
	}, cliroute.Host{})

	d := mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout", Overlay: llmroute.Overlay{Effort: "high"}})
	if d.Plan.Effort != "high" || d.Plan.Model != "balanced" {
		t.Fatalf("plan effort %q model %q, want the ladder effort high at the pinned balanced tier", d.Plan.Effort, d.Plan.Model)
	}
	if d = mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout"}); d.Plan.Effort != "" {
		t.Fatalf("plan effort %q without a ladder effort, want empty", d.Plan.Effort)
	}
}

func TestResolve_TheLegacyPathCarriesALadderEffort(t *testing.T) {
	r := mustRouter(t, cliroute.Table{}, cliroute.Host{LookPath: everyBinary()})

	d := mustResolve(t, r, cliroute.Request{Agent: "scout", DefaultModel: "balanced", Overlay: llmroute.Overlay{Effort: "high"}})
	if d.Plan.Effort != "high" || d.Plan.Model != "balanced" {
		t.Fatalf("legacy plan effort %q model %q, want high at balanced", d.Plan.Effort, d.Plan.Model)
	}
}

func TestResolve_ALegacyPinKeepsTheLadderEffortOff(t *testing.T) {
	table := mustCompile(t, policy.Policy{Pins: map[string]policy.Pin{"scout": {CLI: "claude", Model: "balanced"}}}, syntheticCatalog(), syntheticProfiles(t))
	r := mustRouter(t, table, cliroute.Host{LookPath: everyBinary()})

	d := mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout", DefaultModel: "balanced", Overlay: llmroute.Overlay{Effort: "high"}})
	if d.Rule != cliroute.RuleLegacyPin || d.Plan.Effort != "" {
		t.Fatalf("rule %q effort %q, want the legacy pin to keep every overlay off, the ladder effort too", d.Rule, d.Plan.Effort)
	}
}
