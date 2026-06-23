package core

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

// JudgeVerdict is the LLM-as-judge route-quality grade (ADR-0052 WS4-S3). It is
// advisory telemetry, NOT a trust boundary: Score is in [0,1] on a successful
// grade, or the sentinel -1 ("no opinion") when the judge could not produce a
// valid grade. A caller treats -1 as non-blocking.
type JudgeVerdict struct {
	Score         float64  `json:"score"`
	Rationale     string   `json:"rationale"`
	MissingPhases []string `json:"missing_phases"`
}

// judgeNoOpinion is the fail-open sentinel: no valid grade, caller treats it as
// non-blocking (the judge never gates or alters anything).
var judgeNoOpinion = JudgeVerdict{Score: -1}

// PlanJudge is the optional LLM-as-judge that scores an emitted routing plan
// against the cycle goal (ADR-0052 WS4-S3), behind EVOLVE_ROUTING_JUDGE and
// strictly off the build path. It is deliberately NOT a router.Proposer or
// router.Planner — it only READS a plan and EMITS a score, so router.Select can
// never wire it as a routing brain. That, plus dispatching under the non-router
// "judge" agent label, is the structural recursion guard: a judge call has no
// path back into planning, so it needs no mint denylist. Per D2 the judge is
// the FAST/cheap tier (deep reasoning is reserved for the confidence-critical
// Plan/RePlan).
type PlanJudge struct {
	bridge Bridge
	cli    string
	model  string
}

// NewPlanJudge builds the judge over the given bridge. The fallback cli/model is
// the FAST tier (D2: the judge is off the critical path; deep is reserved for
// Plan/RePlan). The composition root may override from a judge profile when it
// wires the judge live.
func NewPlanJudge(bridge Bridge) *PlanJudge {
	return &PlanJudge{bridge: bridge, cli: "claude-tmux", model: "haiku"}
}

// GradePlan scores plan against the cycle goal and returns a JudgeVerdict.
// FAIL-OPEN BY TYPE: the signature returns ONLY a value (no error), so a caller
// structurally cannot let a malformed grade block the cycle. Every failure path
// — nil bridge, empty workspace, nil plan, bridge error, unparseable/empty
// output, or an out-of-range score — funnels to the sentinel Score=-1. This
// inverts the advisor convention (PhaseAdvisor errors so the kernel clamp
// catches it); the judge has no kernel clamp behind it, so it fails open in the
// value itself.
func (j *PlanJudge) GradePlan(ctx context.Context, in router.RouteInput, plan *router.PhasePlan) JudgeVerdict {
	if j.bridge == nil || in.Workspace == "" || plan == nil {
		return judgeNoOpinion
	}
	profile := ""
	if in.ProjectRoot != "" {
		profile = filepath.Join(in.ProjectRoot, ".evolve", "profiles", "judge.json")
	}
	artifactPath := filepath.Join(in.Workspace, "routing-judge.json")
	resp, err := j.bridge.Launch(ctx, BridgeRequest{
		CLI:          j.cli,
		Profile:      profile,
		Model:        j.model,
		Prompt:       j.composeJudgePrompt(in, plan, artifactPath),
		Workspace:    in.Workspace,
		ArtifactPath: artifactPath, // single-sourced: the prompt instructs the SAME path the bridge watches
		Completion:   "artifact",
		Agent:        "judge", // NON-router label — the recursion guard (never re-enters planning)
		Cycle:        in.Cycle,
		Env:          in.Env,
	})
	if err != nil {
		return judgeNoOpinion
	}
	return parseJudgeVerdict(resp.Stdout)
}

// composeJudgePrompt renders the goal + the candidate plan's run-set and asks
// for a strict-JSON verdict written to the artifact path. Deterministic order ⇒
// prompt-prefix cache friendly.
func (j *PlanJudge) composeJudgePrompt(in router.RouteInput, plan *router.PhasePlan, artifactPath string) string {
	var b strings.Builder
	b.WriteString("You are the evolve-loop ROUTE-QUALITY JUDGE. Score how well the proposed phase plan ")
	b.WriteString("serves the cycle goal. Your score is ADVISORY telemetry — it never gates or alters the plan.\n\n")
	if g := truncateGoal(in.GoalText); g != "" {
		fmt.Fprintf(&b, "## Goal\n%s\n\n", g)
	}
	b.WriteString("## Proposed plan (phases that will RUN)\n")
	for _, e := range plan.Entries {
		if e.Run {
			fmt.Fprintf(&b, "- %s\n", e.Phase)
		}
	}
	fmt.Fprintf(&b, "\nWrite a strict-JSON verdict to %s (no prose, no fence):\n", artifactPath)
	b.WriteString(`{"score":<0..1>,"rationale":"<one sentence>","missing_phases":["<phase>",...]}`)
	b.WriteString("\n")
	return b.String()
}

// parseJudgeVerdict extracts the strict-JSON verdict from the judge output,
// reusing lastBalancedSpan (string-literal-aware, so a brace inside the
// rationale is not miscounted; takes the LAST object so the prompt's echoed
// example is not mistaken for the answer). UNLIKE parseProposal it never returns
// an error: any failure — no object, bad JSON, or a score outside [0,1] —
// yields the fail-open sentinel Score=-1.
func parseJudgeVerdict(stdout string) JudgeVerdict {
	start, end, ok := lastBalancedSpan(stdout, '{', '}')
	if !ok {
		return judgeNoOpinion
	}
	var v JudgeVerdict
	if err := json.Unmarshal([]byte(stdout[start:end+1]), &v); err != nil {
		return judgeNoOpinion
	}
	if v.Score < 0 || v.Score > 1 {
		return judgeNoOpinion
	}
	return v
}
