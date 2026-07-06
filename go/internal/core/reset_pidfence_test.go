package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// reset_pidfence_test.go — RED contract for cycle-554 workspace-hygiene-s1:
// SealCycle's liveness fence (reset.go F1) checks lease freshness only, so a
// crashed owner with a still-fresh heartbeat (2-6min post-crash window) blocks
// sealing and forces `evolve cycle reset --force` at every batch boundary
// (plan docs/plans/workspace-hygiene-2026-07.md §S1). SealOptions.PidAlive
// threads a liveness probe through the fence via runlease.OwnerLive:
// dead-pid+fresh-lease now seals WITHOUT --force, while a genuinely live
// owner (fresh lease, alive pid) still refuses — the invariant the fence
// exists for (cycle-395 race) must survive unweakened.

// TestSealCycle_DeadOwnerFreshLease_SealsWithoutForce — the core fix: a
// stranded marker whose owner crashed (pid dead) but whose lease heartbeat
// has not yet aged past the 10min TTL must seal on the FIRST attempt, with no
// --force needed.
func TestSealCycle_DeadOwnerFreshLease_SealsWithoutForce(t *testing.T) {
	t.Parallel()
	sealClock := time.Date(2026, 5, 27, 8, 0, 0, 0, time.UTC)
	ev := t.TempDir()
	ws := sealFixture(t, ev, 108)
	if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN", OwnerPID: 4242}, sealClock); err != nil {
		t.Fatalf("write lease: %v", err)
	}
	opts := sealOpts(ev)
	opts.PidAlive = func(pid int) bool { return false } // owner process is dead
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

// TestSealCycle_LiveOwnerFreshLease_StillRefuses — NEGATIVE regression: PID
// awareness must not weaken the safety invariant. A genuinely running owner
// (fresh lease, alive pid) is never sealed out from under it without --force.
// Cheapest gaming fake (always-true OwnerLive) fails this test.
func TestSealCycle_LiveOwnerFreshLease_StillRefuses(t *testing.T) {
	t.Parallel()
	sealClock := time.Date(2026, 5, 27, 8, 0, 0, 0, time.UTC)
	ev := t.TempDir()
	ws := sealFixture(t, ev, 108)
	if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN", OwnerPID: 4242}, sealClock); err != nil {
		t.Fatalf("write lease: %v", err)
	}
	opts := sealOpts(ev)
	opts.PidAlive = func(pid int) bool { return pid == 4242 } // owner is genuinely alive
	_, err := SealCycle(context.Background(), &recordingLedger{}, opts)
	if !errors.Is(err, ErrCycleOwnedLive) {
		t.Fatalf("a live owner (fresh lease, alive pid) must still refuse with ErrCycleOwnedLive, got %v", err)
	}
}

// TestSealCycle_NilPidAlive_PreservesFreshnessOnlyBehavior — EDGE/back-compat:
// a caller that does not set PidAlive (e.g. any pre-existing caller not yet
// migrated) must keep the old freshness-only fence — a fresh lease refuses
// regardless of any real process state, exactly like TestSealCycle_LeaseFencing
// already pins for the zero-value SealOptions.
func TestSealCycle_NilPidAlive_PreservesFreshnessOnlyBehavior(t *testing.T) {
	t.Parallel()
	sealClock := time.Date(2026, 5, 27, 8, 0, 0, 0, time.UTC)
	ev := t.TempDir()
	ws := sealFixture(t, ev, 108)
	if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN", OwnerPID: 4242}, sealClock); err != nil {
		t.Fatalf("write lease: %v", err)
	}
	opts := sealOpts(ev) // PidAlive left nil
	_, err := SealCycle(context.Background(), &recordingLedger{}, opts)
	if !errors.Is(err, ErrCycleOwnedLive) {
		t.Fatalf("nil PidAlive must preserve freshness-only refusal, got %v", err)
	}
}
