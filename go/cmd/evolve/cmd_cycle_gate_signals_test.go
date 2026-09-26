package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestWireOrchestratorDeps_ContractGateSignalsWired(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	d := wireOrchestratorDeps(root, evolveDir, io.Discard)
	if !d.Orchestrator.DeclaredDeliverablesGateWired() {
		t.Fatal("precondition: the default policy mounts the declared-deliverables gate (ADR-0100)")
	}
	if !d.Orchestrator.ContractGateSignalsWired() {
		t.Fatal("the production root must hand its Center to the contract gate (ADR-0101 S2b): the gate's decisions are otherwise invisible on the stream")
	}
}

func TestFormatSignalReport_CountsTheGateVerdicts(t *testing.T) {
	s := signalcenter.NewSummary()
	emit := func(kind signalcenter.Kind, sev signalcenter.Severity) {
		s.Observe(signalcenter.Event{Cycle: 7, Module: signalcenter.ModuleGateContract, Origin: "Reviewer.Review", Kind: kind, Severity: sev, Reason: "r"})
	}
	emit(signalcenter.KindGatePassed, signalcenter.SeverityInfo)
	emit(signalcenter.KindGatePassed, signalcenter.SeverityInfo)
	emit(signalcenter.KindGateRejected, signalcenter.SeverityWarn)
	emit(signalcenter.KindGateCorrected, signalcenter.SeverityInfo)
	got := formatSignalReport(7, s.Snapshot())
	if !strings.Contains(got, "gates: 2 passed, 1 rejected, 1 corrected") {
		t.Fatalf("the report line carries the gate verdict counts: %q", got)
	}
	plain := signalcenter.NewSummary()
	plain.Observe(signalcenter.Event{Cycle: 7, Module: signalcenter.ModuleLoop, Origin: "x", Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo, Reason: "r"})
	if got := formatSignalReport(7, plain.Snapshot()); strings.Contains(got, "gates:") {
		t.Fatalf("no gate events, no gate clause: %q", got)
	}
}
