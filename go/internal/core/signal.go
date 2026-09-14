package core

// signal.go — ADR-0101 S1: the orchestrator is a registered LISTENER of the
// Signal Center, and the ADR-0044 C1 chokepoint (recordPhaseOutcome) is the
// first PRODUCER. Listeners observe, never decide: the summary kept here is
// reporting evidence; verdicts, transitions and halts stay with the floors.
//
// Lock order: the Center holds no lock while delivering, so observeSignal
// takes only signalMu; no orchestrator path holds signalMu across an Emit,
// and observeSignal never emits (no feedback loops).

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The orchestrator module's codes — registered with their docs so
// signalcenter.RegisteredCodes() documents them.
const (
	CodePhaseVerdictFail signalcenter.Code = "ORCHESTRATOR_PHASE_VERDICT_FAIL"
	CodePhaseVerdictWarn signalcenter.Code = "ORCHESTRATOR_PHASE_VERDICT_WARN"
	CodePhaseAborted     signalcenter.Code = "ORCHESTRATOR_PHASE_ABORTED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodePhaseVerdictFail, "a phase recorded verdict FAIL; the reason is the phase's own error-severity diagnostics")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodePhaseVerdictWarn, "a phase recorded verdict WARN; the reason carries its error-severity diagnostics, if any")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodePhaseAborted, "the cycle aborted after this phase's outcome (review reject, guard, persistence); the abort reason is in fields.abort_reason")
}

// WithSignalCenter registers the orchestrator as a listener of c. A nil c is
// the Null Object for tests and the two pinned secondary roots; the production
// composition root always passes a Center (cmd_cycle.go), and
// TestWireOrchestratorDeps_SignalCenterWired proves it.
func WithSignalCenter(c *signalcenter.Center) Option {
	return func(o *Orchestrator) {
		if c == nil {
			return
		}
		o.signals = c
		o.signalSummary = signalcenter.NewSummary()
		c.Subscribe(o.observeSignal)
	}
}

// SignalCenterWired reports whether a Center was injected — the composition
// root's wiring proof.
func (o *Orchestrator) SignalCenterWired() bool { return o.signals != nil }

// SignalSummary is the CURRENT cycle's view: signals by severity and kind and
// the last INCIDENT. It is what "closely monitor the loop" means in code.
func (o *Orchestrator) SignalSummary() signalcenter.Summary {
	o.signalMu.Lock()
	defer o.signalMu.Unlock()
	if o.signalSummary == nil {
		return signalcenter.NewSummary().Snapshot()
	}
	return o.signalSummary.Snapshot()
}

// observeSignal is the listener: O(1), never emits, never decides.
func (o *Orchestrator) observeSignal(e signalcenter.Event) {
	o.signalMu.Lock()
	defer o.signalMu.Unlock()
	o.signalSummary.Observe(e)
}

// emitPhaseOutcome is the C1 chokepoint's producer: one phase.outcome (or
// phase.aborted) per terminal disposition, on both dispatch roots. PASS and
// SKIPPED are INFO; WARN and FAIL are WARN with the phase's own reason
// (verdictReason — the same rendering the seal and the floor error use); an abort after the
// verdict is phase.aborted WARN carrying both. A nil Center is a no-op.
func (o *Orchestrator) emitPhaseOutcome(cycle int, out recovery.PhaseOutcome) {
	e := signalcenter.Event{
		Cycle: cycle, RunID: o.signalRunID(), Phase: out.Phase, Attempt: out.AttemptCount,
		Module: signalcenter.ModuleOrchestrator, Origin: "Orchestrator.recordPhaseOutcome",
		Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityInfo,
		Reason: out.Phase + " " + verdictReason(out.Verdict, out.Diagnostics),
		Fields: map[string]string{
			"verdict": out.Verdict, "archetype": out.Archetype, "duration_ms": strconv.FormatInt(out.DurationMS, 10),
		},
	}
	if codes := cyclestate.ErrorCodes(out.Diagnostics); len(codes) > 0 {
		e.Fields["diagnostic_codes"] = strings.Join(codes, ",") // the phase's own gate codes (cyclestate.DiagCode*)
	}
	switch {
	case out.AbortReason != "":
		e.Kind, e.Severity, e.Code = signalcenter.KindPhaseAborted, signalcenter.SeverityWarn, CodePhaseAborted
		e.Reason = out.Phase + " aborted after verdict=" + out.Verdict + ": " + out.AbortReason
		e.Fields["abort_reason"] = out.AbortReason
	case out.Verdict == VerdictFAIL:
		e.Severity, e.Code = signalcenter.SeverityWarn, CodePhaseVerdictFail
	case out.Verdict == VerdictWARN:
		e.Severity, e.Code = signalcenter.SeverityWarn, CodePhaseVerdictWarn
	}
	o.signals.Emit(e)
}

// CodeGateCorrection is the correction ladder's code (ADR-0101 S2b): one
// gate.corrected INFO per rung the orchestrator runs after a gate rejection.
const CodeGateCorrection signalcenter.Code = "ORCHESTRATOR_GATE_CORRECTION"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeGateCorrection, "the correction ladder ran a rung after a gate rejection — fields name the correction ordinal, the budget (max), the rung, the CLI re-dispatched on and whether that CLI was escalated; the reason is the rejection being corrected")
}

// gateCorrection is one rung of the ladder as the stream sees it (a parameter
// object: six of these become the event's fields, and two adjacent bools at
// a call site would otherwise swap silently).
type gateCorrection struct {
	origin       string // the ladder naming itself: the fresh root's or the resume root's
	cycle        int
	phase        Phase
	correction   int // the ordinal of this correction (1-based)
	max          int // the correction budget
	rung         string
	cli          string // the CLI re-dispatched on, after any escalation
	escalated    bool
	salvageRetry bool
	reason       string // the rejection being corrected
}

// emitGateCorrection is the ladder's producer on both dispatch roots: the
// fresh loop's reviewWithCorrections and the resume root's
// reviewResumedDeliverable name themselves as origin. The correction ordinal
// has ONE home, fields.correction. A nil Center is a no-op.
func (o *Orchestrator) emitGateCorrection(gc gateCorrection) {
	o.signals.Emit(signalcenter.Event{
		Cycle: gc.cycle, RunID: o.signalRunID(), Phase: string(gc.phase),
		Module: signalcenter.ModuleOrchestrator, Origin: gc.origin, Kind: signalcenter.KindGateCorrected,
		Severity: signalcenter.SeverityInfo, Code: CodeGateCorrection, Reason: gc.reason,
		Fields: map[string]string{
			"correction": strconv.Itoa(gc.correction), "max": strconv.Itoa(gc.max), "rung": gc.rung, "cli": gc.cli,
			"escalated": strconv.FormatBool(gc.escalated), "salvage_retry": strconv.FormatBool(gc.salvageRetry),
		},
	})
}
