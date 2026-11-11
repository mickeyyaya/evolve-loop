package gaterun

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeViolation  signalcenter.Code = "GATE_HERMETIC_VIOLATION"
	CodeNoEvidence signalcenter.Code = "GATE_HERMETIC_NO_EVIDENCE"

	signalOrigin = "gaterun.Run"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleGateHermetic, CodeViolation, "a gate entry did an operation that the hermetic gate profile denies (ADR-0165 G2, 7.16): a read of a .git entry, a process exec, a network call, a read of a secret, or any other operation outside the grants; emitted at shadow only, where the sandbox reports the operation and lets it run, because enforce refuses it with no report; fields.entry, operation (the sandbox operation), path, rule (gate.git, gate.exec, gate.secret, gate.network or gate.default), mode (shadow), action (logged), process and pid")
	signalcenter.RegisterCode(signalcenter.ModuleGateHermetic, CodeNoEvidence, "a hermetic gate run at shadow gave no complete evidence: on Linux bwrap cannot report a denied operation; on macOS the sandbox reports could not be read, the fence of the run did not show, or a canary report of the child is missing because the sandbox dropped its reports (ADR-0165 G2, 7.16); fields.entry and cause; move the host to enforce to measure")
}

type Emitter interface {
	Emit(signalcenter.Event)
}

func emitViolation(out Emitter, tg Target, mode string, r sandbox.Report) {
	if out == nil {
		return
	}
	action := "logged"
	if r.Denied {
		action = "refused"
	}
	out.Emit(signalcenter.Event{
		Module: signalcenter.ModuleGateHermetic, Origin: signalOrigin, Kind: signalcenter.KindIsolationViolation, Severity: signalcenter.SeverityWarn, Code: CodeViolation,
		Reason: fmt.Sprintf("the gate entry %s did %s on %s", tg.Entry, r.Operation, r.Target),
		Fields: map[string]string{
			"entry": tg.Entry, "operation": r.Operation, "path": r.Target, "rule": r.Rule, "mode": mode, "action": action,
			"process": r.Process, "pid": r.PID,
		},
	})
}

func emitNoEvidence(out Emitter, tg Target, cause string) {
	if out == nil {
		return
	}
	out.Emit(signalcenter.Event{
		Module: signalcenter.ModuleGateHermetic, Origin: signalOrigin, Kind: signalcenter.KindIsolationViolation, Severity: signalcenter.SeverityWarn, Code: CodeNoEvidence,
		Reason: fmt.Sprintf("the gate entry %s ran at shadow with no hermetic evidence: %s", tg.Entry, cause),
		Fields: map[string]string{"entry": tg.Entry, "cause": cause},
	})
}
