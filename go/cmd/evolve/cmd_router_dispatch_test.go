package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestAdvisorDispatch_FallsBackWhenFamilyBenched(t *testing.T) {
	benched := map[string]bool{llmroute.Family("codex-tmux"): true}
	cli, _, ok, err := resolveRouterDispatchHealthy(advisorRouter(t, t.TempDir(), policy.RouterPolicy{CLI: "codex-tmux"}), "", decisionPlan, benched)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || cli != "claude-tmux" {
		t.Errorf("benched primary family must fall back to claude-tmux (ok); got cli=%q ok=%v", cli, ok)
	}
}

func TestAdvisorDispatch_CircuitBreakerAfterRepeatedFailure(t *testing.T) {
	benched := map[string]bool{llmroute.Family("codex-tmux"): true, "claude": true}
	if _, _, ok, _ := resolveRouterDispatchHealthy(advisorRouter(t, t.TempDir(), policy.RouterPolicy{CLI: "codex-tmux"}), "", decisionPlan, benched); ok {
		t.Error("primary + claude fallback both benched must signal !ok (degrade to static)")
	}
}

func TestResolveRouterDispatch_PerDecisionType(t *testing.T) {
	dir := t.TempDir() // no profile file ⇒ base = claude-tmux/opus
	rc := policy.RouterPolicy{}

	for _, dt := range []routerDecisionType{decisionPlan, decisionRePlan, decisionPropose, decisionJudge} {
		if cli, model, _ := resolveRouterDispatchFor(advisorRouter(t, dir, rc), "", dt); cli != "claude-tmux" || model != "opus" {
			t.Errorf("decision %d default = (%s,%s), want (claude-tmux,opus) — must be no-op", dt, cli, model)
		}
	}

	rc.PlanModel = "opus-deep"
	if _, m, _ := resolveRouterDispatchFor(advisorRouter(t, dir, rc), "", decisionPlan); m != "opus-deep" {
		t.Errorf("plan model = %q, want opus-deep", m)
	}
	if _, m, _ := resolveRouterDispatchFor(advisorRouter(t, dir, rc), "", decisionRePlan); m != "opus-deep" {
		t.Errorf("replan model = %q, want opus-deep (a deep decision)", m)
	}
	if _, m, _ := resolveRouterDispatchFor(advisorRouter(t, dir, rc), "", decisionPropose); m != "opus" {
		t.Errorf("propose model = %q, want base opus (PLAN_MODEL must not affect propose)", m)
	}

	rc.ProposeModel = "haiku"
	if _, m, _ := resolveRouterDispatchFor(advisorRouter(t, dir, rc), "", decisionPropose); m != "haiku" {
		t.Errorf("propose model = %q, want haiku", m)
	}
	if _, m, _ := resolveRouterDispatchFor(advisorRouter(t, dir, rc), "", decisionJudge); m != "haiku" {
		t.Errorf("judge model = %q, want haiku (a fast decision)", m)
	}
	if _, m, _ := resolveRouterDispatchFor(advisorRouter(t, dir, rc), "", decisionPlan); m != "opus-deep" {
		t.Errorf("plan model = %q, want opus-deep (PROPOSE_MODEL must not affect plan)", m)
	}
}
