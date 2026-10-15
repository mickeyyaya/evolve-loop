package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (cr *cycleRun) recordSpineFailOpen(next Phase, missingArtifact, reason string) {
	cr.result.SpineFailOpens = append(cr.result.SpineFailOpens, SpineFailOpen{
		Phase:           string(next),
		MissingArtifact: missingArtifact,
		Reason:          reason,
	})
}

func (cr *cycleRun) selectNext() (Phase, loopAction, error) {
	var next Phase
	fromSchedule := false
	switch {
	case cr.scheduledNext != "":
		next = cr.scheduledNext
		cr.scheduledNext = ""
		fromSchedule = true
	case cr.current == PhaseStart:
		next = cr.o.sm.NextFromStart(cr.cs.IntentRequired)
	case !cr.current.IsValid():
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
			Current:        string(cr.current),
			Verdict:        cr.lastVerdict,
			Signals:        signals,
			History:        entriesFromRecords(cr.state.FailedAt),
			Cfg:            cr.o.cfg,
			Completed:      cr.cs.CompletedPhases,
			Strict:         cr.workflowConfig.StrictAudit,
			Now:            cr.o.now(),
			Workspace:      cr.cs.WorkspacePath,
			ProjectRoot:    cr.req.ProjectRoot,
			Cycle:          cr.cycle,
			Env:            cr.envSnap,
			BenchedCLIs:    cr.benchedCLIs,
			Plan:           cr.clampedPlan,
			IntentRequired: cr.cs.IntentRequired,
			PSMASEnabled:   cr.workflowConfig.PSMASEnabled,
		})
		if cr.o.cfg.Stage >= config.StageAdvisory && !fromSchedule {
			if forced, ok := cr.o.enforceNext(cr.current, next, cr.lastVerdict, signals, dec, planRunsShip(cr.clampedPlan)); ok {
				next = forced
			}
			if next != PhaseEnd && !cr.o.sm.SpineSatisfiedUpTo(next, signals, cr.o.cfg) {
				fresh, derr := router.Digest(cr.cs.WorkspacePath, cr.cs.CompletedPhases)
				cleanAbsence := derr == nil && len(fresh.DigestDegraded) == 0
				switch {
				case derr == nil && cr.o.sm.SpineSatisfiedUpTo(next, fresh, cr.o.cfg):
				case cr.o.cfg.SpineFloor == config.StageEnforce && cleanAbsence:
					spineErr := fmt.Errorf("spine gate: next=%s blocked — a mandatory predecessor's handoff artifact is missing (clean absence, fail-closed; cycle-283 class)", next)
					cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, PhaseResponse{Phase: string(next)}, 0, spineErr.Error(), ""))
					cr.recordFailureLearning(next, spineErr, 1)
					return "", loopAbort, spineErr
				default:
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
