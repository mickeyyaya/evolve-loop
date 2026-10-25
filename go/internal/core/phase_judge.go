package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type JudgeVerdict struct {
	Score         float64  `json:"score"`
	Rationale     string   `json:"rationale"`
	MissingPhases []string `json:"missing_phases"`
}

var judgeNoOpinion = JudgeVerdict{Score: -1}

type PlanJudge struct {
	bridge  Bridge
	cli     string
	model   string
	signals *signalcenter.Center
}

type PlanJudgeOption func(*PlanJudge)

func WithPlanJudgeSignals(c *signalcenter.Center) PlanJudgeOption {
	return func(j *PlanJudge) { j.signals = c }
}

func NewPlanJudge(bridge Bridge, opts ...PlanJudgeOption) *PlanJudge {
	seat := policy.SeatDefault(policy.SeatPlanJudge)
	j := &PlanJudge{bridge: bridge, cli: seat.CLI, model: seat.Model}
	for _, o := range opts {
		o(j)
	}
	return j
}

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
	flow := phasecontract.MustFlowDecision(phasecontract.FlowSeatJudge)
	artifactPath := filepath.Join(in.Workspace, flow.ArtifactName)
	if err := flow.Clear(artifactPath); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN plan judge: %v — no opinion\n", err)
		return judgeNoOpinion
	}
	launched := time.Now()
	resp, err := LaunchWithFreshSession(ctx, j.bridge, BridgeRequest{
		CLI:          j.cli,
		Profile:      profile,
		Model:        j.model,
		Prompt:       j.composeJudgePrompt(in, plan, artifactPath),
		Workspace:    in.Workspace,
		Worktree:     worktree,
		ProjectRoot:  in.ProjectRoot,
		ArtifactPath: artifactPath,
		Completion:   CompletionArtifact,
		Agent:        "judge",
		Cycle:        in.Cycle,
		Env:          in.Env,
	})
	if err != nil {
		return judgeNoOpinion
	}
	cell := phasecontract.FlowCell{Cycle: in.Cycle, Phase: in.Current, Origin: "PlanJudge.GradePlan", Path: artifactPath, NotBefore: launched}
	return decodeJudgeVerdict(flow.Resolve(in.ProjectRoot, cell, advisor.StdoutSpan(resp.Stdout, '{', '}'), j.signals))
}

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

func decodeJudgeVerdict(raw []byte) JudgeVerdict {
	var v JudgeVerdict
	if err := json.Unmarshal(raw, &v); err != nil {
		return judgeNoOpinion
	}
	if v.Score < 0 || v.Score > 1 {
		return judgeNoOpinion
	}
	return v
}
