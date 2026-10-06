package core

import (
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeCycleFailed   signalcenter.Code = "ORCHESTRATOR_CYCLE_FAILED"
	CodeSystemFailure signalcenter.Code = "ORCHESTRATOR_SYSTEM_FAILURE"
	CodeQuotaPaused   signalcenter.Code = "ORCHESTRATOR_QUOTA_PAUSED"
	CodeLaneDeferred  signalcenter.Code = "ORCHESTRATOR_LANE_DEFERRED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeCycleFailed, "the cycle sealed with final verdict FAIL; fields carry the termination reason and retro decision")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeSystemFailure, "an ADR-0072 system-level failure was attached to the cycle (INCIDENT when it halts the loop, WARN otherwise); fields.category names the floor")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeQuotaPaused, "every CLI family is quota-exhausted; the cycle is paused at the named phase and resumable")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeLaneDeferred, "a fleet lane could not provision its worktree (after the fetch retries and one re-provision) and ended before any phase dispatched: outcome DEFERRED, its claims released to the queue unbumped, no failure digest, failure learning or retrospective; the lane exits 5, and lane_deferral_halt_ceiling consecutive deferrals halt the batch (LOOP_PIPELINE_BLOCKER_HALT rule lane-deferrals); fields.step=worktree, cause")
}

// signalRunID is the run id the orchestrator stamps on its own events.
func (o *Orchestrator) signalRunID() string {
	runID, _ := o.currentRunID.Load().(string)
	return runID
}

// emitCycleClose ends the cycle's event stream: a system.failure first when
// the closeout attached one, then the cycle.sealed that names the final
// verdict. origin is the caller's own name, so the callee never asserts its
// caller's identity.
func (cr *cycleRun) emitCycleClose(result CycleResult, origin string) {
	o := cr.o
	base := signalcenter.Event{
		Cycle: cr.cycle, RunID: cr.cs.RunID, Module: signalcenter.ModuleOrchestrator, Origin: origin,
	}
	if sf := result.SystemFailure; sf != nil {
		severity := signalcenter.SeverityWarn
		if sf.Halt {
			severity = signalcenter.SeverityIncident
		}
		sfEvent := base
		sfEvent.Kind, sfEvent.Severity, sfEvent.Code = signalcenter.KindSystemFailure, severity, CodeSystemFailure
		sfEvent.Reason = sf.Category + ": " + sf.Evidence
		sfEvent.Fields = map[string]string{"category": sf.Category, "level": sf.Level, "halt": strconv.FormatBool(sf.Halt)}
		o.signals.Emit(sfEvent)
	}
	e := base
	e.Kind, e.Severity = signalcenter.KindCycleSealed, signalcenter.SeverityInfo
	e.Reason = "final verdict " + result.FinalVerdict
	e.Fields = map[string]string{"final_verdict": result.FinalVerdict, "phases_run": strconv.Itoa(len(result.PhasesRun))}
	if result.TerminationReason != "" {
		e.Fields["termination_reason"] = result.TerminationReason
	}
	if result.RetroDecision != "" {
		e.Fields["retro_decision"] = result.RetroDecision
	}
	if result.FinalVerdict == VerdictFAIL {
		e.Severity, e.Code = signalcenter.SeverityWarn, CodeCycleFailed
	}
	o.signals.Emit(e)
}

// emitShipError projects a recorded ShipError verbatim, including the
// ship-error.json path when it was written and the Debug keys the triage
// whitelist names (shiperr.SignalDebugKeys) when non-empty.
// See ADR-0103.
func (o *Orchestrator) emitShipError(cycle int, cs CycleState, se *ShipError, artifactPath string) {
	fields := map[string]string{"class": string(se.Class), "stage": string(se.Stage)}
	if artifactPath != "" {
		fields["path"] = artifactPath
	}
	for _, k := range shiperr.SignalDebugKeys {
		if v := se.Debug[k]; v != "" {
			fields[k] = v
		}
	}
	o.signals.Emit(signalcenter.Event{
		Cycle: cycle, RunID: cs.RunID, Phase: string(PhaseShip), Module: signalcenter.ModuleShip, Origin: "Orchestrator.recordShipError",
		Kind: signalcenter.KindShipError, Severity: se.Class.SignalSeverity(), Code: shiperr.SignalCode(se.Code),
		Reason: se.Message, Fields: fields,
	})
}

// emitQuotaPaused names the phase a quota pause interrupted; a resource
// event, resumable, never terminal evidence.
func (cr *cycleRun) emitQuotaPaused(phase Phase, cause error) {
	cr.o.signals.Emit(signalcenter.Event{
		Cycle: cr.cycle, RunID: cr.cs.RunID, Phase: string(phase), Module: signalcenter.ModuleOrchestrator, Origin: "cycleRun.pauseForQuota",
		Kind: signalcenter.KindQuotaPaused, Severity: signalcenter.SeverityWarn, Code: CodeQuotaPaused,
		Reason: cause.Error(), Fields: map[string]string{"phase": string(phase)},
	})
}
