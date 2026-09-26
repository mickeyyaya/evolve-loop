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
	if o.compositionCarryForward(context.Background(), 1, CycleState{ActiveWorktree: "unused", RunID: want}, "") {
		t.Fatal("snapshot error unexpectedly carried composition forward")
	}
	if got != want {
		t.Fatalf("composition snapshot received run ID %q, want %q", got, want)
	}
}

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
