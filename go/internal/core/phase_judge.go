package core

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// JudgeVerdict is the LLM-as-judge route-quality grade; Score is in [0,1], or the sentinel -1 when the judge has no opinion.
type JudgeVerdict struct {
	Score         float64  `json:"score"`
	Rationale     string   `json:"rationale"`
	MissingPhases []string `json:"missing_phases"`
}

var judgeNoOpinion = JudgeVerdict{Score: -1}

// PlanJudge is the optional LLM-as-judge that scores a routing plan against the cycle goal.
// See ADR-0052.
type PlanJudge struct {
	bridge Bridge
	cli    string
	model  string
}

// NewPlanJudge builds the judge over the given bridge on the fast tier.
func NewPlanJudge(bridge Bridge) *PlanJudge {
	return &PlanJudge{bridge: bridge, cli: "claude-tmux", model: "haiku"}
}

// GradePlan scores plan against the cycle goal and returns a JudgeVerdict, failing open to Score=-1 on any error.
func (j *PlanJudge) GradePlan(ctx context.Context, in router.RouteInput, plan *router.PhasePlan) JudgeVerdict {
	if j.bridge == nil || in.Workspace == "" || plan == nil {
		return judgeNoOpinion
	}
	profile := ""
	if in.ProjectRoot != "" {
		profile = filepath.Join(in.ProjectRoot, ".evolve", "profiles", "judge.json")
	}
	worktree := in.ActiveWorktree
	if worktree == "" {
		worktree = in.Workspace
	}
	artifactPath := filepath.Join(in.Workspace, "routing-judge.json")
	resp, err := j.bridge.Launch(ctx, BridgeRequest{
		CLI:          j.cli,
		Profile:      profile,
		Model:        j.model,
		Prompt:       j.composeJudgePrompt(in, plan, artifactPath),
		Workspace:    in.Workspace,
		Worktree:     worktree,
		ProjectRoot:  in.ProjectRoot,
		ArtifactPath: artifactPath,
		Completion:   "artifact",
		Agent:        "judge",
		Cycle:        in.Cycle,
		Env:          in.Env,
	})
	if err != nil {
		return judgeNoOpinion
	}
	return parseJudgeVerdict(resp.Stdout)
}

// composeJudgePrompt orders phases deterministically so the prompt prefix stays cache-friendly.
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

// parseJudgeVerdict takes the LAST balanced JSON object in stdout, since a
// brace inside the rationale must not be miscounted and the prompt's echoed
// example must not be mistaken for the answer.
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
