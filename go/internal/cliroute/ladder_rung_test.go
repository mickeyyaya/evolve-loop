package cliroute_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func pinnedBuilderRouter(t *testing.T, rule policy.AgentRule) *cliroute.Router {
	t.Helper()
	rule.CLI, rule.Model = agyClaude, "balanced"
	return syntheticRouter(t, policy.CLIRouting{CLIs: agyClaude, Default: agyClaude, Agents: map[string]policy.AgentRule{"builder": rule}}, cliroute.Host{})
}

func builderRequest(ov llmroute.Overlay) cliroute.Request {
	return cliroute.Request{Agent: "builder", Phase: "build", DefaultModel: "balanced", Overlay: ov}
}

func traced(d cliroute.Decision, words ...string) bool {
	trace := strings.Join(d.Trace, "\n")
	for _, w := range words {
		if !strings.Contains(trace, w) {
			return false
		}
	}
	return true
}

func TestResolve_ThePinIsTheBaseRungAndOnlyTheLadderRaisesIt(t *testing.T) {
	r := pinnedBuilderRouter(t, policy.AgentRule{})

	if d := mustResolve(t, r, builderRequest(llmroute.Overlay{})); d.Plan.Model != "balanced" {
		t.Fatalf("first try model %q, want the pinned base balanced", d.Plan.Model)
	}
	if d := mustResolve(t, r, builderRequest(llmroute.Overlay{LadderTier: "deep"})); d.Plan.Model != "deep" || d.Plan.Tiers[0] != "deep" {
		t.Fatalf("ladder rung deep gave model %q tiers %v, want deep", d.Plan.Model, d.Plan.Tiers)
	}
	d := mustResolve(t, r, builderRequest(llmroute.Overlay{Tier: "deep"}))
	if d.Plan.Model != "balanced" || !traced(d, "advisor tier deep kept off") {
		t.Fatalf("advisor overlay deep gave model %q trace %v, want it kept off the pin", d.Plan.Model, d.Trace)
	}
}

func TestResolve_ALadderRungAboveTheBoundsIsClampedAndReported(t *testing.T) {
	r := pinnedBuilderRouter(t, policy.AgentRule{MaxTier: "deep", MaxEffort: "medium"})

	d := mustResolve(t, r, builderRequest(llmroute.Overlay{LadderTier: "top", Effort: "xhigh"}))
	if d.Plan.Model != "deep" || d.Plan.Effort != "medium" {
		t.Fatalf("plan model %q effort %q, want deep and medium", d.Plan.Model, d.Plan.Effort)
	}
	if !traced(d, "ladder tier top clamped to max_tier deep", "ladder effort xhigh clamped to max_effort medium") {
		t.Fatalf("trace %v does not report both clamps", d.Trace)
	}
}

func TestResolve_UnsetLadderBoundsAreDeepAndHigh(t *testing.T) {
	r := pinnedBuilderRouter(t, policy.AgentRule{})

	d := mustResolve(t, r, builderRequest(llmroute.Overlay{LadderTier: "top", Effort: "max"}))
	if d.Plan.Model != "deep" || d.Plan.Effort != "high" {
		t.Fatalf("plan model %q effort %q, want the defaults deep and high", d.Plan.Model, d.Plan.Effort)
	}
}

func TestResolve_AnExplicitBoundLetsTheLadderReachIt(t *testing.T) {
	r := pinnedBuilderRouter(t, policy.AgentRule{MaxTier: "top", MaxEffort: "xhigh"})

	d := mustResolve(t, r, builderRequest(llmroute.Overlay{LadderTier: "top", Effort: "xhigh"}))
	if d.Plan.Model != "top" || d.Plan.Effort != "xhigh" || traced(d, "clamped") {
		t.Fatalf("plan model %q effort %q trace %v, want top and xhigh unclamped", d.Plan.Model, d.Plan.Effort, d.Trace)
	}
}

func TestResolve_AFamilyFloorSeatStaysOnItsFloorWhenTheLadderRaisesIt(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{CLIs: agyClaude, Default: agyClaude}, cliroute.Host{})

	d := mustResolve(t, r, cliroute.Request{Agent: "tdd-engineer", Phase: "tdd", Overlay: llmroute.Overlay{LadderTier: "deep"}})
	if d.Plan.Model != "deep" {
		t.Fatalf("model %q, want the ladder rung deep", d.Plan.Model)
	}
	for _, family := range families(d.Plan.Candidates) {
		if family != "claude" {
			t.Fatalf("a floor seat left claude on a ladder raise: %v", d.Plan.Candidates)
		}
	}
}

func TestCompile_AnUnknownLadderBoundIsAFinding(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: agyClaude, Default: agyClaude,
		Agents: map[string]policy.AgentRule{"builder": {CLI: agyClaude, MaxTier: "huge", MaxEffort: "turbo"}},
	})

	requireFinding(t, findings, "cli_routing.agents.builder.max_tier", cliroute.SeverityError, "huge")
	requireFinding(t, findings, "cli_routing.agents.builder.max_effort", cliroute.SeverityError, "turbo")
}

func TestCompile_ABoundsOnlyRuleNeedsNoChain(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: agyClaude, Default: agyClaude,
		Agents: map[string]policy.AgentRule{"builder": {MaxTier: "top"}},
	})

	if errs := errorFindings(findings); len(errs) > 0 {
		t.Fatalf("a max_tier-only rule is valid, got %+v", errs)
	}
}

func TestResolve_TheLegacyPathRaisesToTheLadderRungWithinTheDefaultBounds(t *testing.T) {
	r := mustRouter(t, cliroute.Table{}, cliroute.Host{LookPath: everyBinary()})

	d := mustResolve(t, r, cliroute.Request{Agent: "scout", DefaultModel: "balanced", Overlay: llmroute.Overlay{LadderTier: "top", Effort: "max"}})
	if d.Plan.Model != "deep" || d.Plan.Effort != "high" {
		t.Fatalf("legacy plan model %q effort %q, want the ladder rung clamped to deep and high", d.Plan.Model, d.Plan.Effort)
	}
}

func TestResolve_ABoundOnAPhaseAliasBoundsItsAgent(t *testing.T) {
	r := syntheticRouter(t, policy.CLIRouting{CLIs: agyClaude, Default: agyClaude, Agents: map[string]policy.AgentRule{"build": {CLI: agyClaude, MaxTier: "balanced"}}}, cliroute.Host{})

	d := mustResolve(t, r, builderRequest(llmroute.Overlay{LadderTier: "deep"}))
	if d.Plan.Model != "balanced" || !traced(d, "clamped to max_tier balanced") {
		t.Fatalf("model %q trace %v, want agents.build.max_tier to bound the builder", d.Plan.Model, d.Trace)
	}
	findings := compileSynthetic(t, policy.CLIRouting{CLIs: agyClaude, Default: agyClaude, Agents: map[string]policy.AgentRule{"build": {CLI: agyClaude, MaxTier: "huge"}}})
	requireFinding(t, findings, "cli_routing.agents.build.max_tier", cliroute.SeverityError, "huge")
}

func TestResolve_ALegacyPinKeepsTheLadderOff(t *testing.T) {
	table := mustCompile(t, policy.Policy{Pins: map[string]policy.Pin{"scout": {CLI: "claude", Model: "balanced"}}}, syntheticCatalog(), syntheticProfiles(t))
	r := mustRouter(t, table, cliroute.Host{LookPath: everyBinary()})

	d := mustResolve(t, r, cliroute.Request{Agent: "scout", Phase: "scout", DefaultModel: "balanced", Overlay: llmroute.Overlay{LadderTier: "deep"}})
	if d.Plan.Model != "balanced" || d.Rule != cliroute.RuleLegacyPin {
		t.Fatalf("model %q rule %q, want the legacy pin to hold balanced", d.Plan.Model, d.Rule)
	}
}
