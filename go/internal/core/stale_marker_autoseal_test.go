package core

// stale_marker_autoseal_test.go — RED tests (cycle 507, task
// wire-boot-recovery-functions) for auto-sealing a stranded cycle-state marker
// whose owner process is dead, at loop boot. Function-level behavior contract
// for the recovery primitive the Builder (re)implements in
// stale_marker_autoseal.go.
//
// Root cause (scout-report.md Key Finding 5): a crashed cycle strands a
// role-gated cycle-state marker (e.g. phase=retro) whose role-gate then BLOCKS
// operator/inbox writes, gating even the recovery actions behind a manual
// `evolve cycle reset --force`. Boot must auto-seal such a marker when its owner
// PID is dead — reusing the SAME SealCycle(Force) path (no duplicated sealing
// logic, per never_duplicate_centralize_via_design_patterns).
//
// References markerShouldAutoseal / AutosealStaleMarker, which the Builder
// implements. RED now (undefined symbols → core test package fails to compile).
// Do NOT modify this file — implement the production seam.

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// AC1 (positive): a marker owned by a DEAD pid must be auto-sealed. Liveness is
// the injected probe (kill -0 semantics) so the test does not depend on the OS
// process table.
func TestMarkerShouldAutoseal_DeadOwner(t *testing.T) {
	if !markerShouldAutoseal(999999, true /*hasPID*/, func(int) bool { return false /*dead*/ }) {
		t.Error("a marker whose owner PID is dead must be auto-sealed")
	}
}

// AC2 (negative): a marker owned by a LIVE pid must NOT be touched — boot must
// not tear down an in-progress cycle just because it is old.
func TestMarkerShouldAutoseal_LiveOwnerUntouched(t *testing.T) {
	self := os.Getpid()
	alive := func(pid int) bool { return pid == self }
	if markerShouldAutoseal(self, true, alive) {
		t.Error("a marker owned by a live process must NOT be auto-sealed")
	}
}

// AC3 (edge): a malformed/missing-pid marker cannot assert liveness, so boot
// treats it as stale (fail-safe toward auto-seal) — the alive probe is not even
// consulted for a marker with no usable pid.
func TestMarkerShouldAutoseal_MissingPidFailsSafe(t *testing.T) {
	if !markerShouldAutoseal(0, false /*hasPID*/, func(int) bool { return true /*would-be-alive, must be ignored*/ }) {
		t.Error("a marker that cannot assert liveness (missing/zero pid) must fail SAFE toward auto-seal")
	}
}

// AC4 + AC5 (end-to-end, reuse-SealCycle): seed a real stranded marker with a
// lease that LOOKS fresh (heartbeat at opts.Now) but is owned by a dead pid.
// AutosealStaleMarker must seal it via SealCycle(Force) — proven by the single
// recorded ledger append (AC5: no bespoke/duplicated seal path) — and after the
// seal a subsequent boot finds nothing to reset (AC4: the role-gate block is
// cleared so the next dispatch/inbox write proceeds).
func TestAutosealStaleMarker_DeadOwnerSealsViaSealCycleAndClearsBlock(t *testing.T) {
	evolveDir := t.TempDir()
	workspace := sealFixture(t, evolveDir, 491)
	// A lease that would read as FRESH (heartbeat == opts.Now) yet whose owner
	// pid is dead — proves liveness is pid-based, not merely heartbeat-age based,
	// and that AutosealStaleMarker forces the seal over a "fresh" lease.
	frozen := time.Date(2026, 5, 27, 8, 0, 0, 0, time.UTC)
	if err := runlease.Write(workspace, runlease.Lease{RunID: "run-491", OwnerPID: 999999}, frozen); err != nil {
		t.Fatalf("seed lease: %v", err)
	}
	dead := func(int) bool { return false }

	led := &recordingLedger{}
	res, sealed, err := AutosealStaleMarker(context.Background(), led, sealOpts(evolveDir), dead)
	if err != nil {
		t.Fatalf("AutosealStaleMarker: %v", err)
	}
	if !sealed {
		t.Fatal("dead-owner marker must be auto-sealed even when its lease heartbeat looks fresh")
	}
	if res.SealedCycleID != 491 {
		t.Errorf("must seal the stranded cycle 491; sealed %d", res.SealedCycleID)
	}
	if len(led.entries) != 1 {
		t.Errorf("auto-seal must REUSE SealCycle (exactly one ledger append); got %d — a bespoke seal path duplicates logic (AC5)", len(led.entries))
	}

	// AC4: the block is cleared — a subsequent boot finds nothing to reset, so
	// the next cycle dispatch / inbox write is no longer role-gate blocked.
	_, sealed2, err2 := AutosealStaleMarker(context.Background(), led, sealOpts(evolveDir), dead)
	if sealed2 {
		t.Error("the marker must be cleared after seal; a second autoseal must be a no-op")
	}
	if !errors.Is(err2, ErrNothingToReset) {
		t.Errorf("after seal the marker is gone → ErrNothingToReset; got %v", err2)
	}
}
