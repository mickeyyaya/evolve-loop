package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestAdvisorDispatch_FallsBackWhenFamilyBenched(t *testing.T) {
	benched := map[string]bool{llmroute.Family("codex-tmux"): true}
	rd, err := resolveRouterDispatchHealthy(advisorRouter(t, t.TempDir(), policy.RouterPolicy{CLI: "codex-tmux"}), "", decisionPlan, benched)
	if err != nil {
		t.Fatal(err)
	}
	if !rd.healthy || rd.cli != "claude-tmux" {
		t.Errorf("benched primary family must fall back to claude-tmux (ok); got cli=%q ok=%v", rd.cli, rd.healthy)
	}
}

func TestAdvisorDispatch_CircuitBreakerAfterRepeatedFailure(t *testing.T) {
	benched := map[string]bool{llmroute.Family("codex-tmux"): true, "claude": true}
	if rd, _ := resolveRouterDispatchHealthy(advisorRouter(t, t.TempDir(), policy.RouterPolicy{CLI: "codex-tmux"}), "", decisionPlan, benched); rd.healthy {
		t.Error("primary + claude fallback both benched must signal !ok (degrade to static)")
	}
}

func TestResolveRouterDispatch_PerDecisionType(t *testing.T) {
	dir := t.TempDir() // no profile file ⇒ base = claude-tmux/opus
	rc := policy.RouterPolicy{}

	for _, dt := range []routerDecisionType{decisionPlan, decisionRePlan, decisionPropose, decisionJudge} {
		if rd, _ := resolveRouterDispatchHealthy(advisorRouter(t, dir, rc), "", dt, nil); rd.cli != "claude-tmux" || rd.model != "opus" {
			t.Errorf("decision %d default = (%s,%s), want (claude-tmux,opus) — must be no-op", dt, rd.cli, rd.model)
		}
	}

	rc.PlanModel = "opus-deep"
	if m := modelFor(t, advisorRouter(t, dir, rc), decisionPlan); m != "opus-deep" {
		t.Errorf("plan model = %q, want opus-deep", m)
	}
	if m := modelFor(t, advisorRouter(t, dir, rc), decisionRePlan); m != "opus-deep" {
		t.Errorf("replan model = %q, want opus-deep (a deep decision)", m)
	}
	if m := modelFor(t, advisorRouter(t, dir, rc), decisionPropose); m != "opus" {
		t.Errorf("propose model = %q, want base opus (PLAN_MODEL must not affect propose)", m)
	}

	rc.ProposeModel = "haiku"
	if m := modelFor(t, advisorRouter(t, dir, rc), decisionPropose); m != "haiku" {
		t.Errorf("propose model = %q, want haiku", m)
	}
	if m := modelFor(t, advisorRouter(t, dir, rc), decisionJudge); m != "haiku" {
		t.Errorf("judge model = %q, want haiku (a fast decision)", m)
	}
	if m := modelFor(t, advisorRouter(t, dir, rc), decisionPlan); m != "opus-deep" {
		t.Errorf("plan model = %q, want opus-deep (PROPOSE_MODEL must not affect plan)", m)
	}
}

func modelFor(t *testing.T, r *cliroute.Router, dt routerDecisionType) string {
	t.Helper()
	rd, err := resolveRouterDispatchHealthy(r, "", dt, nil)
	if err != nil {
		t.Fatal(err)
	}
	return rd.model
}
