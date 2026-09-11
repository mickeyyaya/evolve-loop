package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// runResumeBatch owns the resume-only protocol. A resume executes exactly one
// cycle from its checkpoint and maps that result to the batch exit contract.
func runResumeBatch(
	ctx context.Context,
	cfg loopConfig,
	orch loopCycleRunner,
	cycleEnv, cycleCtx map[string]string,
	lr *loopResult,
	stdout, stderr io.Writer,
) int {
	lr.Resumed = true
	rp, err := core.LoadResumeState(ctx, cfg.ProjectRoot, cfg.EvolveDir, core.ResumeOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "evolve loop: resume: %v\n", err)
		lr.StopReason = "error"
		lr.emitFatal(stdout, stderr, cfg, 0)
		return 2
	}
	fmt.Fprintf(stderr, "[resume] cycle=%d phase=%s reason=%s cost=$%.2f state=%s\n",
		rp.CycleID, rp.Phase, rp.Reason, rp.CostAtPause, rp.StatePath)

	// A fleet checkpoint must keep reading and writing the per-run state file
	// that discovery selected, including a second pause during this invocation.
	restoreState := core.ActivateResumeStatePath(rp, cfg.EvolveDir)
	defer restoreState()

	req := core.CycleRequest{
		ProjectRoot:           cfg.ProjectRoot,
		GoalHash:              cfg.GoalHash,
		Env:                   cycleEnv,
		Context:               cycleCtx,
		DisableWorkspaceGuard: disableWorkspaceGuardForTest,
		BypassPolicy:          cfg.BypassPolicy,
	}
	result, err := orch.RunCycleFromPhase(ctx, req, rp)
	reapCycleSessions(cfg.ProjectRoot, result.Cycle, stderr)
	lr.Cycles = append(lr.Cycles, result)

	if ctx.Err() != nil {
		emitSignalStop(stdout, stderr, lr, result.Cycle)
		return 130
	}
	if errors.Is(err, core.ErrAllFamiliesExhausted) {
		lr.emitQuotaPause(cfg, result.Cycle, stdout, stderr)
		return 5
	}
	if err != nil {
		lr.StopReason = "error"
		fmt.Fprintf(stderr, "evolve loop: resume cycle %d: %v\n", result.Cycle, err)
	} else if result.FinalVerdict == core.VerdictFAIL {
		lr.StopReason = "fail"
	} else {
		lr.StopReason = "resumed_complete"
	}
	if lr.StopReason == "error" || lr.StopReason == "fail" {
		lr.emitFatal(stdout, stderr, cfg, result.Cycle)
		return 2
	}

	finalizeCompletedCycle(cfg, stderr)
	lr.emit(stdout)
	return 0
}
