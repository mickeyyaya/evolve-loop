package core

// signal_cycle.go — ADR-0101 S2a producers. Each is an Adapter from a typed
// value the pipeline already owns (CycleResult, SystemFailureSignal, ShipError,
// the quota sentinel) to ONE Event at the chokepoint that owns the fact:
// cycle.sealed + system.failure at completeCycle (the closeout both dispatch
// roots share), ship.error at recordShipError, quota.paused at the
// quota pause (pauseForQuota, both roots); the abnormal epilogue seals the
// aborted cycle. Producers observe; every decision stays where it was.

import (
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeCycleFailed   signalcenter.Code = "ORCHESTRATOR_CYCLE_FAILED"
	CodeSystemFailure signalcenter.Code = "ORCHESTRATOR_SYSTEM_FAILURE"
	CodeQuotaPaused   signalcenter.Code = "ORCHESTRATOR_QUOTA_PAUSED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeCycleFailed, "the cycle sealed with final verdict FAIL; fields carry the termination reason and retro decision")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeSystemFailure, "an ADR-0072 system-level failure was attached to the cycle (INCIDENT when it halts the loop, WARN otherwise); fields.category names the floor")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeQuotaPaused, "every CLI family is quota-exhausted; the cycle is paused at the named phase and resumable")
}

// signalRunID is the run id the orchestrator stamps on its own events.
func (o *Orchestrator) signalRunID() string {
	runID, _ := o.currentRunID.Load().(string)
	return runID
}

// emitCycleClose ends the cycle's event stream: a system.failure first when
// the closeout attached one (INCIDENT if it halts the loop), then the
// cycle.sealed that names the final verdict — WARN with its own code on FAIL.
// origin is the caller's own name (the closeout, or the abnormal epilogue for
// a cycle that died mid-phase): origin is vocabulary, so the callee never
// asserts its caller's identity.
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

// emitShipError projects a recorded ShipError verbatim: module ship, the
// SHIP_<code> projection, INCIDENT for the integrity class (§5.3), the
// ship-error.json path when it was written, and the Debug keys the triage
// whitelist names (shiperr.SignalDebugKeys — the landing step, the git exit
// code, the branches, the repair outcome; ADR-0103 unit 07) when non-empty.
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

// emitQuotaPaused names the phase a quota pause interrupted, with the one
// error text the ledger and the phase diag also carry; a resource event,
// resumable, never terminal evidence. Emitted from pauseForQuota — the seam
// both dispatch roots reach.
func (cr *cycleRun) emitQuotaPaused(phase Phase, cause error) {
	cr.o.signals.Emit(signalcenter.Event{
		Cycle: cr.cycle, RunID: cr.cs.RunID, Phase: string(phase), Module: signalcenter.ModuleOrchestrator, Origin: "cycleRun.pauseForQuota",
		Kind: signalcenter.KindQuotaPaused, Severity: signalcenter.SeverityWarn, Code: CodeQuotaPaused,
		Reason: cause.Error(), Fields: map[string]string{"phase": string(phase)},
	})
}
