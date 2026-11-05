package core

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeCycleFailed   signalcenter.Code = "ORCHESTRATOR_CYCLE_FAILED"
	CodeSystemFailure signalcenter.Code = "ORCHESTRATOR_SYSTEM_FAILURE"
	CodeQuotaPaused   signalcenter.Code = "ORCHESTRATOR_QUOTA_PAUSED"
	CodeLaneDeferred  signalcenter.Code = "ORCHESTRATOR_LANE_DEFERRED"
	CodeLaneResumed   signalcenter.Code = "ORCHESTRATOR_LANE_RESUMED"

	CodeLaneBindingSettled signalcenter.Code = "ORCHESTRATOR_LANE_BINDING_SETTLED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeCycleFailed, "the cycle sealed with final verdict FAIL; fields carry the termination reason and retro decision")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeSystemFailure, "an ADR-0072 system-level failure was attached to the cycle (INCIDENT when it halts the loop, WARN otherwise); fields.category names the floor")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeQuotaPaused, "every CLI family is quota-exhausted; the cycle is paused at the named phase and resumable")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeLaneDeferred, "a fleet lane could not provision its worktree (after the fetch retries and one re-provision) and ended before any phase dispatched: outcome DEFERRED, its claims released to the queue unbumped, no failure digest, failure learning or retrospective; the lane exits 5, and lane_deferral_halt_ceiling consecutive deferrals halt the batch (LOOP_PIPELINE_BLOCKER_HALT rule lane-deferrals); fields.step=worktree, cause")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeLaneResumed, "a fleet lane found that the last cycle of its todo paused on a quota wall, so it resumed that cycle at the paused phase instead of a fresh cycle; the dispatch chain gives the phase to the first healthy family; the resume counts against the auto-resume budget of the checkpoint; INFO; fields.scope, phase, attempts, max_attempts")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeLaneBindingSettled, "a resumed lane cycle held no continuation manifest while the root registry bound its scope to an older cycle, so the runtime settled the binding before the resumed phase ran: action=released drops the binding, action=recorded writes it into the workspace manifest; standing is the state of the ancestor ledger (absent, settled, owed, unreadable) or newer for a binding that is newer than the cycle; a release with standing=absent drops a debt that no ledger could prove, and a deleted manifest looks identical to a manifest that was never written, so the event is the trace; WARN; fields.scope_id, ancestor_cycle, action, standing")
}

func (o *Orchestrator) signalRunID() string {
	runID, _ := o.currentRunID.Load().(string)
	return runID
}

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
	e.Fields = map[string]string{cyclestate.SealedVerdictField: result.FinalVerdict, "phases_run": strconv.Itoa(len(result.PhasesRun))}
	if result.TerminationReason != "" {
		e.Fields[cyclestate.SealedTerminationField] = result.TerminationReason
	}
	if result.RetroDecision != "" {
		e.Fields["retro_decision"] = result.RetroDecision
	}
	if result.FinalVerdict == VerdictFAIL {
		e.Severity, e.Code = signalcenter.SeverityWarn, CodeCycleFailed
	}
	o.signals.Emit(e)
}

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

func (cr *cycleRun) emitQuotaPaused(phase Phase, cause error) {
	cr.o.signals.Emit(signalcenter.Event{
		Cycle: cr.cycle, RunID: cr.cs.RunID, Phase: string(phase), Module: signalcenter.ModuleOrchestrator, Origin: "cycleRun.pauseForQuota",
		Kind: signalcenter.KindQuotaPaused, Severity: signalcenter.SeverityWarn, Code: CodeQuotaPaused,
		Reason: cause.Error(), Fields: map[string]string{"phase": string(phase)},
	})
}

func EmitLaneResumed(signals *signalcenter.Center, rp *ResumePoint, scope []string) {
	joined := strings.Join(scope, ",")
	signals.Emit(signalcenter.Event{
		Cycle: rp.CycleID, Phase: rp.Phase, Module: signalcenter.ModuleOrchestrator, Origin: "core.EmitLaneResumed",
		Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo, Code: CodeLaneResumed,
		Reason: "lane " + joined + " resumes cycle " + strconv.Itoa(rp.CycleID) + " at phase " + rp.Phase + " after a quota pause",
		Fields: map[string]string{"scope": joined, "phase": rp.Phase, "attempts": strconv.Itoa(rp.AutoAttempts), "max_attempts": strconv.Itoa(rp.AutoMaxAttempts)},
	})
}

type BindingSettlement struct {
	Cycle, Ancestor           int
	ScopeID, Action, Standing string
}

func EmitLaneBindingSettled(signals *signalcenter.Center, s BindingSettlement) {
	signals.Emit(signalcenter.Event{
		Cycle: s.Cycle, Module: signalcenter.ModuleOrchestrator, Origin: "core.EmitLaneBindingSettled",
		Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityWarn, Code: CodeLaneBindingSettled,
		Reason: "cycle " + strconv.Itoa(s.Cycle) + " " + s.Action + " the binding of " + s.ScopeID + " to cycle-" + strconv.Itoa(s.Ancestor) + " before its resume (ancestor ledger " + s.Standing + ")",
		Fields: map[string]string{"scope_id": s.ScopeID, "ancestor_cycle": strconv.Itoa(s.Ancestor), "action": s.Action, "standing": s.Standing},
	})
}
