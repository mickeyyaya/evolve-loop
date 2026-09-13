package core

// gate_signal_test.go — ADR-0101 S2b: the correction ladder is a producer.
// Every rung the orchestrator runs after a gate rejection is ONE gate.corrected
// INFO under module orchestrator, on BOTH dispatch roots, naming the correction
// ordinal, the budget, the rung and the CLI it re-dispatched on — so the stream
// shows "rejected → corrected (1/2, redispatch) → passed" for a phase boundary.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestReviewWithCorrections_EmitsGateCorrectedPerRedispatch(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{phase: "build", threshold: neverDemotingThreshold, approveAfter: 1}
	signals, got := recordingCenter()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	runners[PhaseBuild] = probe
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithReviewer(probe), WithSignalCenter(signals))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true}); err != nil {
		t.Fatalf("RunCycle should proceed after one correction: %v", err)
	}
	corrected := eventsOfKind(*got, signalcenter.KindGateCorrected)
	if len(corrected) != 1 {
		t.Fatalf("one correction re-dispatch → one gate.corrected: %+v", corrected)
	}
	e := corrected[0]
	if e.Module != signalcenter.ModuleOrchestrator || e.Origin != "cycleRun.reviewWithCorrections" || e.Severity != signalcenter.SeverityInfo || e.Code != CodeGateCorrection || e.Phase != "build" || e.Cycle != 1 {
		t.Fatalf("gate.corrected INFO ORCHESTRATOR_GATE_CORRECTION from the fresh root's ladder: %+v", e)
	}
	if e.Fields["correction"] != "1" || e.Fields["max"] == "" || e.Fields["rung"] != "redispatch" || e.Fields["escalated"] != "false" {
		t.Fatalf("fields name the ordinal, the budget, the rung and whether the CLI escalated: %+v", e.Fields)
	}
	if e.Reason == "" || e.Reason != probe.phase+" deliverable failed contract: [missing_section] required section 'Findings' not found" {
		t.Fatalf("the reason is the rejection being corrected: %q", e.Reason)
	}
}

func TestReviewWithCorrections_EscalatedRedispatchSaysSo(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{phase: "build", threshold: neverDemotingThreshold, approveAfter: 2}
	signals, got := recordingCenter()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	runners[PhaseBuild] = probe
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithReviewer(probe), WithSignalCenter(signals))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true}); err != nil {
		t.Fatalf("RunCycle should proceed after two corrections: %v", err)
	}
	corrected := eventsOfKind(*got, signalcenter.KindGateCorrected)
	if len(corrected) != 2 || corrected[0].Fields["correction"] != "1" || corrected[1].Fields["correction"] != "2" {
		t.Fatalf("two re-dispatches → two ordered gate.corrected: %+v", corrected)
	}
	if corrected[1].Fields["escalated"] != "true" || corrected[1].Fields["cli"] != "codex-tmux" {
		t.Fatalf("the second identical block escalates the CLI, and the signal says so: %+v", corrected[1].Fields)
	}
}

func TestResumeReviewGate_EmitsGateCorrectedOnTheResumeRoot(t *testing.T) {
	projectRoot := t.TempDir()
	ws := filepath.Join(projectRoot, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	st := &fakeStorage{
		state:      State{LastCycleNumber: 9},
		cycleState: CycleState{CycleID: 9, WorkspacePath: ws, ActiveWorktree: t.TempDir()},
	}
	probe := &escalationProbe{phase: "audit", threshold: neverDemotingThreshold, approveAfter: 1}
	signals, got := recordingCenter()
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil),
		WithWorktreeProvisioner(&fakeWorktree{path: st.cycleState.ActiveWorktree}),
		WithReviewer(probe), WithSignalCenter(signals))
	if _, err := o.RunCycleFromPhase(context.Background(),
		CycleRequest{ProjectRoot: projectRoot, GoalHash: "g"},
		&ResumePoint{Phase: string(PhaseAudit), CycleID: 9}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	corrected := eventsOfKind(*got, signalcenter.KindGateCorrected)
	if len(corrected) != 1 || corrected[0].Origin != "Orchestrator.reviewResumedDeliverable" || corrected[0].Phase != "audit" || corrected[0].Cycle != 9 || corrected[0].Fields["correction"] != "1" || corrected[0].Fields["rung"] != "redispatch" {
		t.Fatalf("the resume root's ladder is the same producer: %+v", corrected)
	}
}

func TestGateCorrectionCode_IsRegisteredUnderOrchestrator(t *testing.T) {
	if m, ok := signalcenter.IsRegistered(CodeGateCorrection); !ok || m != signalcenter.ModuleOrchestrator {
		t.Fatalf("ORCHESTRATOR_GATE_CORRECTION must be registered under module orchestrator: %v %v", m, ok)
	}
}

// gateSignalsMarker stands in for the contract gate in the chain: the
// composition-root proof must require BOTH capabilities — a chain member with
// a SignalsWired method that is not the declared-deliverables gate proves
// nothing, and the gate without a Center is the silence the proof exists for.
type gateSignalsMarker struct{ gate, wired bool }

func (m gateSignalsMarker) Review(context.Context, ReviewInput) ReviewResult {
	return ReviewResult{Approve: true}
}
func (m gateSignalsMarker) VerifiesDeclaredDeliverables() bool { return m.gate }
func (m gateSignalsMarker) SignalsWired() bool                 { return m.wired }

func TestContractGateSignalsWired_RequiresTheGateAndItsCenter(t *testing.T) {
	for _, tc := range []struct {
		name string
		rev  DeliverableReviewer
		want bool
	}{
		{"no reviewer", nil, false},
		{"gate with a Center", gateSignalsMarker{gate: true, wired: true}, true},
		{"gate without a Center", gateSignalsMarker{gate: true, wired: false}, false},
		{"another reviewer with a Center is not the gate", gateSignalsMarker{gate: false, wired: true}, false},
		{"nested in the chain", ChainReviewers(gateMarker{on: false}, gateSignalsMarker{gate: true, wired: true}), true},
	} {
		o := &Orchestrator{reviewer: tc.rev}
		if got := o.ContractGateSignalsWired(); got != tc.want {
			t.Errorf("%s: ContractGateSignalsWired() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
