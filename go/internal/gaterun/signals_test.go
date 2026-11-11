package gaterun

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type recorder struct{ events []signalcenter.Event }

func (r *recorder) Emit(e signalcenter.Event) { r.events = append(r.events, e) }

func TestCodes_BothAreRegisteredUnderTheHermeticModule(t *testing.T) {
	t.Parallel()
	for _, code := range []signalcenter.Code{CodeViolation, CodeNoEvidence} {
		if !code.BelongsTo(signalcenter.ModuleGateHermetic) {
			t.Errorf("%s must belong to %s", code, signalcenter.ModuleGateHermetic)
		}
	}
	if CodeViolation != "GATE_HERMETIC_VIOLATION" || CodeNoEvidence != "GATE_HERMETIC_NO_EVIDENCE" {
		t.Errorf("codes = %s, %s", CodeViolation, CodeNoEvidence)
	}
}

func TestEmitViolation_NamesTheEntryTheOperationAndThePath(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	emitViolation(rec, Target{Entry: "testdup", Lane: "/lane"}, "shadow", sandbox.Report{Denied: false, Operation: "file-read-data", Target: "/lane/.git/HEAD", Rule: sandbox.RuleGateGit, Process: "git", PID: "7"})
	if len(rec.events) != 1 {
		t.Fatalf("events = %+v", rec.events)
	}
	e := rec.events[0]
	if e.Code != CodeViolation || e.Module != signalcenter.ModuleGateHermetic || e.Severity != signalcenter.SeverityWarn || e.Kind != signalcenter.KindIsolationViolation {
		t.Errorf("event = %+v", e)
	}
	for k, want := range map[string]string{"entry": "testdup", "operation": "file-read-data", "path": "/lane/.git/HEAD", "rule": "gate.git", "mode": "shadow", "action": "logged", "process": "git", "pid": "7"} {
		if e.Fields[k] != want {
			t.Errorf("field %s = %q, want %q", k, e.Fields[k], want)
		}
	}
	if !strings.Contains(e.Reason, "testdup") {
		t.Errorf("reason = %q, want the entry", e.Reason)
	}
}

func TestEmitViolation_ADeniedOperationIsRefused(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	emitViolation(rec, Target{Entry: "e"}, "enforce", sandbox.Report{Denied: true})
	if rec.events[0].Fields["action"] != "refused" {
		t.Errorf("fields = %v", rec.events[0].Fields)
	}
}

func TestEmitNoEvidence_GivesTheEntryAndTheCause(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	emitNoEvidence(rec, Target{Entry: "testdup"}, "bwrap cannot report")
	e := rec.events[0]
	if e.Code != CodeNoEvidence || e.Fields["entry"] != "testdup" || e.Fields["cause"] != "bwrap cannot report" || e.Severity != signalcenter.SeverityWarn {
		t.Errorf("event = %+v", e)
	}
}

func TestEmit_ANilEmitterIsASilentNoop(t *testing.T) {
	t.Parallel()
	var none Emitter
	emitViolation(none, Target{}, "shadow", sandbox.Report{})
	emitNoEvidence(none, Target{}, "x")
}
