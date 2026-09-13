package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurelearning"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Test 28 — the engine is constructed in ONE non-test file (the seam); the
// helper skips the leaf (which never spells its own package name).
func TestFailureLearningEngine_OneConstructionSite(t *testing.T) {
	const onlySite = "internal/core/failure_learning_engine.go"
	if offenders := nonTestSourcesMentioning(t, "failurelearning.New(", onlySite); len(offenders) > 0 {
		t.Errorf("the failure-learning engine has ONE construction file (%s); these non-test files construct their own: %v", onlySite, offenders)
	}
}

// Test 29 — a literal orchestrator (the remediation and floor-verdict tests
// build one) lazily gets ONE engine whose clock follows o.now swapped later.
func TestFailureLearning_LiteralOrchestratorGetsTheEngineOnce(t *testing.T) {
	o := &Orchestrator{now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }}
	first := o.failureLearning()
	if first == nil || first.SignalsWired() || o.failureLearning() != first {
		t.Fatal("the lazily built engine is unwired and kept, not rebuilt")
	}
	st := &State{}
	fl := failureLearningRequest{Cycle: 3, Failed: PhaseBuild, Err: errors.New("x"), State: st, CycleState: &CycleState{WorkspacePath: t.TempDir()}}
	o.recordFailedApproachState(fl)
	o.now = func() time.Time { return time.Date(2027, 2, 2, 0, 0, 0, 0, time.UTC) }
	o.recordFailedApproachState(fl)
	if st.FailedAt[0].TS != "2026-01-01T00:00:00Z" || st.FailedAt[1].TS != "2027-02-02T00:00:00Z" {
		t.Fatalf("the engine's clock is a closure over o.now, never a snapshot: %s / %s", st.FailedAt[0].TS, st.FailedAt[1].TS)
	}
}

// Test 30 — a Center applied AFTER construction receives the engine's WARN
// with the unit's module, kind, origin, cycle, phase and fields.
func TestFailureLearning_SeesASignalCenterAppliedAfterConstruction(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if o.failureLearning().SignalsWired() {
		t.Fatal("no Center at construction ⇒ not wired")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "policy.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	fl := failureLearningRequest{CycleRequest: CycleRequest{ProjectRoot: root}, Cycle: 7, Failed: PhaseAudit, Err: errors.New("x"),
		State: &State{}, CycleState: &CycleState{WorkspacePath: t.TempDir()}}
	o.writeDeterministicLearning(fl, "s", nil) // a provoked fault reports into nothing
	c, got := recordingCenter()
	WithSignalCenter(c)(o)
	if !o.failureLearning().SignalsWired() {
		t.Fatal("the accessor is read live")
	}
	o.writeDeterministicLearning(fl, "s", nil)
	w := eventsOfKind(*got, signalcenter.KindFailureLearningWarning)
	if len(w) != 1 || w[0].Code != failurelearning.CodePolicyLoadFailed || w[0].Module != signalcenter.ModuleFailureLearning ||
		w[0].Origin != "Engine.WriteFloor" || w[0].Cycle != 7 || w[0].Phase != "audit" || w[0].Fields["path"] != filepath.Join(root, ".evolve", "policy.json") {
		t.Fatalf("exactly one FAILURELEARNING_POLICY_LOAD_FAILED through the facade: %+v", *got)
	}
}

// Test 31 — NewOrchestrator builds the engine eagerly, wired, and its mint
// lands in the state the orchestrator's lifecycle persists. (The eager order
// — engine after o.carry — is kept for one-construction hygiene; building
// the engine first is an EQUIVALENT mutant because a Lifecycle holds only the
// live Center accessor: recorded, not padded.)
func TestFailureLearning_EagerConstructionSharesTheWiredLifecycle(t *testing.T) {
	st := &fakeStorage{}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil), WithSignalCenter(signalcenter.New()))
	if o.learn == nil || !o.failureLearning().SignalsWired() {
		t.Fatal("NewOrchestrator builds the engine eagerly, wired to the root's Center")
	}
	carry := o.carry
	state := &State{}
	o.recordFailedApproachState(failureLearningRequest{Cycle: 5, Failed: PhaseBuild, Err: errors.New("x"), State: state, CycleState: &CycleState{WorkspacePath: t.TempDir()}})
	o.writeFailureLearningState(context.Background(), state)
	if o.carry != carry || len(st.state.CarryoverTodos) != 1 || st.state.CarryoverTodos[0].ID != "cycle-5-failed-build" {
		t.Fatalf("the engine's mint lands in the state the orchestrator's lifecycle persists: %+v", st.state.CarryoverTodos)
	}
}
