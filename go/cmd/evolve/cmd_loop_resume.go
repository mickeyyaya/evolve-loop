package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// runResumeBatch owns the resume-only protocol. A resume executes exactly one
// cycle from its checkpoint and maps that result to the batch exit contract.
func runResumeBatch(
	ctx context.Context,
	cfg loopConfig,
	orch loopCycleRunner,
	lifecycle inboxmover.LedgerAppender,
	signals *signalcenter.Center,
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
	// A resumed cycle — possibly a fleet lane's checkpoint, with its lane pin —
	// reaches the inbox lifecycle like every other root (F30 architecture
	// review M1): the failure walk for a FAIL (a cycle-level failure error
	// included, as at the cycle-run root), the planned-no-work hand-off for a
	// lane that answered for its scope.
	closeout := result
	var clf *core.ErrCycleLevelFailure
	if errors.As(err, &clf) {
		closeout.FinalVerdict = core.VerdictFAIL
	}
	if err == nil || clf != nil {
		if applied, cerr := closeoutCycleOutcome(closeout, cfg.ProjectRoot, cfg.EvolveDir, stderr, lifecycle, signals); cerr != nil {
			fmt.Fprintf(stderr, "evolve loop: resume: WARN: could not apply cycle %d %s to the inbox: %v\n", result.Cycle, applied, cerr)
		}
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
