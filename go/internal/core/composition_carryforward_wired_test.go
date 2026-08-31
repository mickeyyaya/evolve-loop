// composition_carryforward_wired_test.go — cycle-804 TDD contract (inbox
// weight 0.98, wire-rung0-composition-writer-into-fleet-rebase).
//
// cycle-786/801 built the RUNG 0 composition-verdict writer (ledger),
// reader (ship), and core seam (composition_carryforward.go) — three
// Option-injected closures (WithCompositionSnapshot/GateRunner/
// VerdictWriter), all nil by default. Nothing in cmd/evolve ever binds
// them, so compositionCarryForward's nil-guard (composition_carryforward.go
// :96) always trips and every clean fleet rebase falls through to a full
// re-audit — the exact behavior the writer was built to eliminate.
//
// This file pins the "wired" check itself (mirrors FailureAdviserWired /
// failure_hook.go:56 — same AND-of-three-closures observability pattern
// already established for a different Option trio). The production
// composition root's use of it is pinned separately in
// cmd/evolve/cmd_cycle_composition_test.go.
//
// RED today: CompositionFastPathWired is undefined on *Orchestrator.
package core

import (
	"context"
	"errors"
	"testing"
)

func TestCompositionSnapshot_ReceivesCycleRunID(t *testing.T) {
	want := "run-bound-to-cycle"
	got := ""
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithCompositionSnapshot(func(_ context.Context, _ string, runID string) (CompositionAuditSnapshot, error) {
			got = runID
			return CompositionAuditSnapshot{}, errors.New("stop after observing binding")
		}),
		WithCompositionGateRunner(func(context.Context, string) map[string]string { return nil }),
		WithCompositionVerdictWriter(func(string, CompositionVerdictInput) error { return nil }),
	)
	if o.compositionCarryForward(context.Background(), 1, CycleState{ActiveWorktree: "unused", RunID: want}) {
		t.Fatal("snapshot error unexpectedly carried composition forward")
	}
	if got != want {
		t.Fatalf("composition snapshot received run ID %q, want %q", got, want)
	}
}

// TestOrchestrator_CompositionFastPathWired: false on a bare orchestrator,
// false when only some of the three composition closures are injected
// (AND, not OR — a partial binding must not report itself as wired), true
// only when all three are set.
func TestOrchestrator_CompositionFastPathWired(t *testing.T) {
	t.Parallel()

	dummySnapshot := func(ctx context.Context, worktree, runID string) (CompositionAuditSnapshot, error) {
		return CompositionAuditSnapshot{}, nil
	}
	dummyGateRunner := func(ctx context.Context, worktree string) map[string]string { return nil }
	dummyWriter := func(ledgerPath string, in CompositionVerdictInput) error { return nil }

	bare := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if bare.CompositionFastPathWired() {
		t.Error("bare orchestrator must report CompositionFastPathWired()=false")
	}

	snapshotOnly := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithCompositionSnapshot(dummySnapshot))
	if snapshotOnly.CompositionFastPathWired() {
		t.Error("orchestrator with only WithCompositionSnapshot must report CompositionFastPathWired()=false (partial binding is not wired)")
	}

	snapshotAndGates := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithCompositionSnapshot(dummySnapshot),
		WithCompositionGateRunner(dummyGateRunner))
	if snapshotAndGates.CompositionFastPathWired() {
		t.Error("orchestrator with two of three composition closures must report CompositionFastPathWired()=false")
	}

	wired := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithCompositionSnapshot(dummySnapshot),
		WithCompositionGateRunner(dummyGateRunner),
		WithCompositionVerdictWriter(dummyWriter))
	if !wired.CompositionFastPathWired() {
		t.Error("orchestrator with all three WithComposition* options must report CompositionFastPathWired()=true")
	}
}
