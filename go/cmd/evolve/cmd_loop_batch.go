package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type loopBatchCoordinator struct {
	ctx              context.Context
	cfg              loopConfig
	deps             orchDeps
	orch             loopCycleRunner
	cycleEnv         map[string]string
	cycleContext     map[string]string
	result           *loopResult
	workflow         policy.WorkflowConfig
	lastBeforeGCHook int
	stdout           io.Writer
	stderr           io.Writer
}

func (b *loopBatchCoordinator) run() int {
	cfg := b.cfg
	deps := b.deps
	lr := b.result
	lastBeforeGCHook := b.lastBeforeGCHook
	stdout := b.stdout
	stderr := b.stderr
	wc := b.workflow

	dc := loadDispatchConfig(cfg.EvolveDir)
	dispPolicy := resolveDispatchPolicy(dc.Policy, stderr)
	threshold := resolveCircuitBreakerThreshold(dc.RepeatThreshold)

	// Circuit-breaker state. PREV_RAN_CYCLE tracks the cycle number
	// returned by the most-recent RunCycle; SAME_CYCLE_STREAK counts
	// consecutive identical values. Trips at threshold.
	prevRanCycle := -1
	sameCycleStreak := 0

	// Consecutive-verdict-FAIL breaker.
	// Default 1 ⇒ stop on the first FAIL (pre-flag contract); >1 lets the
	// batch absorb isolated work-quality misses so a 3-PASS streak can form,
	// while the streak cap still halts a genuinely broken run.
	maxConsecutiveFails := wc.MaxConsecutiveFails
	consecutiveFails := 0

	// Advisor-decided cycle budget. Off ⇒ the operator's
	// --max-cycles governs (byte-identical to today). Enforce with no explicit
	// --max-cycles ⇒ the ceiling becomes the safety cap and per-cycle completion
	// (backlog drained) drives the early stop; advisory computes + logs the
	// would-stop without changing behavior.
	budgetStage := cyclebudget.ParseStage(wc.CycleBudget)
	effectiveMax := cfg.MaxCycles
	if budgetStage == cyclebudget.Enforce && !cfg.MaxCyclesExplicit {
		effectiveMax = wc.MaxCyclesCap
		fmt.Fprintf(stderr, "[loop] no --cycles: advisor-decided, completion-driven (stops when the backlog drains), safety cap=%d\n", effectiveMax)
	}

	// FLEET-AS-POLICY S2: the batch-start snapshot — wave 0's baseline. Every
	// iteration re-resolves the committed fleet block via
	// reloadFleetConfigAtWaveBoundary below (cycle 739), so operator
	// count/min_lanes directives committed mid-batch take effect at the next
	// wave without a lane-killing bounce. Count==1 (absent block, the default)
	// keeps iterations on the existing sequential path below — shouldRunWave
	// gates the wave branch off entirely, so no Supervisor is ever constructed.
	fleetCfg := loadFleetConfig(cfg.EvolveDir)
	for _, w := range fleetCfg.Warnings {
		fmt.Fprintf(stderr, "[loop] WARN: fleet: %s\n", w)
	}
	var waveBinPath string
	if shouldRunWave(fleetCfg) || shouldRunPool(fleetCfg) {
		if bp, err := os.Executable(); err == nil {
			waveBinPath = bp
		} else {
			fmt.Fprintf(stderr, "[loop] WARN: fleet: cannot resolve binary for fleet dispatch, staying sequential: %v\n", err)
			fleetCfg.Count = 1
		}
	}

	// Fleet work-supply-starvation observer (L3 leg of the
	// fleet-concurrency-respect architecture): ONE tracker held across the batch
	// loop so a starved streak spans waves. It advances only on the wave-ran path
	// below; a sequential (Count==1) batch never touches it.
	var starvationTracker fleet.StarvationTracker

	// Stall escalation (cmd_loop_goalstall.go): TWO counters over the running
	// goal, both held outside the loop so a streak spans cycles, both
	// escalate-and-continue. goalStall counts consecutive empty/blocked cycles
	// (cycles 640-644). nonprogress counts the UNION — every cycle that landed
	// nothing, FAIL included — because the empty-only counter and the
	// consecutive-FAIL breaker each reset on the other's outcome, so an
	// interleaved FAIL,EMPTY,FAIL,EMPTY goal escaped both. Thresholds/weight
	// sourced from policy.json (never a Go literal).
	//
	// SEQUENTIAL PATH ONLY (review MEDIUM — do not read this as closing the
	// fleet gap): the wave and pool branches `continue` above, and each lane is
	// a separate `evolve cycle` process holding no cross-cycle streak, so no
	// fleet lane advances either counter. Fleet breaker parity is a standing
	// separate gap.
	var goalStall, nonprogress goalStallTracker
	stallCfg := loadGoalStallConfig(cfg.EvolveDir)

	// Pipeline-blocker breaker scope: only failures from THIS batch count
	// (digests of cycles > batchStartCycle), so a historic blocker that was
	// already fixed can never halt a fresh healthy run. The floor is the
	// ALLOCATION lease, not the completion counter — an aborted cycle writes a
	// digest without ever advancing LastCycleNumber, and anchoring on that
	// counter kept re-collecting those digests on every relaunch
	// (readBatchWindowFloor, cmd_loop_control.go).
	batchStartCycle, _ := readBatchWindowFloor(context.Background(), deps.Storage)
	sequentialState := sequentialBatchState{
		dispPolicy:          dispPolicy,
		threshold:           threshold,
		prevRanCycle:        prevRanCycle,
		sameCycleStreak:     sameCycleStreak,
		maxConsecutiveFails: maxConsecutiveFails,
		consecutiveFails:    consecutiveFails,
		budgetStage:         budgetStage,
		effectiveMax:        effectiveMax,
		goalStall:           goalStall,
		nonprogress:         nonprogress,
		stallCfg:            stallCfg,
	}

iterations:
	for i := 0; i < effectiveMax; i++ {
		if decision := b.prepareIteration(i, &fleetCfg, &waveBinPath, batchStartCycle); decision.flow == batchReturn {
			return decision.exitCode
		}
		switch decision := b.dispatchFleetIteration(i, fleetCfg, waveBinPath, &starvationTracker); decision.flow {
		case batchNextIteration:
			continue
		case batchReturn:
			return decision.exitCode
		}
		switch decision := b.runSequentialIteration(i, &sequentialState); decision.flow {
		case batchNextIteration:
			continue
		case batchStopIterations:
			break iterations
		case batchReturn:
			return decision.exitCode
		}
	}

	if lr.StopReason == "error" || lr.StopReason == "fail" {
		lr.emitFatal(stdout, stderr, cfg, lastCycleIn(*lr))
		return 2
	}
	finalizeCompletedCycle(cfg, stderr)
	// Batch-END workspace hygiene sweep (workspace-hygiene S5 wiring). Ordered
	// finalize FIRST, then the hook, so the sweep observes a FINALIZED batch —
	// the completed cycle's marker is already cleared, so the run dirs and
	// worktrees this batch produced are reapable now instead of one batch
	// later. Only this clean-exit path fires it: the error/fail path returned
	// above and every signal path returns 130 without reaching here, because a
	// signal-interrupted batch stays resumable (`evolve loop --resume`) and
	// reaping its worktrees/branches would destroy the state the resume needs.
	gcHookFn(cfg, cycleWorkspace(cfg.ProjectRoot, batchEndGCCycle(*lr, lastBeforeGCHook+1)), stderr)
	lr.emit(stdout)
	// E1 exit-code contract: when any cycle in the batch hit a
	// recoverable failure (verify-fail + classify → recoverable) OR a
	// verdict-FAIL the breaker absorbed and continued past, signal rc=3
	// so CI sees "batch completed but with audit/build/infra issues".
	// Matches bash dispatcher's DISPATCH_RC=3.
	if lr.RecoverableFailures > 0 || lr.ContinuedFailures > 0 {
		return 3
	}
	return 0
}
