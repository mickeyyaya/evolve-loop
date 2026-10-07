package cliroute_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute/cliroutetest"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type operatorFixture struct {
	router *cliroute.Router
	agents []string
	profs  cliroute.ProfileSource
}

func newOperatorFixture(t *testing.T) operatorFixture {
	t.Helper()
	profileDir, agents := cliroutetest.TrackedProfiles(t)
	profs := profiles.NewFromDir(profileDir)
	table := mustCompile(t, operatorTable(), realCatalog(t), profs)
	return operatorFixture{router: mustRouter(t, table, cliroute.Host{LookPath: everyBinary()}), agents: agents, profs: profs}
}

func (f operatorFixture) requests(agent string) []cliroute.Request {
	var out []cliroute.Request
	for _, tier := range []string{"", "deep", "top"} {
		ov := llmroute.Overlay{Tier: tier}
		out = append(out,
			cliroute.Request{Agent: agent, Phase: cliroutetest.PhaseOf(agent), DefaultModel: "balanced", Overlay: ov},
			cliroute.Request{Agent: agent, DefaultModel: "balanced", Overlay: ov})
	}
	return out
}

func TestCompile_TheOperatorTableCompilesWithOnlyAcceptedWarnings(t *testing.T) {
	profileDir, _ := cliroutetest.TrackedProfiles(t)
	_, findings := cliroute.Compile(operatorTable(), realCatalog(t), profiles.NewFromDir(profileDir))
	if errs := errorFindings(findings); len(errs) > 0 {
		t.Fatalf("the operator's table must compile against the tracked profiles: %+v", errs)
	}
	requireFinding(t, findings, "cross_family_with.auditor+builder", cliroute.SeverityWarn, "claude")
}

func TestResolve_TheOperatorTableRoutesNoLaunchToCodex(t *testing.T) {
	f := newOperatorFixture(t)
	for _, agent := range f.agents {
		for _, req := range f.requests(agent) {
			d := mustResolve(t, f.router, req)
			for _, fam := range families(d.Plan.Candidates) {
				if fam == "codex" {
					t.Fatalf("%s (phase %q, tier %q) chain %v reaches codex", agent, req.Phase, req.Overlay.Tier, d.Plan.Candidates)
				}
			}
		}
	}
}

func TestResolve_TheOperatorTableKeepsTheFloorAgentsOnClaude(t *testing.T) {
	f := newOperatorFixture(t)
	for agent := range profiles.ClaudeFamilyFloor() {
		for _, req := range f.requests(agent) {
			d := mustResolve(t, f.router, req)
			for _, attempt := range walkEveryAttemptWalled(d.Plan) {
				if !strings.HasPrefix(attempt, "claude") {
					t.Fatalf("floor agent %s attempted %s (chain %v)", agent, attempt, d.Plan.Candidates)
				}
			}
			if !reflect.DeepEqual(d.Allowed, []string{"claude"}) {
				t.Fatalf("floor agent %s allowed %v, want [claude]", agent, d.Allowed)
			}
		}
	}
}

func TestResolve_DeepAndTopNeverRunOnAgy(t *testing.T) {
	f := newOperatorFixture(t)
	checked := 0
	for _, agent := range f.agents {
		for _, req := range f.requests(agent) {
			d := mustResolve(t, f.router, req)
			for _, attempt := range walkEveryAttemptWalled(d.Plan) {
				checked++
				if attempt == "agy-tmux@deep" || attempt == "agy-tmux@top" {
					t.Fatalf("%s (phase %q, tier %q) attempted %s; chain %v tiers %v", agent, req.Phase, req.Overlay.Tier, attempt, d.Plan.Candidates, d.Plan.Tiers)
				}
			}
		}
	}
	if checked < len(f.agents)*6 {
		t.Fatalf("only %d attempts walked — the corpus is too small to prove the ceiling", checked)
	}
}

func TestResolve_BuilderBalancedOnAgyEscalatesToAgyOwnedClaudeAtDeep(t *testing.T) {
	f := newOperatorFixture(t)
	builder, err := f.profs.Get("builder")
	if err != nil {
		t.Fatal(err)
	}
	d := mustResolve(t, f.router, cliroute.Request{Agent: "builder", Phase: "build", DefaultModel: "balanced"})
	if first := walkWith(d.Plan, nil).Attempts[0]; first != "agy-tmux@balanced" {
		t.Fatalf("the builder at its default tier runs on agy at balanced, first attempt %s (chain %v tiers %v)", first, d.Plan.Candidates, d.Plan.Tiers)
	}
	escalated := builder.ModelTierOverrides["m_complex_5plus_files"]
	if escalated != "deep" {
		t.Fatalf("builder.json model_tier_overrides.m_complex_5plus_files = %q; this test pins its deep escalation", escalated)
	}
	d = mustResolve(t, f.router, cliroute.Request{Agent: "builder", Phase: "build", DefaultModel: "balanced", Overlay: llmroute.Overlay{Tier: escalated}})
	if first := walkWith(d.Plan, nil).Attempts[0]; first != "agy-claude-tmux@deep" {
		t.Fatalf("a deep builder escalation runs Claude through agy first, first attempt %s (chain %v tiers %v)", first, d.Plan.Candidates, d.Plan.Tiers)
	}
}

func TestResolve_WalledClaudeModelsAtDeepStepDownNotToAgyDeep(t *testing.T) {
	f := newOperatorFixture(t)
	d := mustResolve(t, f.router, cliroute.Request{Agent: "builder", Phase: "build", DefaultModel: "balanced", Overlay: llmroute.Overlay{Tier: "deep"}})
	res := walkWith(d.Plan, map[string]int{"agy-claude-tmux@deep": 85, "claude-tmux@deep": 85})
	if want := []string{"agy-claude-tmux@deep", "claude-tmux@deep", "agy-tmux@balanced"}; !reflect.DeepEqual(res.Attempts, want) {
		t.Fatalf("attempts = %v, want %v", res.Attempts, want)
	}
	if res.Err != nil || res.CLI != "agy-tmux" || res.Tier != "balanced" {
		t.Fatalf("the step-down lands on agy at balanced: %+v", res)
	}
}
