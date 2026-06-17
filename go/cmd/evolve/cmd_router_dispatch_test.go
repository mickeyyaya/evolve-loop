package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

// TestAdvisorDispatch_FallsBackWhenFamilyBenched pins WS6-S2 (ADR-0052): when the
// router's resolved CLI family is benched (the cli-health circuit breaker), the
// dispatch falls back to the universal claude family — keeping the advisor alive
// on a healthy footing rather than failing outright.
func TestAdvisorDispatch_FallsBackWhenFamilyBenched(t *testing.T) {
	t.Setenv("EVOLVE_ROUTER_CLI", "codex-tmux") // primary family = codex
	benched := map[string]bool{llmroute.Family("codex-tmux"): true}
	cli, _, ok := resolveRouterDispatchHealthy(t.TempDir(), decisionPlan, benched)
	if !ok || cli != "claude-tmux" {
		t.Errorf("benched primary family must fall back to claude-tmux (ok); got cli=%q ok=%v", cli, ok)
	}
}

// TestAdvisorDispatch_CircuitBreakerAfterRepeatedFailure pins the breaker tail:
// when the primary family AND the claude fallback are both benched (every family
// down), the resolver signals !ok and the caller degrades to the static spine.
func TestAdvisorDispatch_CircuitBreakerAfterRepeatedFailure(t *testing.T) {
	t.Setenv("EVOLVE_ROUTER_CLI", "codex-tmux")
	benched := map[string]bool{llmroute.Family("codex-tmux"): true, "claude": true}
	if _, _, ok := resolveRouterDispatchHealthy(t.TempDir(), decisionPlan, benched); ok {
		t.Error("primary + claude fallback both benched must signal !ok (degrade to static)")
	}
}

// TestResolveRouterDispatch_PerDecisionType pins WS6-S1 (ADR-0052, optional
// multi-model): resolveRouterDispatchFor returns the BASE dispatch for every
// decision type by default (strictly no-op, single-model), and applies per-type
// model overrides — EVOLVE_ROUTER_PLAN_MODEL to the DEEP decisions (plan/replan),
// EVOLVE_ROUTER_PROPOSE_MODEL to the FAST ones (propose/judge) — without
// crosstalk (D2: deep for plan/replan, fast for propose/judge).
func TestResolveRouterDispatch_PerDecisionType(t *testing.T) {
	dir := t.TempDir() // no profile file ⇒ base = claude-tmux/opus

	// Default: every decision type returns the base value (no-op).
	for _, dt := range []routerDecisionType{decisionPlan, decisionRePlan, decisionPropose, decisionJudge} {
		if cli, model := resolveRouterDispatchFor(dir, dt); cli != "claude-tmux" || model != "opus" {
			t.Errorf("decision %d default = (%s,%s), want (claude-tmux,opus) — must be no-op", dt, cli, model)
		}
	}

	// PLAN_MODEL overrides the DEEP decisions only.
	t.Setenv("EVOLVE_ROUTER_PLAN_MODEL", "opus-deep")
	if _, m := resolveRouterDispatchFor(dir, decisionPlan); m != "opus-deep" {
		t.Errorf("plan model = %q, want opus-deep", m)
	}
	if _, m := resolveRouterDispatchFor(dir, decisionRePlan); m != "opus-deep" {
		t.Errorf("replan model = %q, want opus-deep (a deep decision)", m)
	}
	if _, m := resolveRouterDispatchFor(dir, decisionPropose); m != "opus" {
		t.Errorf("propose model = %q, want base opus (PLAN_MODEL must not affect propose)", m)
	}

	// PROPOSE_MODEL overrides the FAST decisions only.
	t.Setenv("EVOLVE_ROUTER_PROPOSE_MODEL", "haiku")
	if _, m := resolveRouterDispatchFor(dir, decisionPropose); m != "haiku" {
		t.Errorf("propose model = %q, want haiku", m)
	}
	if _, m := resolveRouterDispatchFor(dir, decisionJudge); m != "haiku" {
		t.Errorf("judge model = %q, want haiku (a fast decision)", m)
	}
	if _, m := resolveRouterDispatchFor(dir, decisionPlan); m != "opus-deep" {
		t.Errorf("plan model = %q, want opus-deep (PROPOSE_MODEL must not affect plan)", m)
	}
}
