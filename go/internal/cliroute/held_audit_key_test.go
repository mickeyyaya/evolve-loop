package cliroute_test

import (
	"reflect"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func heldAuditProfiles() *profiles.Loader {
	return profiles.NewFromFS(fstest.MapFS{
		"adversarial-review.json": {Data: []byte(`{"name":"adversarial-review","cli":"claude-tmux","allowed_clis":["claude"],"model_tier_default":"deep"}`)},
	})
}

func heldAuditBlock(extra func(*policy.CLIRouting)) policy.CLIRouting {
	block := policy.CLIRouting{
		CLIs: []string{"agy", "agy-claude", "claude"}, Default: []string{"claude", "agy", "agy-claude"},
		Floors: policy.Floors{"claude": {"adversarial-review"}},
		Agents: map[string]policy.AgentRule{cliroute.HeldAuditKey: {CLI: []string{"agy", "claude"}, Model: "deep"}},
	}
	if extra != nil {
		extra(&block)
	}
	return block
}

func heldAuditRouter(t *testing.T, block policy.CLIRouting) *cliroute.Router {
	t.Helper()
	return mustBuild(t, cliroute.Setup{Policy: routingPolicy(block), Profiles: heldAuditProfiles(), Host: cliroute.Host{LookPath: everyBinary()}})
}

func TestHeldAuditKey_ResolvesItsOwnChainWithNoFloor(t *testing.T) {
	t.Parallel()
	r := heldAuditRouter(t, heldAuditBlock(nil))

	d := mustResolve(t, r, cliroute.Request{Agent: cliroute.HeldAuditKey, DefaultModel: "deep", NeedsTools: true})

	if !slices.Contains(d.Plan.Candidates, "agy-tmux") || !slices.Contains(d.Plan.Candidates, "claude-tmux") {
		t.Errorf("candidates = %v, want agy and claude on the held-audit chain", d.Plan.Candidates)
	}
	if got := r.FloorFamily(cliroute.HeldAuditKey); got != "" {
		t.Errorf("FloorFamily(held key) = %q, want no floor", got)
	}
	if d.Plan.Model != "deep" {
		t.Errorf("model = %q, want the deep tier of the policy entry", d.Plan.Model)
	}
}

func TestHeldAuditKey_TheFloorOfAdversarialReviewStaysUnchanged(t *testing.T) {
	t.Parallel()
	with := heldAuditRouter(t, heldAuditBlock(nil))
	without := heldAuditRouter(t, heldAuditBlock(func(b *policy.CLIRouting) { b.Agents = nil }))
	req := cliroute.Request{Agent: "adversarial-review", DefaultModel: "deep", NeedsTools: true}

	a, b := mustResolve(t, with, req), mustResolve(t, without, req)

	if !reflect.DeepEqual(a.Plan.Candidates, b.Plan.Candidates) || !reflect.DeepEqual(a.Allowed, b.Allowed) {
		t.Errorf("adversarial-review with the held key %v/%v, without %v/%v: the key must not change the floor route", a.Plan.Candidates, a.Allowed, b.Plan.Candidates, b.Allowed)
	}
	if !reflect.DeepEqual(a.Allowed, []string{"claude"}) || with.FloorFamily("adversarial-review") != "claude" {
		t.Errorf("allowed = %v floor = %q, want the claude floor kept", a.Allowed, with.FloorFamily("adversarial-review"))
	}
}

func TestHeldAuditKey_AnotherUnknownAgentKeyIsStillACompileError(t *testing.T) {
	t.Parallel()
	block := heldAuditBlock(func(b *policy.CLIRouting) {
		b.Agents = map[string]policy.AgentRule{"held-proposal-audit-2": {CLI: []string{"agy"}}}
	})

	_, findings := cliroute.Compile(routingPolicy(block), nil, heldAuditProfiles())

	requireFinding(t, findings, "cli_routing.agents.held-proposal-audit-2", cliroute.SeverityError, "neither a tracked profile nor a phase")
}

func TestHeldAuditKey_ACeilingThatEmptiesItsChainIsAFindingAtCompileTime(t *testing.T) {
	t.Parallel()
	block := heldAuditBlock(func(b *policy.CLIRouting) {
		b.Floors, b.AfterChain = policy.Floors{}, "stop"
		b.Tiers = map[string]policy.TierRule{"deep": {CLIs: []string{"agy-claude"}}, "balanced": {CLIs: []string{"agy-claude"}}, "fast": {CLIs: []string{"agy-claude"}}}
		b.Agents = map[string]policy.AgentRule{cliroute.HeldAuditKey: {CLI: []string{"claude"}, Model: "deep"}}
	})

	_, findings := cliroute.Compile(routingPolicy(block), nil, heldAuditProfiles())

	requireFinding(t, findings, "agent."+cliroute.HeldAuditKey+".tier_ceiling", cliroute.SeverityError)
}

func TestHeldAuditKey_AFloorOnTheKeyThatADeepCeilingExcludesIsAFinding(t *testing.T) {
	t.Parallel()
	block := heldAuditBlock(func(b *policy.CLIRouting) {
		b.Floors = policy.Floors{"claude": {"adversarial-review", cliroute.HeldAuditKey}}
		b.Tiers = map[string]policy.TierRule{"deep": {CLIs: []string{"agy"}}}
	})

	_, findings := cliroute.Compile(routingPolicy(block), nil, heldAuditProfiles())

	requireFinding(t, findings, "cli_routing.tiers.deep", cliroute.SeverityError, "excludes claude", cliroute.HeldAuditKey)
}
