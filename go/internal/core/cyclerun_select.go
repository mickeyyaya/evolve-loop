package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// recordSpineFailOpen appends one spine-gate fail-open to the cycle result;
// repeats accumulate, and a cycle that never fails open carries an empty
// slice, so the rollup's threshold alarm stays silent on a healthy batch.
func (cr *cycleRun) recordSpineFailOpen(next Phase, missingArtifact, reason string) {
	cr.result.SpineFailOpens = append(cr.result.SpineFailOpens, SpineFailOpen{
		Phase:           string(next),
		MissingArtifact: missingArtifact,
		Reason:          reason,
	})
}

// recordRoutingDecision runs after the spine gate's fail-closed abort
// returns, so an aborted routing decision is never recorded.
func (cr *cycleRun) selectNext() (Phase, loopAction, error) {
	var next Phase
	// The dynamic-routing override (enforceNext) must not second-guess a
	// transition already authoritatively injected via cr.scheduledNext (the
	// retro branch, ship-error recovery, or the debugger).
	fromSchedule := false
	switch {
	case cr.scheduledNext != "":
		next = cr.scheduledNext
		cr.scheduledNext = ""
		fromSchedule = true
	case cr.current == PhaseStart:
		next = cr.o.sm.NextFromStart(cr.cs.IntentRequired)
	case !cr.current.IsValid():
		// current is a user-defined phase, reachable only when dynamic
		// routing selected it; the static state machine has no rule for it.
		next = cr.o.nextInOrder(cr.current)
	default:
		n, err := cr.o.sm.Next(cr.current, cr.lastVerdict)
		if err != nil {
			return "", loopAbort, fmt.Errorf("transition from %s: %w", cr.current, err)
		}
		next = n
	}

	if cr.current == PhaseTriage {
		terminal := cr.o.triageTermination(cr.req.ProjectRoot, cr.cs.WorkspacePath, cr.cs.CycleID, cr.cs.CompletedPhases, cr.lastVerdict)
		if terminal.stop {
			cr.result.TerminationReason = terminal.reason
			return PhaseEnd, loopBreak, nil
		}
	}

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
			if forced, ok := cr.o.enforceNext(cr.current, next, cr.lastVerdict, signals, dec, planRunsShip(cr.clampedPlan)); ok {
				next = forced
			}
			if next != PhaseEnd && !cr.o.sm.SpineSatisfiedUpTo(next, signals, cr.o.cfg) {
				// A digest error is never a clean absence, so it fails open too.
				fresh, derr := router.Digest(cr.cs.WorkspacePath, cr.cs.CompletedPhases)
				cleanAbsence := derr == nil && len(fresh.DigestDegraded) == 0
				switch {
				case derr == nil && cr.o.sm.SpineSatisfiedUpTo(next, fresh, cr.o.cfg):
					// Transient: the handoff appeared on re-read. dec was
					// decided from the stale signals — a diagnostic record
					// only, since the gate, not Decide, owns blocking.
				case cr.o.cfg.SpineFloor == config.StageEnforce && cleanAbsence:
					spineErr := fmt.Errorf("spine gate: next=%s blocked — a mandatory predecessor's handoff artifact is missing (clean absence, fail-closed; cycle-283 class)", next)
					// startedAt is empty: the spine gate blocks before this
					// phase dispatches, so there is no measured wall-clock
					// start. EndedAt is still stamped, to mark when the gate
					// blocked it.
					cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, PhaseResponse{Phase: string(next)}, 0, spineErr.Error(), ""))
					cr.recordFailureLearning(next, spineErr, 1)
					return "", loopAbort, spineErr
				default:
					// Two fail-open sources, deliberately identical in
					// behavior: SpineFloor dialed below enforce, or the
					// absence is not clean (a degraded read or digest error);
					// the reason string tells them apart.
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
					// The anchor is read from the signals the gate blocked on,
					// not the fresh re-digest, so the record explains this
					// fail-open. ok should always be true here; an empty
					// anchor is recorded as "unknown" rather than dropped.
					missing, ok := cr.o.sm.UnsatisfiedSpineAnchor(next, signals, cr.o.cfg)
					anchor := string(missing)
					if !ok || anchor == "" {
						anchor = "unknown"
					}
					cr.recordSpineFailOpen(next, anchor, reason)
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
