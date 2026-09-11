package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

type sequentialCycle struct {
	result     core.CycleResult
	lastBefore int
	lastAfter  int
	cycle      int
	workspace  string
}

func (b *loopBatchCoordinator) dispatchSequentialCycle() (sequentialCycle, batchDecision) {
	lastBefore, _ := readLastCycleNumber(context.Background(), b.deps.Storage)
	request := core.CycleRequest{
		ProjectRoot:           b.cfg.ProjectRoot,
		GoalHash:              b.cfg.GoalHash,
		Env:                   b.cycleEnv,
		Context:               b.cycleContext,
		DisableWorkspaceGuard: disableWorkspaceGuardForTest,
		BypassPolicy:          b.cfg.BypassPolicy,
	}
	result, err := b.orch.RunCycle(b.ctx, request)
	reapCycleSessions(b.cfg.ProjectRoot, result.Cycle, b.stderr)
	b.result.Cycles = append(b.result.Cycles, result)
	if b.ctx.Err() != nil {
		emitSignalStop(b.stdout, b.stderr, b.result, result.Cycle)
		return sequentialCycle{}, batchDecision{flow: batchReturn, exitCode: 130}
	}
	if err != nil {
		return sequentialCycle{}, b.handleCycleError(result, err)
	}
	lastAfter, _ := readLastCycleNumber(context.Background(), b.deps.Storage)
	return sequentialCycle{
		result:     result,
		lastBefore: lastBefore,
		lastAfter:  lastAfter,
		cycle:      result.Cycle,
		workspace:  cycleWorkspace(b.cfg.ProjectRoot, result.Cycle),
	}, batchDecision{flow: batchProceed}
}

func (b *loopBatchCoordinator) handleCycleError(result core.CycleResult, cycleErr error) batchDecision {
	var cycleFailure *core.ErrCycleLevelFailure
	if !errors.As(cycleErr, &cycleFailure) {
		b.result.StopReason = "error"
		fmt.Fprintf(b.stderr, "evolve loop: cycle %d: %v\n", result.Cycle, cycleErr)
		return batchDecision{flow: batchStopIterations}
	}

	fmt.Fprintf(b.stderr, "evolve loop: cycle %d: %v\n", result.Cycle, cycleErr)
	b.result.RecoverableFailures++
	workspace := cycleWorkspace(b.cfg.ProjectRoot, result.Cycle)
	classification := cycleclassify.Classify(workspace)
	_, recordErr := failurelog.Record(
		filepath.Join(b.cfg.EvolveDir, "state.json"),
		filepath.Join(b.cfg.ProjectRoot, ".evolve", "runs"),
		failurelog.RecordRequest{
			Cycle:          result.Cycle,
			Classification: string(classification.Class),
			ReportPath:     filepath.Join(workspace, "orchestrator-report.md"),
			Now:            time.Now().UTC(),
		},
	)
	if recordErr != nil && !errors.Is(recordErr, failurelog.ErrStateMissing) {
		fmt.Fprintf(b.stderr, "[loop] WARN: could not record cycle failure: %v\n", recordErr)
	}
	if _, err := cycleoutcome.ApplyFailure(cycleoutcome.FailureInputsFor(
		b.cfg.ProjectRoot, b.cfg.EvolveDir, workspace, result.Cycle, b.stderr,
	)); err != nil {
		fmt.Fprintf(b.stderr, "[loop] WARN: could not release cycle %d inbox claims: %v\n", result.Cycle, err)
	}
	if errors.Is(cycleErr, core.ErrAllFamiliesExhausted) {
		b.result.emitQuotaPause(b.cfg, result.Cycle, b.stdout, b.stderr)
		return batchDecision{flow: batchReturn, exitCode: 5}
	}
	return batchDecision{flow: batchNextIteration}
}
