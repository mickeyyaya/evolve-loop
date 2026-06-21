package core

import (
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// recordAndBranch runs the end-of-iteration record + branch step (extracted
// behavior-preserving from RunCycle): the success "phase" ledger append, phase
// bindings, build-worktree normalize, CompletedPhases append + cycle-state
// persist, phase-boundary checkpoint, the success outcome record + cursor
// advance (current/lastVerdict), and the retro and debugger non-verdict-driven
// branches that inject cr.scheduledNext.
//
// Returns loopBreak when the retro/debugger branch routes to PhaseEnd (the
// caller terminates the loop → finalizeCycle), loopAbort + error on a ledger /
// state-write failure or an illegal retro/debugger edge, or loopNext to advance
// to the next iteration. The per-phase resp/attemptCount arrive via dr.
func (cr *cycleRun) recordAndBranch(next Phase, dr dispatchResult) (loopAction, error) {
	if err := cr.o.ledger.Append(cr.ctx, LedgerEntry{
		TS:       cr.o.now().UTC().Format(time.RFC3339),
		Cycle:    cr.cycle,
		Role:     string(next),
		Kind:     "phase",
		ExitCode: 0,
	}); err != nil {
		lerr := fmt.Errorf("ledger append for %s: %w", next, err)
		// ADR-0044 C1: the phase completed; a persistence failure must
		// not erase its outcome from the timing/usage record.
		cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, lerr.Error()))
		return loopAbort, lerr
	}

	cr.o.emitPhaseBindings(cr.ctx, cr.cycle, cr.req.ProjectRoot, cr.cs, next, dr.resp.Verdict)

	// Cycle-156 fix (Option C): a committing builder (e.g. agy/Gemini
	// following evolve-builder.md:235) leaves its work in a worktree
	// commit, but audit + binding inspect `git diff HEAD` (empty after a
	// commit). Soft-reset the build's commits to the cycle base so the
	// work is pending again before audit runs (next iteration). No-op for
	// non-committing builders. See the cycle-156 incident doc.
	cr.o.normalizeBuildWorktree(cr.ctx, next, cr.cs)

	cr.cs.CompletedPhases = append(cr.cs.CompletedPhases, string(next))
	if err := cr.o.storage.WriteCycleState(cr.ctx, cr.cs); err != nil {
		werr := fmt.Errorf("write cycle-state post-%s: %w", next, err)
		cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, werr.Error()))
		return loopAbort, werr
	}

	if PhaseBoundaryCheckpointer != nil {
		if err := PhaseBoundaryCheckpointer(cr.cs, cr.req.ProjectRoot, cr.o.now()); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase boundary checkpoint failed: %v\n", err)
		}
	}

	cr.result.FinalVerdict = dr.resp.Verdict
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, ""))
	cr.current = next
	cr.lastVerdict = dr.resp.Verdict

	// Retro is the one phase whose successor isn't verdict-driven: the
	// failure-adapter consults cycle history (state.FailedAt) and the retro
	// verdict to pick {ship | tdd | end}. Set scheduledNext so the next loop
	// iteration runs the chosen phase. The history-branch gate is config-driven
	// (ADR-0058) — successorStrategy resolves the completed phase's
	// branching_strategy and owns the byte-identity degrade.
	if cr.o.successorStrategy(cr.current) == phasespec.BranchingHistory {
		var branch Phase
		var extraEnv map[string]string
		var reason string
		if cr.o.cfg.Stage >= config.StageAdvisory {
			// Failure floor Phase 3: the failure branch is advisor-
			// decidable (clamped) and leaves a routing-decision artifact.
			cr.routingSeq++
			branch, extraEnv, reason = cr.o.decideAfterRetroRouted(cr.ctx, cr.cycle, cr.cs, cr.routingSeq, dr.resp.Verdict, cr.state.FailedAt, router.RouteInput{
				Cfg:            cr.o.cfg,
				Completed:      cr.cs.CompletedPhases,
				Strict:         envchain.BoolValue(cr.envSnap["EVOLVE_STRICT_AUDIT"], false),
				Workspace:      cr.cs.WorkspacePath,
				ProjectRoot:    cr.req.ProjectRoot,
				Cycle:          cr.cycle,
				Env:            cr.envSnap,
				Plan:           cr.clampedPlan,
				IntentRequired: cr.cs.IntentRequired,
				PSMASEnabled:   cr.workflowConfig.PSMASEnabled,
			})
		} else {
			branch, extraEnv, reason = cr.o.decideAfterRetro(dr.resp.Verdict, cr.state.FailedAt)
		}
		for k, v := range extraEnv {
			cr.envSnap[k] = v
		}
		cr.result.RetroDecision = reason
		if branch == PhaseEnd {
			return loopBreak, nil
		}
		if !cr.o.sm.CanTransition(PhaseRetro, branch) {
			return loopAbort, fmt.Errorf("retro→%s not allowed by state machine", branch)
		}
		cr.scheduledNext = branch
	}

	// The debugger phase is decision-driven (RESHIP / RERUN_PHASE / BLOCK),
	// not verdict-driven — mirror the retro branch. The debugger runner
	// surfaces its decision on PhaseResponse.Signals; decideAfterDebugger
	// maps it to the next phase, which the next iteration runs via
	// scheduledNext (bypassing the routing override, like retro).
	if cr.current == PhaseDebugger {
		branch := cr.o.decideAfterDebugger(dr.resp)
		cr.o.recordDebuggerDecision(cr.ctx, cr.cycle, cr.cs, dr.resp)
		if branch == PhaseEnd {
			return loopBreak, nil
		}
		if !cr.o.sm.CanTransition(PhaseDebugger, branch) {
			return loopAbort, fmt.Errorf("debugger→%s not allowed by state machine", branch)
		}
		cr.scheduledNext = branch
	}

	return loopNext, nil
}
