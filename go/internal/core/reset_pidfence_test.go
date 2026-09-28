package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestSealCycle_DeadOwnerFreshLease_SealsWithoutForce(t *testing.T) {
	t.Parallel()
	sealClock := time.Date(2026, 5, 27, 8, 0, 0, 0, time.UTC)
	ev := t.TempDir()
	ws := sealFixture(t, ev, 108)
	if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN", OwnerPID: 4242}, sealClock); err != nil {
		t.Fatalf("write lease: %v", err)
	}
	opts := sealOpts(ev)
	opts.PidAlive = func(pid int) bool { return false }
	res, err := SealCycle(context.Background(), &recordingLedger{}, opts)
	if err != nil {
		t.Fatalf("dead-owner fresh-lease must seal WITHOUT --force (PID-aware fence): %v", err)
	}
	if res.SealedCycleID != 108 {
		t.Errorf("SealedCycleID = %d, want 108", res.SealedCycleID)
	}
	if res.ForcedOverLiveOwner {
		t.Error("ForcedOverLiveOwner must be false — the owner was never live, so Force was never consulted")
	}
}

func TestSealCycle_LiveOwnerFreshLease_StillRefuses(t *testing.T) {
	t.Parallel()
	sealClock := time.Date(2026, 5, 27, 8, 0, 0, 0, time.UTC)
	ev := t.TempDir()
	ws := sealFixture(t, ev, 108)
	if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN", OwnerPID: 4242}, sealClock); err != nil {
		t.Fatalf("write lease: %v", err)
	}
	opts := sealOpts(ev)
	opts.PidAlive = func(pid int) bool { return pid == 4242 }
	_, err := SealCycle(context.Background(), &recordingLedger{}, opts)
	if !errors.Is(err, ErrCycleOwnedLive) {
		t.Fatalf("a live owner (fresh lease, alive pid) must still refuse with ErrCycleOwnedLive, got %v", err)
	}
}

func TestSealCycle_NilPidAlive_PreservesFreshnessOnlyBehavior(t *testing.T) {
	t.Parallel()
	sealClock := time.Date(2026, 5, 27, 8, 0, 0, 0, time.UTC)
	ev := t.TempDir()
	ws := sealFixture(t, ev, 108)
	if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN", OwnerPID: 4242}, sealClock); err != nil {
		t.Fatalf("write lease: %v", err)
	}
	opts := sealOpts(ev)
	_, err := SealCycle(context.Background(), &recordingLedger{}, opts)
	if !errors.Is(err, ErrCycleOwnedLive) {
		t.Fatalf("nil PidAlive must preserve freshness-only refusal, got %v", err)
	}
}
