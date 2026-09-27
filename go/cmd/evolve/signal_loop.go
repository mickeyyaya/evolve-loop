package main

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeLoopSystemFailureHalt   signalcenter.Code = "LOOP_SYSTEM_FAILURE_HALT"
	CodeLoopPipelineBlockerHalt signalcenter.Code = "LOOP_PIPELINE_BLOCKER_HALT"
	CodeLoopFleetLaneHalt       signalcenter.Code = "LOOP_FLEET_LANE_HALT"
	CodeLoopHalt                signalcenter.Code = "LOOP_HALT"
	CodeLoopEscalationBoundary  signalcenter.Code = "LOOP_ESCALATION_BOUNDARY"
	// CodeLoopMinWidthRepair projects the wave engine's min-width-repair code.
	CodeLoopMinWidthRepair signalcenter.Code = loopwave.CodeMinWidthRepair
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopSystemFailureHalt, "the batch halted on an ADR-0072 system failure the cycle itself signalled (the pipeline, not the task, is the cause); fields.category names the floor, fields.next is the escalation dossier's next_action, fields.escalation and fields.inbox_item the dossier and the P0 item the halt wrote")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopPipelineBlockerHalt, "the pipeline-blocker breaker halted the batch (identical fingerprints, unexplained failures or consecutive failures over the ceiling); fields.rule and fields.fingerprint name the rule, the rest are the system-failure halt's own fields (next, escalation, inbox_item)")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopFleetLaneHalt, "a fleet lane exited with the system-failure halt code; the lane's own LOOP_SYSTEM_FAILURE_HALT names the failure and the escalation it filed")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopHalt, "the batch halted at a wave boundary (plane diverged, sync refused); the reason is the halt error")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopEscalationBoundary, "the escalation boundary staged inbox items (bumped/filed/planned) after a cycle; fields carry the counts and the stage")
}

// loopHaltRule is what only the caller of haltOnSystemFailure knows: the
// code naming the rule that halted the batch and the rule's own fields. The
// chokepoint merges them into the ONE INCIDENT it emits.
type loopHaltRule struct {
	code   signalcenter.Code
	fields map[string]string
}

// systemFailureRule is the rule of a halt the cycle itself signalled (a
// forged verdict, a phase-infra floor): the generic system-failure code.
var systemFailureRule = loopHaltRule{code: CodeLoopSystemFailureHalt}

// emitLoopHalt is the loop.halt producer: an INCIDENT — the batch stops.
func emitLoopHalt(signals *signalcenter.Center, cycle int, origin string, code signalcenter.Code, reason string, fields map[string]string) {
	signals.Emit(signalcenter.Event{
		Cycle: cycle, Module: signalcenter.ModuleLoop, Origin: origin, Kind: signalcenter.KindLoopHalt,
		Severity: signalcenter.SeverityIncident, Code: code, Reason: reason, Fields: fields,
	})
}

// emitLoopWave is the coordinator's spelling of the one loop.wave producer
// (loopwave.EmitWave): batch-level, INFO without a code, WARN with one.
func emitLoopWave(signals *signalcenter.Center, wave int, origin string, code signalcenter.Code, reason string, fields map[string]string) {
	loopwave.EmitWave(signals, wave, origin, code, reason, fields)
}

// emitLoopEscalation is the loop.escalation producer: a WARN that names the
// stage and what the boundary staged.
func emitLoopEscalation(signals *signalcenter.Center, cycle int, origin, reason string, fields map[string]string) {
	signals.Emit(signalcenter.Event{
		Cycle: cycle, Module: signalcenter.ModuleLoop, Origin: origin, Kind: signalcenter.KindLoopEscalation,
		Severity: signalcenter.SeverityWarn, Code: CodeLoopEscalationBoundary, Reason: reason, Fields: fields,
	})
}

// formatSignalReport renders the runner's per-cycle signal counts for the
// batch report — a report line, not a signal; the loop reports, never gates.
func formatSignalReport(cycle int, s signalcenter.Summary) string {
	if s.Total == 0 {
		return ""
	}
	line := fmt.Sprintf("[loop] cycle %d signals: %d total, %d WARN, %d INCIDENT", cycle, s.Total,
		s.BySeverity[signalcenter.SeverityWarn], s.BySeverity[signalcenter.SeverityIncident])
	if s.LastIncident != nil {
		line += fmt.Sprintf(" (last INCIDENT %s — %s)", s.LastIncident.Code, s.LastIncident.Reason)
	}
	// The gate verdicts per cycle, the "checked → advanced" record.
	if p, r, c := s.ByKind[signalcenter.KindGatePassed], s.ByKind[signalcenter.KindGateRejected], s.ByKind[signalcenter.KindGateCorrected]; p+r+c > 0 {
		line += fmt.Sprintf(" · gates: %d passed, %d rejected, %d corrected", p, r, c)
	}
	return line + "\n"
}
