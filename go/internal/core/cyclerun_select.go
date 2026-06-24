package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

// selectNext computes the next phase for this iteration (the static transition
// switch + the dynamic-routing override + the spine-integrity gate) extracted
// behavior-preserving from RunCycle. It consumes/clears cr.scheduledNext and
// increments cr.routingSeq; recordRoutingDecision runs AFTER the spine gate's
// fail-closed abort returns (so an aborted routing decision is NOT recorded —
// the original ordering, H14).
//
// Returns loopAbort + error (illegal static transition, or a fail-closed spine
// gate at enforce), loopBreak (next == PhaseEnd → terminate the loop), or
// loopNext + the chosen next phase.
func (cr *cycleRun) selectNext() (Phase, loopAction, error) {
	var next Phase
	// fromSchedule marks an iteration whose `next` came from scheduledNext —
	// an authoritative injection by the retro branch, the ship-error recovery
	// seam, or the debugger decision. The dynamic-routing override
	// (enforceNext) must NOT second-guess such a transition, so it is gated on
	// !fromSchedule (generalizing the prior current!=PhaseRetro guard).
	fromSchedule := false
	switch {
	case cr.scheduledNext != "":
		next = cr.scheduledNext
		cr.scheduledNext = ""
		fromSchedule = true
	case cr.current == PhaseStart:
		// First edge is gated by intent_required, not by verdict.
		next = cr.o.sm.NextFromStart(cr.cs.IntentRequired)
	case !cr.current.IsValid():
		// current is a user-defined phase (only reachable when dynamic
		// routing selected it). The static successor is simply the next
		// entry in the configured order; the routing block below refines it.
		next = cr.o.nextInOrder(cr.current)
	default:
		n, err := cr.o.sm.Next(cr.current, cr.lastVerdict)
		if err != nil {
			return "", loopAbort, fmt.Errorf("transition from %s: %w", cr.current, err)
		}
		next = n
	}

	// Dynamic routing (shadow → advisory → enforce). Stage:Off — the
	// default — leaves the static state machine fully in control: no
	// digest, no ledger entry, byte-identical to legacy. When enabled,
	// digest the completed handoffs, ask the Strategy for a decision,
	// record it forensically, and — at Advisory and above — let the clamped
	// decision override the static successor, re-validated against the
	// legality oracle (CanTransition) AND the artifact-backed spine gate
	// (SpineSatisfiedUpTo). Retro keeps its failure-adapter shim
	// (decideAfterRetro) while routing is bedded in, so routing never
	// overrides the retro branch. The configurable mandatory set
	// (cfg.Mandatory) decides which phases are never-skip; the integrity
	// floor (ClampPlanToFloor, applied to the upfront plan) decides what ship
	// still requires regardless of how small the operator makes that set.
	if cr.o.cfg.Stage != config.StageOff {
		cr.routingSeq++
		signals, _ := router.Digest(cr.cs.WorkspacePath, cr.cs.CompletedPhases)
		dec := cr.o.strategy.Decide(router.RouteInput{
			Current:   string(cr.current),
			Verdict:   cr.lastVerdict,
			Signals:   signals,
			History:   entriesFromRecords(cr.state.FailedAt),
			Cfg:       cr.o.cfg,
			Completed: cr.cs.CompletedPhases,
			Strict:    cr.workflowConfig.StrictAudit,
			Now:       cr.o.now(),
			// Proposer context (DynamicLLM only; ignored by pure Route).
			Workspace:   cr.cs.WorkspacePath,
			ProjectRoot: cr.req.ProjectRoot,
			Cycle:       cr.cycle,
			Env:         cr.envSnap,
			BenchedCLIs: cr.benchedCLIs,
			// Clamped whole-cycle plan (Stage>=Advisory). nil below Advisory
			// or on planner failure ⇒ shouldRun runs the legacy trigger path.
			Plan:           cr.clampedPlan,
			IntentRequired: cr.cs.IntentRequired,
			PSMASEnabled:   cr.workflowConfig.PSMASEnabled,
		})
		if cr.o.cfg.Stage >= config.StageAdvisory && !fromSchedule {
			if forced, ok := cr.o.enforceNext(cr.current, next, signals, dec, planRunsShip(cr.clampedPlan)); ok {
				next = forced
			}
			// Full spine-integrity check on the SELECTED next (static OR
			// override). R5 (cycle-283 fix): the gate now fails CLOSED at
			// EVOLVE_PHASE_RECOVERY=enforce when the absence is CLEAN —
			// Digest distinguishes a transient read miss (DigestDegraded)
			// from a genuine gap, which was the original fail-open
			// rationale. Sequence: re-digest once (the artifact may have
			// landed between the routing digest and this check); a
			// still-unsatisfied spine with a degraded digest, or any miss
			// below enforce, keeps the loud-WARN fail-open (shadow =
			// byte-compatible until the R8.5 dial flip); a clean absence
			// at enforce aborts FAILED-EXPLAINED with the worktree
			// preserved. The operator waiver stays cfg.Mandatory
			// (isConfiguredMandatory) — no new escape hatch.
			if next != PhaseEnd && !cr.o.sm.SpineSatisfiedUpTo(next, signals, cr.o.cfg) {
				// The re-digest runs only in this already-anomalous branch
				// (zero cost on the happy path). A non-nil digest ERROR
				// means we cannot even establish what is absent — that is
				// never a "clean absence", so it fails open like a
				// degraded read (review F6: a missing workspace must not
				// masquerade as a spine gap at enforce).
				fresh, derr := router.Digest(cr.cs.WorkspacePath, cr.cs.CompletedPhases)
				cleanAbsence := derr == nil && len(fresh.DigestDegraded) == 0
				switch {
				case derr == nil && cr.o.sm.SpineSatisfiedUpTo(next, fresh, cr.o.cfg):
					// Transient: the handoff appeared on re-read. Proceed.
					// (dec was decided from the stale signals — diagnostic
					// record only; the gate, not Decide, owns blocking.)
				case cr.o.cfg.PhaseRecovery == config.StageEnforce && cleanAbsence:
					spineErr := fmt.Errorf("spine gate: next=%s blocked — a mandatory predecessor's handoff artifact is missing (clean absence, fail-closed; cycle-283 class)", next)
					cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, PhaseResponse{Phase: string(next)}, 0, spineErr.Error()))
					cr.recordFailureLearning(next, spineErr, 1)
					return "", loopAbort, spineErr
				default:
					// Two fail-open sources, deliberately identical in
					// behavior: (a) dial below enforce (shadow default —
					// byte-compatible until the R8.5 flip), (b) the
					// absence is NOT clean (degraded read or digest
					// error). The reason string tells them apart.
					dec.Clamps = append(dec.Clamps, router.Clamp{
						Rule:     "spine-unsatisfied-warn",
						Proposed: string(next),
						Forced:   string(next),
					})
					reason := "would-block at enforce"
					switch {
					case derr != nil:
						reason = "re-digest error: " + derr.Error()
					case len(fresh.DigestDegraded) > 0:
						reason = "digest degraded: " + strings.Join(fresh.DigestDegraded, "; ")
					}
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN spine not satisfied for next=%s (a mandatory predecessor's handoff artifact is missing); proceeding fail-open (%s)\n", next, reason)
				}
			}
		}
		cr.o.recordRoutingDecision(cr.ctx, cr.cycle, cr.cs, cr.routingSeq, dec)
	}

	if next == PhaseEnd {
		return next, loopBreak, nil
	}
	return next, loopNext, nil
}
