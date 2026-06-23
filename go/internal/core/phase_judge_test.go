package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

// TestJudgeVerdict_Fields names the JudgeVerdict type directly (public-API
// coverage) and pins its zero/sentinel semantics: the -1 "no opinion" sentinel
// is distinct from a real [0,1] score.
func TestJudgeVerdict_Fields(t *testing.T) {
	t.Parallel()
	v := JudgeVerdict{Score: 0.5, Rationale: "ok", MissingPhases: []string{"bug-reproduction"}}
	if v.Score != 0.5 || v.Rationale != "ok" || len(v.MissingPhases) != 1 {
		t.Errorf("JudgeVerdict fields = %+v", v)
	}
	if (JudgeVerdict{Score: -1}).Score != -1 {
		t.Error("the -1 no-opinion sentinel must be representable")
	}
}

func judgeInput() router.RouteInput {
	return router.RouteInput{Workspace: "/tmp/ws", ProjectRoot: "/proj", Cycle: 9, GoalText: "fix the auth nil-panic"}
}

func samplePlan() *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "build", Run: true},
		{Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
}

// WS4-S3 (ADR-0052): GradePlan scores a routing plan against the goal and
// returns a typed verdict, dispatched as a NON-router agent on the fast tier.
func TestRouteQualityJudge_ScoresPlanAgainstGoal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		stdout    string
		wantScore float64
	}{
		{"clean json", `{"score":0.8,"rationale":"covers the bug path","missing_phases":[]}`, 0.8},
		{"fenced + prose", "Here is my grade:\n```json\n{\"score\":0.5,\"rationale\":\"ok\",\"missing_phases\":[\"bug-reproduction\"]}\n```\nDone.", 0.5},
		// Inclusive boundaries must be ACCEPTED (a future `>`→`>=` slip is caught here).
		{"boundary lower 0.0", `{"score":0,"rationale":"worst plan but valid"}`, 0},
		{"boundary upper 1.0", `{"score":1,"rationale":"perfect"}`, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			fb := &fakeBridge{stdout: c.stdout}
			v := NewPlanJudge(fb).GradePlan(context.Background(), judgeInput(), samplePlan())
			if v.Score != c.wantScore {
				t.Errorf("score=%v, want %v", v.Score, c.wantScore)
			}
		})
	}
	// missing_phases threads through, and dispatch uses the NON-router "judge"
	// label (the recursion guard).
	fb := &fakeBridge{stdout: `{"score":0.4,"rationale":"x","missing_phases":["bug-reproduction","plan-review"]}`}
	v := NewPlanJudge(fb).GradePlan(context.Background(), judgeInput(), samplePlan())
	if len(v.MissingPhases) != 2 {
		t.Errorf("missing_phases=%v, want 2", v.MissingPhases)
	}
	if fb.gotReq.Agent != "judge" {
		t.Errorf("Agent=%q, want judge (never the router label)", fb.gotReq.Agent)
	}
	// D2: the judge is the FAST/cheap tier (deep is reserved for Plan/RePlan), and
	// it uses the uniform artifact-completion contract — locks the dispatch shape
	// against drift (a flip to opus would defeat D2; a wrong artifact path is the
	// cycle-210 silent-timeout class the sibling advisor test pins).
	if fb.gotReq.CLI != "claude-tmux" {
		t.Errorf("CLI=%q, want claude-tmux", fb.gotReq.CLI)
	}
	if fb.gotReq.Model != "haiku" {
		t.Errorf("Model=%q, want haiku (D2: judge is fast/cheap, never deep)", fb.gotReq.Model)
	}
	if !strings.HasSuffix(fb.gotReq.ArtifactPath, "routing-judge.json") {
		t.Errorf("ArtifactPath=%q, want .../routing-judge.json", fb.gotReq.ArtifactPath)
	}
	if !strings.HasSuffix(fb.gotReq.Profile, "/.evolve/profiles/judge.json") {
		t.Errorf("Profile=%q, want .../.evolve/profiles/judge.json", fb.gotReq.Profile)
	}
	if fb.gotReq.Completion != "artifact" {
		t.Errorf("Completion=%q, want artifact", fb.gotReq.Completion)
	}
}

// WS4-S3: every failure path yields the fail-open sentinel Score=-1. The
// signature has NO error return, so a malformed grade structurally cannot block
// the cycle.
func TestRouteQualityJudge_FailOpenToMinusOne(t *testing.T) {
	t.Parallel()
	in, plan := judgeInput(), samplePlan()
	cases := []struct {
		name string
		j    *PlanJudge
	}{
		{"bridge error", NewPlanJudge(&fakeBridge{err: errors.New("boom")})},
		{"no json", NewPlanJudge(&fakeBridge{stdout: "I cannot grade this."})},
		{"malformed", NewPlanJudge(&fakeBridge{stdout: `{"score":}`})},
		{"empty", NewPlanJudge(&fakeBridge{stdout: ""})},
		{"score above range", NewPlanJudge(&fakeBridge{stdout: `{"score":1.5}`})},
		{"score below range", NewPlanJudge(&fakeBridge{stdout: `{"score":-0.5}`})},
		{"nil bridge", NewPlanJudge(nil)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if v := c.j.GradePlan(context.Background(), in, plan); v.Score != -1 {
				t.Errorf("%s: score=%v, want -1 (fail-open sentinel)", c.name, v.Score)
			}
		})
	}
	noWs := judgeInput()
	noWs.Workspace = ""
	if v := NewPlanJudge(&fakeBridge{stdout: `{"score":0.9}`}).GradePlan(context.Background(), noWs, plan); v.Score != -1 {
		t.Errorf("empty workspace: score=%v, want -1", v.Score)
	}
	if v := NewPlanJudge(&fakeBridge{stdout: `{"score":0.9}`}).GradePlan(context.Background(), in, nil); v.Score != -1 {
		t.Errorf("nil plan: score=%v, want -1", v.Score)
	}
}

// WS4-S3: the judge can NEVER trigger an advisor. Structural guard: a PlanJudge
// is not a routing brain, so it cannot be wired as a Proposer/Planner into
// LLMProposal/Select — closing the recursion path without depending on the
// WS1-S2 mint denylist (not yet built).
func TestRouteQualityJudge_RecursionGuarded(t *testing.T) {
	t.Parallel()
	var v interface{} = NewPlanJudge(&fakeBridge{stdout: `{"score":0.5}`})
	if _, ok := v.(router.Planner); ok {
		t.Error("PlanJudge must NOT implement router.Planner (it would become a routing brain)")
	}
	if _, ok := v.(router.Proposer); ok {
		t.Error("PlanJudge must NOT implement router.Proposer")
	}
	fb := &fakeBridge{stdout: `{"score":0.5,"rationale":"ok"}`}
	NewPlanJudge(fb).GradePlan(context.Background(), judgeInput(), samplePlan())
	// Positive form (not == "router"): an empty/unset Agent must also fail, so
	// deleting the Agent:"judge" literal in production is caught here.
	if fb.gotReq.Agent != "judge" {
		t.Error("the judge must dispatch under the NON-router judge label")
	}
}
