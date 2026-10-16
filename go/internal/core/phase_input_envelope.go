package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func readUpstreamBuildPlan(stage config.Stage, phase Phase, phaseEnables map[string]string, workspace string) string {
	if stage < config.StageAdvisory || phase != PhaseBuild || workspace == "" || phaseEnables["build-planner"] != "on" {
		return ""
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "build-plan.md")); err == nil {
		return string(data)
	}
	return ""
}

func (cr *cycleRun) assemblePhaseIO(phase Phase, phaseWorktree string, phaseCtx map[string]string) phaseio.PhaseInput {
	sig, err := router.Digest(cr.cs.WorkspacePath, cr.cs.CompletedPhases)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phaseio shadow digest failed for %s: %v\n", phase, err)
		if cr.o.cfg.PhaseIO >= config.StageEnforce {
			return cr.buildPhaseInput(phase, phaseWorktree, phaseCtx, phaseio.Handoffs{})
		}
		return phaseio.PhaseInput{}
	}
	cr.emitPhaseIOShadowWithSig(phase, phaseCtx, sig)
	if cr.o.cfg.PhaseIO >= config.StageEnforce {
		return cr.buildPhaseInput(phase, phaseWorktree, phaseCtx, router.HandoffsFromSignals(sig))
	}
	return phaseio.PhaseInput{}
}

func (cr *cycleRun) buildPhaseInput(phase Phase, phaseWorktree string, phaseCtx map[string]string, up phaseio.Handoffs) phaseio.PhaseInput {
	return phaseio.NewPhaseInput(phaseio.PhaseInputInit{
		Cycle:         cr.cycle,
		RunID:         cr.cs.RunID,
		GoalHash:      cr.req.GoalHash,
		ProjectRoot:   cr.req.ProjectRoot,
		Workspace:     cr.cs.WorkspacePath,
		Worktree:      phaseWorktree,
		Phase:         string(phase),
		PreviousPhase: string(cr.current),
		Env:           cr.envSnap,
		Upstream:      up,
		CycleInputs:   assembleCycleInputs(phaseCtx),
		Error:         assembleErrorContext(phaseCtx),
	})
}
