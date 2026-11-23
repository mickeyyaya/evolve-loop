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
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeQuotaPaused, "the dispatch ended at a wall with at least one family on quota, or no recorded cause. The cycle pauses at the named phase and stays resumable. A wall of only login or other causes does not emit this code")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeLaneDeferred, "a fleet lane failed to provision its worktree (after fetch retries and one re-provision) and exited 5 as DEFERRED. Claims release unbumped with no digest, learning, or retro. lane_deferral_halt_ceiling consecutive deferrals halt the batch (LOOP_PIPELINE_BLOCKER_HALT rule lane-deferrals). fields.step=worktree, cause")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeLaneResumed, "a fleet lane found its todo's last cycle paused at a wall and resumed it at the paused phase. The dispatch chain picks the first healthy family. Resume uses the checkpoint auto-resume budget; INFO. Fields: fields.scope, phase, attempts, max_attempts")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeLaneBindingSettled, "a lane cycle held no manifest while the root registry bound its scope to an older cycle. Runtime settled the binding before the audit, at a lane resume or after the triage adoption of a fresh cycle: action=released drops it, action=recorded writes it to the manifest. standing is absent, settled, owed, unreadable, or newer. Release with standing=absent drops an unproven debt (trace event, WARN). Fields: fields.scope_id, ancestor_cycle, action, standing, origin ("+string(BindingOriginLaneResume)+" or "+string(BindingOriginFreshStart)+")")
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

func (cr *cycleRun) emitQuotaPaused(phase Phase, cause error, wall Wall) {
	cr.o.signals.Emit(signalcenter.Event{
		Cycle: cr.cycle, RunID: cr.cs.RunID, Phase: string(phase), Module: signalcenter.ModuleOrchestrator, Origin: "cycleRun.pauseForQuota",
		Kind: signalcenter.KindQuotaPaused, Severity: signalcenter.SeverityWarn, Code: CodeQuotaPaused,
		Reason: cause.Error(), Fields: map[string]string{"phase": string(phase), "stop_reason": wall.StopReason()},
	})
}

func EmitLaneResumed(signals *signalcenter.Center, rp *ResumePoint, scope []string) {
	joined := strings.Join(scope, ",")
	signals.Emit(signalcenter.Event{
		Cycle: rp.CycleID, Phase: rp.Phase, Module: signalcenter.ModuleOrchestrator, Origin: "core.EmitLaneResumed",
		Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo, Code: CodeLaneResumed,
		Reason: "lane " + joined + " resumes cycle " + strconv.Itoa(rp.CycleID) + " at phase " + rp.Phase + " after a wall pause",
		Fields: map[string]string{"scope": joined, "phase": rp.Phase, "attempts": strconv.Itoa(rp.AutoAttempts), "max_attempts": strconv.Itoa(rp.AutoMaxAttempts)},
	})
}

type BindingOrigin string

const (
	BindingOriginLaneResume BindingOrigin = "lane-resume"
	BindingOriginFreshStart BindingOrigin = "fresh-start"
)

type BindingSettlement struct {
	Cycle, Ancestor           int
	ScopeID, Action, Standing string
	Origin                    BindingOrigin
}

func EmitLaneBindingSettled(signals *signalcenter.Center, s BindingSettlement) {
	signals.Emit(signalcenter.Event{
		Cycle: s.Cycle, Module: signalcenter.ModuleOrchestrator, Origin: "core.EmitLaneBindingSettled",
		Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityWarn, Code: CodeLaneBindingSettled,
		Reason: "cycle " + strconv.Itoa(s.Cycle) + " " + s.Action + " the binding of " + s.ScopeID + " to cycle-" + strconv.Itoa(s.Ancestor) + " before its audit (ancestor ledger " + s.Standing + ")",
		Fields: map[string]string{"scope_id": s.ScopeID, "ancestor_cycle": strconv.Itoa(s.Ancestor), "action": s.Action, "standing": s.Standing, "origin": string(s.Origin)},
	})
}
