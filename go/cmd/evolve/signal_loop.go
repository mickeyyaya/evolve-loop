// signal_loop.go — ADR-0101 S4a: the loop module's Signal Center producers.
// A batch halt is ONE loop.halt INCIDENT whose code names the rule (system
// failure, pipeline blocker, a fleet lane's halt code, a wave-boundary halt);
// a wave is loop.wave (INFO for the summary the report also prints — INFO
// never reaches the console — WARN with a code for a min-width repair); an
// escalation boundary is loop.escalation WARN. Each replaces a hand-written
// "[loop] …" line 1:1 (the wave summary stays: it is the operator's report,
// not a fact printed twice). The batch report reads the driven runner's
// per-cycle SignalSummary through formatSignalReport — it reports, never
// gates.
package main

import (
	"fmt"
	"maps"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeLoopSystemFailureHalt   signalcenter.Code = "LOOP_SYSTEM_FAILURE_HALT"
	CodeLoopPipelineBlockerHalt signalcenter.Code = "LOOP_PIPELINE_BLOCKER_HALT"
	CodeLoopFleetLaneHalt       signalcenter.Code = "LOOP_FLEET_LANE_HALT"
	CodeLoopHalt                signalcenter.Code = "LOOP_HALT"
	CodeLoopEscalationBoundary  signalcenter.Code = "LOOP_ESCALATION_BOUNDARY"
	CodeLoopMinWidthRepair      signalcenter.Code = "LOOP_MIN_WIDTH_REPAIR"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopSystemFailureHalt, "the batch halted on an ADR-0072 system failure the cycle itself signalled (the pipeline, not the task, is the cause); fields.category names the floor, fields.next is the escalation dossier's next_action, fields.escalation and fields.inbox_item the dossier and the P0 item the halt wrote")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopPipelineBlockerHalt, "the pipeline-blocker breaker halted the batch (identical fingerprints, unexplained failures or consecutive failures over the ceiling); fields.rule and fields.fingerprint name the rule, the rest are the system-failure halt's own fields (next, escalation, inbox_item)")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopFleetLaneHalt, "a fleet lane exited with the system-failure halt code; the lane's own LOOP_SYSTEM_FAILURE_HALT names the failure and the escalation it filed")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopHalt, "the batch halted at a wave boundary (plane diverged, sync refused); the reason is the halt error")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopEscalationBoundary, "the escalation boundary staged inbox items (bumped/filed/planned) after a cycle; fields carry the counts and the stage")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopMinWidthRepair, "the fleet shrank below its committed width and one isolated lane was dispatched instead (min-width repair)")
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

// emitLoopWave is the loop.wave producer: batch-level (no cycle), the wave
// number stamped on the producer's own copy of the caller's fields; INFO
// without a code, WARN with one.
func emitLoopWave(signals *signalcenter.Center, wave int, origin string, code signalcenter.Code, reason string, fields map[string]string) {
	stamped := make(map[string]string, len(fields)+1)
	maps.Copy(stamped, fields)
	stamped["wave"] = strconv.Itoa(wave)
	e := signalcenter.Event{
		Module: signalcenter.ModuleLoop, Origin: origin, Kind: signalcenter.KindLoopWave,
		Severity: signalcenter.SeverityInfo, Reason: reason, Fields: stamped,
	}
	if code != "" {
		e.Severity, e.Code = signalcenter.SeverityWarn, code
	}
	signals.Emit(e)
}

// emitLoopEscalation is the loop.escalation producer: a WARN that names the
// stage and what the boundary staged.
func emitLoopEscalation(signals *signalcenter.Center, cycle int, origin, reason string, fields map[string]string) {
	signals.Emit(signalcenter.Event{
		Cycle: cycle, Module: signalcenter.ModuleLoop, Origin: origin, Kind: signalcenter.KindLoopEscalation,
		Severity: signalcenter.SeverityWarn, Code: CodeLoopEscalationBoundary, Reason: reason, Fields: fields,
	})
}

// formatSignalReport renders the runner's per-cycle view for the batch
// report: counts by severity and the last INCIDENT; "" when the cycle raised
// no signal. A report line, not a signal — the loop reports, never gates.
func formatSignalReport(cycle int, s signalcenter.Summary) string {
	if s.Total == 0 {
		return ""
	}
	line := fmt.Sprintf("[loop] cycle %d signals: %d total, %d WARN, %d INCIDENT", cycle, s.Total,
		s.BySeverity[signalcenter.SeverityWarn], s.BySeverity[signalcenter.SeverityIncident])
	if s.LastIncident != nil {
		line += fmt.Sprintf(" (last INCIDENT %s — %s)", s.LastIncident.Code, s.LastIncident.Reason)
	}
	return line + "\n"
}
