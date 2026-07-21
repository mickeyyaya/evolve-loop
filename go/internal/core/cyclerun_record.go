package core

import (
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
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
		cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, lerr.Error(), cr.cs.PhaseStartedAt))
		return loopAbort, lerr
	}

	// Cycle-778 ship-window lease: audit's binding snapshot (`git rev-parse
	// HEAD` inside emitPhaseBindings→recordAuditBinding) opens the window a
	// sibling landing on main would turn into a deep-tier re-audit — acquire
	// BEFORE the snapshot; any later completed phase (normally ship, after its
	// push) releases below. No-op for every phase but audit; fail-open.
	cr.acquireShipWindow(next)

	cr.o.emitPhaseBindings(cr.ctx, cr.cycle, cr.req.ProjectRoot, cr.cs, next, dr.resp.Verdict)

	// The first completed phase AFTER audit closes the critical section: ship
	// has pushed (or the cycle routed away from ship, where holding buys
	// nothing) — free the window for sibling lanes. No-op when not held.
	if next != PhaseAudit {
		cr.releaseShipWindow()
	}

	// Cycle-156 fix (Option C): a committing builder (e.g. agy/Gemini
	// following evolve-builder.md:235) leaves its work in a worktree
	// commit, but audit + binding inspect `git diff HEAD` (empty after a
	// commit). Soft-reset the build's commits to the cycle base so the
	// work is pending again before audit runs (next iteration). No-op for
	// non-committing builders. See the cycle-156 incident doc.
	cr.o.normalizeBuildWorktree(cr.ctx, next, cr.cs)

	// Cycle-636 (ship-sha-repin-after-build): close the frozen-pin
	// SELF_SHA_TAMPERED cascade (denied ship on 625->634). A legitimate in-version
	// rebuild replaces go/bin/evolve, but the cycle-514 boot healer only re-pins at
	// boot — so re-pin here too, immediately after a successful build, through the
	// SAME provenance-gated primitive (phaseintegrity.RepinIfDrifted) the boot path
	// uses. Fail-open: refusal/error WARNs; the ship gate stays the backstop.
	if next == PhaseBuild {
		repinShipSHAAfterBuild(cr.req.ProjectRoot)

		// Cycle-675 (new-package-graduation-buildentry-gate, 3rd recurrence):
		// a go/internal package NEW this cycle and absent from
		// go/.apicover-enforce fails the build phase HERE, with an explicit
		// abort_reason — after the worktree normalize (so a committing
		// builder's work is pending again) and before the phase is marked
		// completed. Deliberately abort-capable, unlike the WARN-only
		// buildSelfCheck: graduation is the builder's own obligation, and the
		// audit-side twin (apicoverNewPackageGraduationDefault) firing two
		// attempts later is exactly the recurrence this closes.
		if reason := buildGraduationCheck(cr.ctx, cr.cs.ActiveWorktree); reason != "" {
			gerr := fmt.Errorf("build graduation guard: %s", reason)
			cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, gerr.Error(), cr.cs.PhaseStartedAt))
			return loopAbort, gerr
		}
	}

	cr.cs.CompletedPhases = append(cr.cs.CompletedPhases, string(next))
	if err := cr.o.storage.WriteCycleState(cr.ctx, cr.cs); err != nil {
		werr := fmt.Errorf("write cycle-state post-%s: %w", next, err)
		cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, werr.Error(), cr.cs.PhaseStartedAt))
		return loopAbort, werr
	}

	if PhaseBoundaryCheckpointer != nil {
		if err := PhaseBoundaryCheckpointer(cr.cs, cr.req.ProjectRoot, cr.o.now()); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase boundary checkpoint failed: %v\n", err)
		}
	}

	// Cycle-802 (retro-bridge-timeout-width10): floor-gated verdict write —
	// CompletedPhases already includes `next` (appended above), so
	// floorAlreadyCompleted reflects whether an authoritative (floor/ship)
	// verdict preceded this phase. A non-floor post-verdict phase failing under
	// quota/timeout no longer clobbers a floor PASS; it degrades into
	// SkippedPhases instead. See final_verdict_floor.go.
	cr.o.recordFinalVerdict(&cr.result, next, dr.resp.Verdict, cr.o.floorAlreadyCompleted(cr.cs.CompletedPhases))
	// Learn from a FLOOR-phase FAIL verdict returned with NO dispatch error:
	// audit's in-process CI-parity gates (skills-drift / gofmt / EGPS / apicover)
	// override the auditor's narrative PASS to FAIL (err==nil), so this success
	// path — not an error path — records the outcome. Without feeding
	// failure-learning here, the deterministic gate-FAIL never reached
	// state.FailedAt, so the failure-adapter and Scout were blind to it and a
	// self-defeating task (the skills-drift storm, cycles 836/838/841/843/849)
	// re-derived the same doomed fix forever. Retro still runs via the normal
	// FAIL→retro transition, so this records only — it does not run retro.
	if dr.resp.Verdict == VerdictFAIL && cr.o.isAuthoritativePhase(next) {
		cr.o.recordFloorVerdictFailure(cr.ctx, cr.req, cr.cycle, next, &cr.state, &cr.cs, dr.resp.Diagnostics)
	}
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, "", cr.cs.PhaseStartedAt))
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
		var sysFail *SystemFailureSignal
		// Carry the cross-cycle failure history onto the per-cycle checkpoint so
		// the ADR-0072 S4 dossier composes its non-progress counters from live
		// evidence (additive; keeps the judgment layer non-inert).
		cr.cs.FailedAt = cr.state.FailedAt
		if cr.o.cfg.Stage >= config.StageAdvisory {
			// Failure floor Phase 3: the failure branch is advisor-
			// decidable (clamped) and leaves a routing-decision artifact.
			cr.routingSeq++
			branch, extraEnv, reason, sysFail = cr.o.decideAfterRetroRouted(cr.ctx, cr.cycle, cr.cs, cr.routingSeq, dr.resp.Verdict, cr.state.FailedAt, router.RouteInput{
				Cfg:            cr.o.cfg,
				Completed:      cr.cs.CompletedPhases,
				Strict:         cr.workflowConfig.StrictAudit,
				Workspace:      cr.cs.WorkspacePath,
				ProjectRoot:    cr.req.ProjectRoot,
				Cycle:          cr.cycle,
				Env:            cr.envSnap,
				Plan:           cr.clampedPlan,
				IntentRequired: cr.cs.IntentRequired,
				PSMASEnabled:   cr.workflowConfig.PSMASEnabled,
			})
		} else {
			branch, extraEnv, reason, sysFail = cr.o.decideAfterRetro(cr.cs, dr.resp.Verdict, cr.state.FailedAt)
		}
		for k, v := range extraEnv {
			cr.envSnap[k] = v
		}
		reason = cr.o.escalateRetroReasonForHistory(cr.req.ProjectRoot, reason, cr.state.FailedAt)
		cr.result.RetroDecision = reason
		// ADR-0072 S4: a floor category classified at the retro chokepoint is a
		// SYSTEM-level failure — mark it so the batch loop HALTS + escalates for
		// pipeline diagnosis instead of re-selecting the same task. finalizeCycle
		// sees SystemFailure already set and skips its own re-detection.
		if sysFail != nil && cr.result.SystemFailure == nil {
			cr.result.SystemFailure = sysFail
		}
		if branch == PhaseEnd {
			return loopBreak, nil
		}
		if !cr.o.sm.CanTransition(PhaseRetro, branch) {
			return loopAbort, fmt.Errorf("retro→%s not allowed by state machine", branch)
		}
		cr.scheduledNext = branch
	}

	// The debugger phase is decision-driven (RESHIP / RERUN_PHASE / BLOCK), not
	// verdict-driven — mirror the retro branch. The debugger runner surfaces its
	// decision on PhaseResponse.Signals; decideAfterDebugger maps it to the next
	// phase, which the next iteration runs via scheduledNext. The signal-branch
	// gate is config-driven (ADR-0058) — successorStrategy resolves the debugger's
	// branching_strategy from the builtinControlSpec seam and owns the degrade.
	if cr.o.successorStrategy(cr.current) == phasespec.BranchingSignal {
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
